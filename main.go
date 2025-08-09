package main

import (
	"bufio"
	"debug/elf"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: cpwn [-s <path>] | <file> [<version>]")
		return
	}

	if os.Args[1] == "-s" {
		if len(os.Args) < 3 {
			fmt.Println("Usage: cpwn -s <path>")
			return
		}
		saveToCpwnFile(os.Args[2])
		return
	}
	if os.Args[1] == "-l" {
		if len(os.Args) < 3 {
			fmt.Println("Usage: cpwn -l <lib>")
			return
		}
		libpath, err := filepath.Abs(os.Args[2])
		checkErr("libpath", err)
		Putv(libpath)
	}
	file := os.Args[1]
	version := "0"
	if len(os.Args) > 2 {
		version = os.Args[2]
	}

	fullPath, err := filepath.Abs(file)
	checkErr("Error getting absolute path", err)

	executeLdd(fullPath, version)
}

// ---------- 核心路径和配置读取 -----------
// Putv 读取 libc 文件的版本信息
func Putv(libPath string) {
	// 用 strings 命令 + grep
	cmd := exec.Command("strings", libPath)
	pipe, err := cmd.StdoutPipe()
	checkErr("Error creating pipe", err)

	if err := cmd.Start(); err != nil {
		logFail("Error starting strings: " + err.Error())
		return
	}

	scanner := bufio.NewScanner(pipe)
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Ubuntu GLIBC") {
			fmt.Println(line)
			found = true
		}
	}
	if err := cmd.Wait(); err != nil {
		logFail("Error waiting for strings: " + err.Error())
	}
	if !found {
		logInfo("No Ubuntu GLIBC string found in " + libPath)
	}
}

func getCpwnPath() string {
	home, err := os.UserHomeDir()
	checkErr("Error getting home directory", err)
	return filepath.Join(home, ".cpwn")
}

func saveToCpwnFile(content string) {
	err := os.WriteFile(getCpwnPath(), []byte(content), 0644)
	if err != nil {
		logFail("Failed to save cpwn path: " + err.Error())
	} else {
		logSuccess("Saved cpwn base path to: " + getCpwnPath())
	}
}

func readCpwnBasePath() string {
	data, err := os.ReadFile(getCpwnPath())
	checkErr("Error reading .cpwn", err)
	return strings.TrimSpace(string(data))
}

// ---------- glibc 目录查找 ----------

func glib(version, arch string) string {
	base := readCpwnBasePath()
	libPath := filepath.Join(base, "libs")

	// 如果 libs 目录不存在就创建
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		err = os.MkdirAll(libPath, 0755)
		checkErr("Error creating libs directory", err)
	}

	entries, err := os.ReadDir(libPath)
	checkErr("Error reading libs directory", err)

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), version) && strings.HasSuffix(e.Name(), arch) {
			return filepath.Join(libPath, e.Name())
		}
	}
	logFail("No matching glibc found.")
	return ""
}

// ---------- ELF 文件架构识别 ----------

func getArchitecture(file string) string {
	f, err := elf.Open(file)
	checkErr("Error opening ELF file", err)
	defer f.Close()

	switch f.FileHeader.Machine {
	case elf.EM_386:
		return "i386"
	case elf.EM_X86_64:
		return "amd64"
	default:
		return "unknown"
	}
}

// ---------- 主流程：执行 ldd ----------

func executeLdd(file, version string) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		logFail("No file found: " + file)
		return
	}

	base := readCpwnBasePath()
	arch := getArchitecture(file)
	logInfo("File architecture: " + arch)

	glibPath := glib(version, arch)
	if glibPath == "" {
		handleVersionSelection(base, version, arch, file)
		return
	}

	ld := getLdPath(glibPath, arch)
	executePatchelf(file, ld, glibPath)
	fixDebugSymbols(version, glibPath)
}

// ---------- 路径构造 ----------

func getLdPath(glibPath, arch string) string {
	if arch == "amd64" {
		return filepath.Join(glibPath, "ld-linux-x86-64.so.2")
	}
	return filepath.Join(glibPath, "ld-linux.so.2")
}

// ---------- 版本选择与下载 ----------

func handleVersionSelection(base, version, arch, file string) {
	logInfo("Searching for matching version...")
	listPath := filepath.Join(base, "list")
	oldListPath := filepath.Join(base, "old_list")

	listContent, err := os.ReadFile(listPath)
	if err != nil {
		logFail("Error reading list: " + err.Error())
		return
	}
	oldListContent, err := os.ReadFile(oldListPath)
	if err != nil {
		logFail("Error reading old_list: " + err.Error())
		return
	}

	lines := strings.Split(string(listContent), "\n")
	oldLines := strings.Split(string(oldListContent), "\n")

	filteredLines := filterLines(lines, version, arch)
	filteredOldLines := filterLines(oldLines, version, arch)

	if len(filteredLines) > 0 {
		selectVersion(filteredLines, base, "download", file, version)
	} else if len(filteredOldLines) > 0 {
		logInfo("New version not found, trying old version:")
		selectVersion(filteredOldLines, base, "download_old", file, version)
	} else {
		logFail("No matching versions found.")
	}
}

