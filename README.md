# cpwn

> Patch ELF binaries to use custom glibc versions for compatibility and debugging.

`cpwn` 是一个用于快速修复 ELF 可执行文件 `ld.so` 路径和 `rpath` 的工具，适用于 glibc 降级调试、ctf 题目环境复现、兼容性测试等场景。它支持自动识别架构，自动选择匹配版本的 glibc，并执行 `patchelf` 和符号表处理。

---

## ✨ 功能特性

- ✅ 自动识别 ELF 架构（i386 / amd64）
- ✅ 支持指定 glibc 版本并自动修复 ELF
- ✅ 支持 `.cpwn` 配置，统一管理 glibc 库路径
- ✅ 支持 fallback 到旧版本列表（`old_list`）
- ✅ 自动 自动修复符号表，兼容 GDB 调试

---

## 📦 安装

```bash
git clone https://github.com/yourname/cpwn.git
cd cpwn
go build -o cpwn
```

---

## 🛠 使用方法

### 设置 glibc 基础目录（例如你的 glibc 库放在 `/opt/glibc`）

```bash
./cpwn -s 你的glibc-all-in-one目录
```

### 修复 ELF 可执行文件使用指定 glibc 版本：

```bash
./cpwn ./test_binary 2.27
```

### 示例输出：

```text
[→] File architecture: amd64
[✔] Patchelf patched successfully.
[→] Running ldd on patched file:
    linux-vdso.so.1 (0x00007ffcdd9f9000)
    libc.so.6 => /opt/glibc/libs/2.27-amd64/libc.so.6 (0x00007fcd18b00000)
    ...
```

---

## 📁 目录结构要求

基础目录 `.cpwn` 里结构应如下（通过 `-s` 设置）：

- `list` 和 `old_list`: 包含所有可用 glibc 版本名的文本列表
- `download` / `download_old`: 用于下载和解压指定版本的脚本

---

## 🔧 依赖项

- [Go](https://golang.org/) 1.18+
- [`patchelf`](https://github.com/NixOS/patchelf)

---

## 📜 License

MIT License © 2025 [Ruoqian](https://github.com/yourname)

---

## ❤️ 致谢

本项目灵感来自于 CTF 和安全研究中频繁遇到的 libc 降级复现问题。
