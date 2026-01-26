# Elkeid Driver 编译指南 (Elkeid Driver Build Guide)

## 文档概述 (Document Overview)

本文档详细描述如何在不同 Linux 发行版上编译 Elkeid Driver 内核模块。

This document provides detailed instructions for building the Elkeid Driver kernel module on different Linux distributions.

---

## 1. 环境要求 (Prerequisites)

### 1.1 系统要求

| 项目 | 要求 |
|------|------|
| 操作系统 | Linux (内核 2.6.32 - 6.3) |
| 架构 | x86_64, ARM64 (AArch64), RISC-V |
| 内存 | 最少 2GB RAM |
| 磁盘 | 最少 500MB 可用空间 |
| 权限 | root 或 sudo 权限 (用于加载模块) |

### 1.2 支持的发行版 (Supported Distributions)

| 发行版 | 版本 | 内核范围 | 状态 |
|--------|------|----------|------|
| Debian | 8, 9, 10 | 3.16 - 5.4.x | ✅ 完全支持 |
| Ubuntu | 14.04, 16.04, 18.04, 20.04+ | 3.12 - 5.4.x | ✅ 完全支持 |
| CentOS | 6.x, 7.x, 8.x / RHEL | 2.6.32 - 5.4.x | ✅ 完全支持 |
| Amazon Linux | 2 | 4.9 - 4.14.x | ✅ 完全支持 |
| Alibaba Cloud Linux | 3 | 4.19 - 5.10.x | ✅ 完全支持 |
| EulerOS | V2.0 | 3.10.x | ✅ 完全支持 |

---

## 2. 依赖安装 (Dependency Installation)

### 2.1 Debian / Ubuntu

```bash
# 更新包列表
sudo apt update

# 安装内核开发包和构建工具
sudo apt install -y \
    linux-headers-$(uname -r) \
    build-essential \
    git

# 验证安装
uname -r                    # 显示当前内核版本
ls /lib/modules/$(uname -r)/build   # 检查内核头文件
```

### 2.2 CentOS / RHEL / AlmaLinux / Rocky Linux

```bash
# CentOS/RHEL 7
sudo yum install -y \
    kernel-devel-$(uname -r) \
    kernel-headers-$(uname -r) \
    gcc \
    make \
    git

# CentOS/RHEL 8 / AlmaLinux / Rocky Linux
sudo dnf install -y \
    kernel-devel-$(uname -r) \
    kernel-headers-$(uname -r) \
    gcc \
    make \
    git

# 验证安装
uname -r
ls /usr/src/kernels/$(uname -r)
```

### 2.3 Amazon Linux 2

```bash
sudo yum install -y \
    kernel-devel-$(uname -r) \
    gcc \
    make \
    git

# 验证安装
uname -r
ls /usr/src/kernels/
```

### 2.4 Fedora

```bash
sudo dnf install -y \
    kernel-devel-$(uname -r) \
    gcc \
    make \
    git

# 验证安装
uname -r
ls /lib/modules/$(uname -r)/build
```

### 2.5 Arch Linux

```bash
sudo pacman -S \
    linux-headers \
    base-devel \
    git

# 验证安装
uname -r
ls /lib/modules/$(uname -r)/build
```

---

## 3. 获取源码 (Getting Source Code)

### 3.1 克隆完整仓库

```bash
# 克隆 Elkeid 仓库
git clone https://github.com/bytedance/Elkeid.git
cd Elkeid

# 或使用 SSH
git clone git@github.com:bytedance/Elkeid.git
cd Elkeid
```

### 3.2 仅下载 Driver 部分

```bash
# 使用 git sparse-checkout (Git 2.25+)
mkdir Elkeid && cd Elkeid
git init
git remote add origin https://github.com/bytedance/Elkeid.git
git sparse-checkout init
git sparse-checkout set driver
git pull origin main
```

### 3.3 下载预编译模块 (可选)

```bash
# 下载预编译的内核模块
wget "http://lf26-elkeid.bytetos.com/obj/elkeid-download/ko/hids_driver_1.7.0.10_$(uname -r)_amd64.ko" -O hids_driver.ko

# 注意：预编译模块可能不适用于所有内核版本
```

---

## 4. 编译流程 (Build Process)

### 4.1 标准编译

```bash
# 进入驱动目录
cd driver/LKM

# 清理旧的构建文件
make clean

# 编译内核模块
make

# 编译输出
# hids_driver.ko - 内核模块文件
```