func filterLines(lines []string, version, arch string) []string {
	var filtered []string
	for _, line := range lines {
		if strings.HasPrefix(line, version) && strings.HasSuffix(line, arch) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func selectVersion(versions []string, base, command, file, version string) {
	logPrompt("Select glibc version:")
	for i, v := range versions {
		fmt.Printf("  \033[1;32m[%d]\033[0m %s\n", i, v)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\033[36mEnter your choice number: \033[0m")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	choice, err := strconv.Atoi(input)
	if err != nil || choice < 0 || choice >= len(versions) {
		logFail("Invalid selection.")
		return
	}

	selected := versions[choice]
	logInfo("You selected: " + selected)
	executeDownloadCommand(base, command, selected, file, version)
}

func executeDownloadCommand(base, command, selected, file, version string) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	shellCmd := "bash"
	if strings.HasSuffix(shell, "zsh") {
		shellCmd = "zsh"
	}

	cmdPath := filepath.Join(base, command)
	cmdStr := fmt.Sprintf("%s %s", cmdPath, selected)
	logInfo("Executing: " + cmdStr)
	cmd := exec.Command(shellCmd, "-c", cmdStr)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logFail(fmt.Sprintf("Error executing download command: %v\n%s", err, string(output)))
		return
	}
	logSuccess("Download completed.")
	fmt.Println(string(output))

	arch := getArchitecture(file)
	glibPath := glib(version, arch)
	if glibPath == "" {
		logFail("No suitable glib path found, cannot execute patchelf.")
		return
	}

	ld := getLdPath(glibPath, arch)
	executePatchelf(file, ld, glibPath)
	fixDebugSymbols(version, glibPath)
}

// ---------- patchelf 执行 ----------

func executePatchelf(file, ld, glibPath string) {
	logInfo("Applying patchelf...")
	cmd := exec.Command("patchelf", "--set-interpreter", ld, "--set-rpath", glibPath, file)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logFail(fmt.Sprintf("Error executing patchelf: %v\n%s", err, string(output)))
		return
	}
	logSuccess("Patchelf patched successfully.")
	fmt.Println(string(output))

	logInfo("Running ldd on patched file:")
	lddCmd := exec.Command("ldd", file)
	lddOutput, err := lddCmd.CombinedOutput()
	if err != nil {
		logFail(fmt.Sprintf("Error executing ldd: %v\n%s", err, string(lddOutput)))
		return
	}
	fmt.Println(string(lddOutput))
}

// ---------- 符号表修复 ----------

func fixDebugSymbols(version, glibPath string) {
	verParts := strings.Split(version, ".")
	if len(verParts) < 2 {
		logFail("Invalid version string: " + version)
		return
	}
	major, _ := strconv.Atoi(verParts[0])
	minor, _ := strconv.Atoi(verParts[1])

	debugDir := filepath.Join(glibPath, ".debug")

	arch := detectArch(glibPath)
	if arch == "" {
		logInfo("Unknown architecture, skip fixing debug symbols.")
		return
	}

	if major < 2 || (major == 2 && minor < 30) {
		libDir := fmt.Sprintf("lib/%s-linux-gnu", arch)
		src := filepath.Join(debugDir, libDir, fmt.Sprintf("libc-%s.so", version))
		dst := filepath.Join(debugDir, fmt.Sprintf("libc-%s.so", version))
		err := copyFile(src, dst)
		if err != nil {
			logFail("Error copying debug symbol: " + err.Error())
		} else {
			logSuccess("Copied debug symbol to: " + dst)
		}
	} else {
		filepath.Walk(debugDir+"/.build-id", func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			dst := filepath.Join(debugDir, info.Name())
			err = copyFile(path, dst)
			if err != nil {
				logFail("Failed copying: " + path)
			}
			return nil
		})
		logSuccess("Flattened .build-id symbols into .debug/")
	}
}

// ---------- 架构检测 ----------

func detectArch(glibPath string) string {
	if strings.Contains(glibPath, "i386") {
		return "i386"
	}
	if strings.Contains(glibPath, "amd64") || strings.Contains(glibPath, "x86_64") {
		return "x86_64"
	}
	return ""
}

// ---------- 文件复制 ----------

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ---------- 统一输出函数 ----------

func logSuccess(msg string) {
	fmt.Printf("\033[1;32m[✔] %s\033[0m\n", msg)
}

func logFail(msg string) {
	fmt.Printf("\033[1;31m[✘] %s\033[0m\n", msg)
}

func logInfo(msg string) {
	fmt.Printf("\033[1;33m[→] %s\033[0m\n", msg)
}

func logPrompt(msg string) {
	fmt.Printf("\033[1;36m[?] %s\033[0m\n", msg)
}

func checkErr(msg string, err error) {
	if err != nil {
		logFail(fmt.Sprintf("%s: %v", msg, err))
		os.Exit(1)
	}
}