### 4.2 批量编译 (不编译测试程序)

```bash
# 仅编译内核模块，不编译测试程序
BATCH=true make
```

### 4.3 交叉编译 (Cross Compilation)

```bash
# 为不同的内核版本编译
KVERSION=5.4.0-generic make

# 指定内核目录
make KERNELDIR=/path/to/kernel/source
```

### 4.4 CentOS 专用构建脚本

```bash
# CentOS 系统使用专用脚本
sh ./centos_build_ko.sh
```

### 4.5 编译输出说明

```
编译成功后，以下文件将生成：
├── hids_driver.ko           # 内核模块（主要产物）
├── hids_driver.o            # 目标文件
├── hids_driver.mod.o        # 模块目标文件
├── hids_driver.mod.c        # 模块源代码（自动生成）
├── Module.symvers           # 符号版本
├── modules.order            # 模块顺序
└── .tmp_versions/           # 临时版本信息

test/
├── rst                      # 用户空间测试程序
└── main.o                   # 测试程序目标文件
```

---

## 5. Makefile 详解 (Makefile Details)

### 5.1 可用目标 (Available Targets)

| 目标 | 描述 |
|------|------|
| `all` | 编译内核模块和测试程序 (默认) |
| `clean` | 清理所有生成的文件 |
| `modules` | 仅编译内核模块 |
| `test` | 编译测试程序 |
| `insmod` | 加载内核模块 |
| `rmmod` | 卸载内核模块 |

### 5.2 环境变量 (Environment Variables)

| 变量 | 描述 | 默认值 |
|------|------|--------|
| `KVERSION` | 目标内核版本 | `uname -r` |
| `KERNELDIR` | 内核源码目录 | 自动检测 |
| `BATCH` | 不编译测试程序 | false |

### 5.3 内核兼容性检测 (Kernel Compatibility Detection)

Makefile 会自动检测以下内核特性：

```makefile
# 模块内存布局检测
KMOD_CORE_LAYOUT    # 旧内核: module.core
KMOD_MODULE_MEM     # 新内核: module_memory

# class_create API 变化
CLASS_CREATE_HAVE_OWNER

# kgid_t 类型检查
KGID_STRUCT_CHECK
KGID_CONFIG_CHECK

# IPv6 支持
IPV6_SUPPORT

# uaccess API 变化
UACCESS_TYPE_SUPPORT

# trace_seq 结构变化
SMITH_TRACE_SEQ
SMITH_TRACE_READPOS
SMITH_TRACE_FULL
SMITH_TRACE_LEN

# 文件系统操作
SMITH_FS_OP_ITERATE
SMITH_FS_OP_ITERATE_SHARED
SMITH_FS_FILE_REF

# procfs API 变化
SMITH_PROCFS_PDE_DATA
SMITH_PROCFS_pde_data
```

---

## 6. 测试与验证 (Testing & Verification)

### 6.1 模块信息查看

```bash
# 查看模块信息
modinfo hids_driver.ko

# 输出示例：
# filename:       hids_driver.ko
# version:        1.7.0.24
# description:    Elkeid Driver is the core component of Elkeid HIDS project
# author:         Elkeid Team <elkeid@bytedance.com>
# license:        GPL
# srcversion:     XXXXXXXX
# depends:
# vermagic:       5.4.0-generic SMP mod_unload
```

### 6.2 符号表检查

```bash
# 查看模块符号
nm hids_driver.ko | grep -E "(init|exit|hook)"

# 查看未定义符号
nm hids_driver.ko | grep U
```

### 6.3 加载测试

```bash
# 加载模块
sudo insmod hids_driver.ko

# 检查模块是否加载成功
lsmod | grep hids_driver

# 查看内核日志
dmesg | tail -20

# 预期输出包含：
# hids_driver: create XXX print event class
# hids_driver: register 3 subsystems
```

### 6.4 通信接口验证

```bash
# 验证数据通道
ls -l /proc/elkeid-endpoint

# 验证配置通道
ls -l /dev/hids_driver_allowlist

# 测试数据读取
timeout 5 cat /proc/elkeid-endpoint | od -c | head -20
```

### 6.5 运行测试程序

```bash
# 进入测试目录
cd driver/LKM/test

# 编译测试程序（如果尚未编译）
make

# 运行测试程序（读取事件数据）
sudo ./rst

# 运行测试程序（静默模式）
sudo ./rst -q

# 预期输出：安全事件数据流
```

### 6.6 白名单配置测试

```bash
# 添加到白名单
echo "Y/usr/bin/ls" | sudo tee /dev/hids_driver_allowlist

# 打印所有白名单
echo "." | sudo tee /dev/hids_driver_allowlist

# 从白名单删除
echo "D/usr/bin/ls" | sudo tee /dev/hids_driver_allowlist

# 清空所有白名单
echo "w" | sudo tee /dev/hids_driver_allowlist
```

### 6.7 卸载模块

```bash
# 卸载模块
sudo rmmod hids_driver

# 验证卸载
lsmod | grep hids_driver  # 应该没有输出

# 查看日志
dmesg | tail -10
```

---

## 7. 故障排除 (Troubleshooting)

### 7.1 常见编译错误

#### 错误 1: 找不到内核头文件

```
error: linux/module.h: No such file or directory
```

**解决方案:**
```bash
# 检查是否安装了内核头文件
ls /lib/modules/$(uname -r)/build

# 如果不存在，安装内核头文件
# Debian/Ubuntu
sudo apt install linux-headers-$(uname -r)

# CentOS/RHEL
sudo yum install kernel-devel-$(uname -r)
```

#### 错误 2: 版本不匹配

`````
version magic '5.4.0-generic' should be '5.4.0-custom'
````

**解决方案:**
```bash
# 确保编译时的内核版本与运行时一致
uname -r  # 记录当前版本

# 如果使用 DKMS，重新构建
sudo dkms remove hids_driver/1.7.0.24 --all || true
sudo dkms install hids_driver/1.7.0.24 -k $(uname -r)
```

#### 错误 3: 权限错误

```
Permission denied
```

**解决方案:**
```bash
# 使用 sudo 执行需要 root 权限的操作
sudo insmod hids_driver.ko
sudo rmmod hids_driver
```

#### 错误 4: 未知符号

```
Unknown symbol xxx
```

**解决方案:**
```bash
# 检查内核配置
# 某些功能需要在内核编译时启用
# 例如：KPROBES, KRETPROBES

# 检查符号是否在内核中导出
grep -r "EXPORT.*symbol_name" /lib/modules/$(uname -r)/build/
```

#### 错误 5: GCC 版本不兼容

```
error: expected declaration specifiers or '...' before numeric constant
```

**解决方案:**
```bash
# 检查 GCC 版本
gcc --version

# 如果使用旧内核，可能需要旧版本的 GCC
# CentOS 7: sudo yum install gcc-6
# Ubuntu: sudo apt install gcc-6

# 使用指定的 GCC 编译
make CC=gcc-6
```

### 7.2 调试技巧

#### 启用详细输出

```bash
# 使用 make verbose
make V=1

# 查看完整的编译命令
make V=1 2>&1 | tee build.log
```

#### 检查内核配置

```bash
# 检查 KPROBE 支持
grep CONFIG_KPROBES /boot/config-$(uname -r)

# 应该显示：
# CONFIG_KPROBES=y

# 检查必需的内核选项
grep -E "CONFIG_KPROBES|CONFIG_KRETPROBES|CONFIG_HAVE_KPROBES" /boot/config-$(uname -r)
```

#### 模块加载失败调试

```bash
# 查看详细的加载失败信息
sudo insmod hids_driver.ko

# 立即查看 dmesg
dmesg | tail -30

# 使用 modprobe 获取更多信息
sudo modprobe hids_driver
```

#### 符号依赖检查

```bash
# 检查模块依赖的符号
modprobe --show-depends hids_driver.ko

# 查看模块使用的系统调用
strace -e openat,read,write insmod hids_driver.ko 2>&1 | grep -v "= -1"
```

### 7.3 性能分析

```bash
# 使用 ftrace 测量 kprobe 延迟
cd /sys/kernel/debug/tracing
echo 1 > kprobe_events
echo 'p do_sys_open' >> kprobe_events
echo 1 > enable
cat trace_pipe

# 查看统计信息
cat tracing_on
cat trace
```

---

## 8. 高级编译选项 (Advanced Build Options)

### 8.1 DKMS 集成 (Dynamic Kernel Module Support)

DKMS 允许内核模块在内核升级后自动重新编译。

```bash
# 创建 DKMS 目录
sudo mkdir -p /usr/src/hids_driver-1.7.0.24
sudo cp -r driver/LKM/* /usr/src/hids_driver-1.7.0.24/

# 创建 dkms.conf
cat <<EOF | sudo tee /usr/src/hids_driver-1.7.0.24/dkms.conf
PACKAGE_NAME="hids_driver"
PACKAGE_VERSION="1.7.0.24"
BUILT_MODULE_NAME[0]="hids_driver"
DEST_MODULE_LOCATION[0]="/kernel/drivers/misc"
AUTOINSTALL="yes"
MAKE[0]="make -C \${kernel_source_dir} M=\${dkms_tree}/\${PACKAGE_NAME}/\${PACKAGE_VERSION}/build modules"
CLEAN="make -C \${kernel_source_dir} M=\${dkms_tree}/\${PACKAGE_NAME}/\${PACKAGE_VERSION}/build clean"
EOF

# 添加到 DKMS
sudo dkms add -m hids_driver -v 1.7.0.24

# 构建模块
sudo dkms build -m hids_driver -v 1.7.0.24

# 安装模块
sudo dkms install -m hids_driver -v 1.7.0.24

# 验证
dkms status
```

### 8.2 自定义编译选项

```bash
# 添加调试符号
make EXTRA_CFLAGS="-g -O0"

# 启用特定优化
make EXTRA_CFLAGS="-O2"

# 添加自定义定义
make EXTRA_CFLAGS="-DDEBUG_BUILD"
```

### 8.3 多内核版本编译

```bash
# 为多个已安装的内核版本编译
for kver in $(ls /lib/modules); do
    if [ -d "/lib/modules/$kver/build" ]; then
        echo "Building for kernel $kver"
        make clean
        make KVERSION=$kver
        cp hids_driver.ko "hids_driver_$kver.ko"
    fi
done
```

---

## 9. 持续集成 (CI/CD)

### 9.1 GitHub Actions 示例

```yaml
name: Build Elkeid Driver

on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        ubuntu: [18.04, 20.04, 22.04]
    steps:
      - uses: actions/checkout@v3
      - name: Install dependencies
        run: |
          sudo apt update
          sudo apt install -y linux-headers-$(uname -r) build-essential
      - name: Build driver
        run: |
          cd driver/LKM
          make clean && make
      - name: Upload artifacts
        uses: actions/upload-artifact@v3
        with:
          name: hids-driver-${{ matrix.ubuntu }}
          path: driver/LKM/hids_driver.ko
```

### 9.2 本地测试脚本

```bash
#!/bin/bash
# test_build.sh - 自动化构建测试脚本

set -e

echo "=== Elkeid Driver Build Test ==="

# 检查依赖
echo "Checking dependencies..."
if [ ! -d "/lib/modules/$(uname -r)/build" ]; then
    echo "ERROR: Kernel headers not found"
    exit 1
fi

# 清理
echo "Cleaning..."
make clean

# 构建
echo "Building..."
make

# 验证
echo "Verifying..."
if [ ! -f "hids_driver.ko" ]; then
    echo "ERROR: Build failed, hids_driver.ko not found"
    exit 1
fi

# 显示信息
echo "Module info:"
modinfo hids_driver.ko

echo "=== Build successful ==="
```

---

## 10. 生产环境部署建议

### 10.1 部署前检查清单

- [ ] 在测试环境验证
- [ ] 检查内核版本兼容性
- [ ] 备份重要数据
- [ ] 准备回滚方案
- [ ] 监控系统资源
- [ ] 配置日志收集

### 10.2 安全建议

```bash
# 设置模块权限
chmod 644 hids_driver.ko

# 使用 DKMS 自动管理
sudo dkms install hids_driver/1.7.0.24

# 配置自动加载
cat <<EOF | sudo tee /etc/modules-load.d/hids_driver.conf
hids_driver
EOF
```

### 10.3 监控与日志

```bash
# 监控模块状态
watch -n 5 'lsmod | grep hids_driver'

# 持续查看日志
tail -f /var/log/kern.log | grep -i elkeid

# 或使用 dmesg
dmesg -w | grep -i elkeid
```

---

## 11. 参考资料 (References)

- [Linux Kernel Module Programming Guide](https://sysprog21.github.io/lkmpg/)
- [Kprobe Documentation](https://www.kernel.org/doc/html/latest/trace/kprobes.html)
- [Elkeid GitHub](https://github.com/bytedance/Elkeid)
- [Elkeid 官方文档](https://bytedance.github.io/Elkeid/)

---

**文档版本**: 1.0
**最后更新**: 2024
**维护者**: Elkeid Team
