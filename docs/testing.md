# Elkeid Driver 测试指南 (Elkeid Driver Testing Guide)

## 文档概述 (Document Overview)

本文档提供 Elkeid Driver 的全面测试指南，包括功能测试、性能测试、压力测试和故障排查。

This document provides comprehensive testing guidelines for Elkeid Driver, including functional testing, performance testing, stress testing, and troubleshooting.

---

## 1. 测试环境准备 (Test Environment Setup)

### 1.1 虚拟机环境推荐

```bash
# 推荐使用虚拟机进行测试
# 避免在主机上直接测试，以防系统不稳定

# 创建测试虚拟机 (使用 virt-install 或其他虚拟化工具)
# 推荐配置:
#   - CPU: 2+ 核心
#   - 内存: 2GB+
#   - 磁盘: 20GB+
#   - 网络: NAT 或桥接

# 安装基础系统
# Ubuntu 20.04 LTS / CentOS 8 / Debian 10 等
```

### 1.2 内核版本检查

```bash
# 检查当前内核版本
uname -r

# 检查内核配置
cat /boot/config-$(uname -r) | grep -E "KPROBES|KRETPROBES"

# 预期输出:
# CONFIG_KPROBES=y
# CONFIG_HAVE_KPROBES=y
# CONFIG_KRETPROBES=y
```

### 1.3 快照备份 (重要!)

```bash
# 在测试前创建虚拟机快照
# 这样如果出现问题可以快速恢复

# VirtualBox:
# VBoxManage snapshot "TestVM" take "Before Elkeid Test"

# VMware:
# vmrun snapshot "[TestVM.vmx]" "Before Elkeid Test"

# libvirt (virsh):
# virsh snapshot-create-as --domain TestVM --name "before-elkeid-test"
```

---

## 2. 编译验证测试 (Build Verification)

### 2.1 清洁编译测试

```bash
cd driver/LKM

# 完全清理
make clean

# 编译
make

# 检查输出
ls -lh hids_driver.ko

# 验证模块信息
modinfo hids_driver.ko

# 预期输出:
# filename:       hids_driver.ko
# version:        1.7.0.24
# description:    Elkeid Driver is the core component...
# author:         Elkeid Team
# license:        GPL
```

### 2.2 模块完整性检查

```bash
# 检查模块符号
nm hids_driver.ko | grep -E "(smith_init|trace_init|hook_init)"

# 检查模块依赖
modprobe --show-depends hids_driver.ko

# 检查版本信息
modinfo -F vermagic hids_driver.ko

# 验证版本匹配
uname -r
modinfo -F vermagic hids_driver.ko
# 两者应该匹配
```

---

## 3. 基础功能测试 (Basic Functionality Tests)

### 3.1 模块加载测试

```bash
# 加载模块
sudo insmod hids_driver.ko

# 检查模块是否加载
lsmod | grep hids_driver

# 预期输出:
# hids_driver           123456  0
# (数字表示模块大小和引用计数)

# 查看内核日志
dmesg | tail -30

# 预期输出包含:
# [xxx] hids_driver: create XXX print event class
# [xxx] hids_driver: register 3 subsystems
```

### 3.2 通信接口验证

```bash
# 验证数据通道存在
ls -l /proc/elkeid-endpoint

# 预期输出:
# -r--r--r-- 1 root root 0 ... /proc/elkeid-endpoint
# (只读权限)

# 验证配置通道存在
ls -l /dev/hids_driver_allowlist

# 预期输出:
# crw-rw-rw- 1 root root 247, ... /dev/hids_driver_allowlist
# (字符设备，可读写)
```

### 3.3 事件数据读取测试

```bash
# 方法 1: 使用 cat 读取 (简单测试)
timeout 5 cat /proc/elkeid-endpoint | od -c | head -20

# 预期输出:
# 包含 \x17 和 \x1e 分隔符的事件数据

# 方法 2: 使用测试程序
cd driver/LKM/test
./rst

# 方法 3: 静默模式
./rst -q

# 方法 4: 后台运行并保存
./rst > /tmp/elkeid_events.log 2>&1 &

# 在另一个终端触发一些事件
ls /tmp
cat /etc/passwd
ping -c 1 8.8.8.8

# 检查日志
cat /tmp/elkeid_events.log | head -50
```

### 3.4 模块卸载测试

```bash
# 检查是否有进程在使用接口
lsof | grep elkeid-endpoint

# 如果有，先终止它们
kill -9 <PID>

# 卸载模块
sudo rmmod hids_driver

# 验证卸载
lsmod | grep hids_driver
# 应该没有输出

# 查看日志
dmesg | tail -10
```

---

## 4. 白名单功能测试 (Whitelist Function Tests)

### 4.1 白名单操作测试

```bash
# 确保模块已加载
sudo insmod hids_driver.ko

# 测试 1: 添加白名单
echo "Y/usr/bin/ls" | sudo tee /dev/hids_driver_allowlist
# 预期: 返回写入字节数

# 测试 2: 检查是否在白名单中
echo "y/usr/bin/ls" | sudo tee /dev/hids_driver_allowlist
# 预期: dmesg 显示 "[ELKEID DEBUG] execve_exe_check:/usr/bin/ls 1"

# 测试 3: 添加多个路径
echo "Y/usr/bin/cat" | sudo tee /dev/hids_driver_allowlist
echo "Y/usr/bin/bash" | sudo tee /dev/hids_driver_allowlist

# 测试 4: 打印所有白名单
echo "." | sudo tee /dev/hids_driver_allowlist
# 预期: dmesg 显示所有白名单项

# 测试 5: 删除白名单项
echo "D/usr/bin/ls" | sudo tee /dev/hids_driver_allowlist

# 测试 6: 清空所有白名单
echo "w" | sudo tee /dev/hids_driver_allowlist
```

### 4.2 白名单效果验证

```bash
# 添加 ls 到白名单
echo "Y/usr/bin/ls" | sudo tee /dev/hids_driver_allowlist

# 在另一个终端读取事件
./rst -q > /tmp/test_events.log &

# 执行 ls
ls -la /tmp

# 等待几秒后停止
kill %1

# 检查日志
grep -i "/usr/bin/ls" /tmp/test_events.log
# 预期: 如果白名单生效，应该没有或只有少量 ls 的事件

# 清空白名单
echo "w" | sudo tee /dev/hids_driver_allowlist

# 再次测试
./rst -q > /tmp/test_events2.log &
ls -la /tmp
kill %1

grep -i "/usr/bin/ls" /tmp/test_events2.log
# 预期: 应该有大量 ls 的事件
```

---

## 5. 事件类型测试 (Event Type Tests)

### 5.1 进程事件测试

```bash
# 启动事件捕获
./rst -q > /tmp/process_events.log &
TEST_PID=$!

# 触发 execve 事件
/bin/echo "test"
/usr/bin/ls /tmp

# 触发 fork 事件
/bin/bash -c 'sleep 1 &'

# 等待事件收集
sleep 2

# 停止捕获
kill $TEST_PID

# 分析结果
cat /tmp/process_events.log | grep -E "data_type.*59|data_type.*100"
# 59 = execve, 100 = fork (实际 ID 可能不同)
```

### 5.2 网络事件测试

```bash
# 启动事件捕获
./rst -q > /tmp/network_events.log &
TEST_PID=$!

# 触发 connect 事件
curl -s https://www.baidu.com > /dev/null
ping -c 1 8.8.8.8

# 触发 DNS 查询
nslookup google.com

# 等待事件收集
sleep 2

# 停止捕获
kill $TEST_PID

# 分析结果
cat /tmp/network_events.log | grep -E "data_type.*42|data_type.*601"
# 42 = connect, 601 = dns_query
```

### 5.3 文件事件测试

```bash
# 启动事件捕获
./rst -q > /tmp/file_events.log &
TEST_PID=$!

# 触发 open 事件
cat /etc/hostname > /tmp/test_read.txt
echo "test" > /tmp/test_write.txt

# 触发 rename 事件
mv /tmp/test_write.txt /tmp/test_renamed.txt

# 等待事件收集
sleep 2

# 停止捕获
kill $TEST_PID

# 分析结果
cat /tmp/file_events.log | grep -E "data_type.*1|data_type.*2|data_type.*82"
# 1 = write, 2 = open, 82 = rename
```

---

## 6. 性能测试 (Performance Tests)

### 6.1 Hook 延迟测试

```bash
# 使用 ftrace 测量 kprobe 延迟

# 启用 ftrace
cd /sys/kernel/debug/tracing
echo 1 > tracing_on
echo function_graph > current_tracer

# 过滤 hids_driver 相关函数
echo 'hids*' > set_ftrace_filter

# 读取 trace
cat trace_pipe > /tmp/ftrace_output.log &

# 在另一个终端执行一些操作
for i in {1..100}; do ls /tmp; done

# 停止 trace
echo 0 > tracing_on

# 分析结果
cat /tmp/ftrace_output.log | grep -A 5 "do_sys_open"

# 计算 Hook 延迟
# 查看从 kprobe handler 到实际函数调用的延迟
# 预期: TP99 < 3.5us
```

### 6.2 吞吐量测试

```bash
# 测试事件吞吐量

# 启动事件捕获
./rst -q > /tmp/throughput_test.log &
TEST_PID=$!

# 生成大量事件
for i in {1..1000}; do
    ls /tmp >/dev/null
    cat /etc/hostname >/dev/null
done

# 等待事件收集
sleep 2

# 停止捕获
kill $TEST_PID

# 统计事件数量
EVENT_COUNT=$(wc -l < /tmp/throughput_test.log)
echo "Total events: $EVENT_COUNT"

# 预期: 应该接近 2000 (1000 ls + 1000 cat)
# 检查是否有事件丢失
dmesg | grep -i "lost\|drop"
```

### 6.3 CPU 占用测试

```bash
# 测试 CPU 占用率

# 启动事件捕获
./rst -q > /dev/null &
TEST_PID=$!

# 记录初始 CPU 时间
CPU_BEFORE=$(ps -p $TEST_PID -o %cpu | tail -1)

# 生成大量事件
for i in {1..1000}; do
    ls /tmp >/dev/null
done

sleep 1

# 记录最终 CPU 时间
CPU_AFTER=$(ps -p $TEST_PID -o %cpu | tail -1)

echo "CPU usage: $CPU_BEFORE -> $CPU_AFTER"

# 清理
kill $TEST_PID

# 预期: CPU 占用率应该较低 (< 10%)
```

---

## 7. LTP 集成测试 (LTP Integration Tests)

### 7.1 安装 LTP

```bash
# Ubuntu/Debian
sudo apt install ltp

# CentOS/RHEL
sudo yum install ltp

# 或从源码编译
git clone https://github.com/linux-test-project/ltp.git
cd ltp
./configure
make
sudo make install
```

### 7.2 运行 LTP 测试

```bash
# 进入 LTP 测试目录
cd /opt/ltp/tests

# 运行系统调用测试
sudo ./syscalls.sh

# 运行文件系统测试
sudo ./fs.sh

# 运行进程测试
sudo ./process.sh

# 检查是否有崩溃
dmesg | grep -i "bug\|panic\|oops"
```

### 7.3 使用 Elkeid 的 LTP 配置

```bash
# 使用项目提供的 LTP 测试配置
cd driver/LKM

# 查看配置文件
cat ltp_testcase

# 运行指定的测试用例
# 例如: execve, open, connect 等系统调用测试
for test in execve01 execve02 open01 open01 connect01; do
    echo "Running $test..."
    sudo /opt/ltp/testcases/bin/$test
done

# 检查内核日志
dmesg | tail -50
```

### 7.4 KASAN 测试 (可选)

```bash
# KASAN (Kernel Address SANitizer) 可以检测内存错误

# 检查内核是否支持 KASAN
cat /boot/config-$(uname -r) | grep KASAN

# 如果支持，在 KASAN 内核上加载驱动
# 这需要重新编译内核并启用 KASAN

# 运行 LTP 测试
sudo ./syscalls.sh

# 检查 KASAN 报告
dmesg | grep -A 20 "KASAN"
```

---

## 8. 压力测试 (Stress Tests)

### 8.1 系统调用压力测试

```bash
# 使用 stress 工具
sudo apt install stress

# 启动事件捕获
./rst -q > /tmp/stress_test.log &
TEST_PID=$!

# 运行压力测试
stress --cpu 4 --io 4 --vm 2 --vm-bytes 128M --timeout 60s &

# 在另一个终端持续生成事件
while true; do
    ls /tmp >/dev/null
    cat /etc/hostname >/dev/null
    ping -c 1 127.0.0.1 >/dev/null 2>&1
done

# 等待 60 秒
sleep 60

# 停止测试
killall stress
kill $TEST_PID

# 检查系统稳定性
dmesg | grep -i "error\|panic\|bug"

# 检查事件丢失
dmesg | grep -i "lost\|drop"
```

### 8.2 缓冲区溢出测试

```bash
# 测试缓冲区满时的行为

# 启动事件捕获 (不读取，让缓冲区填满)
# 只启动驱动，不启动读取程序
sudo insmod hids_driver.ko

# 生成大量事件
for i in {1..10000}; do
    ls /tmp >/dev/null
    echo $i > /tmp/test_$i
done

# 检查缓冲区状态
# 使用 IOCTL 获取统计信息
# (需要编写测试程序来调用 TRACE_IOCTL_STAT)

# 检查内核日志
dmesg | tail -50

# 预期:
# - 如果是覆盖模式: 旧事件被覆盖
# - 如果是非覆盖模式: 新事件被丢弃
```

---

## 9. 故障排查测试 (Troubleshooting Tests)

### 9.1 内存泄漏检测

```bash
# 检查内核模块内存使用

# 记录初始状态
sudo cat /proc/modules | grep hids_driver
sudo cat /proc/meminfo | grep Slab

# 运行一段时间的事件采集
./rst -q > /dev/null &
TEST_PID=$!

# 生成大量事件
for i in {1..1000}; do
    ls /tmp >/dev/null
done

sleep 5

# 检查内存使用变化
sudo cat /proc/modules | grep hids_driver
sudo cat /proc/meminfo | grep Slab

# 清理
kill $TEST_PID
sudo rmmod hids_driver

# 预期: 内存使用应该稳定，不应该持续增长
```

### 9.2 并发问题测试

```bash
# 测试多进程并发读取

# 启动多个读取进程
for i in {1..5}; do
    ./rst -q > /tmp/concurrent_$i.log &
done

# 生成事件
for i in {1..100}; do
    ls /tmp >/dev/null
done

sleep 2

# 停止所有读取进程
killall rst

# 检查每个进程的日志
for i in {1..5}; do
    echo "Process $i:"
    wc -l /tmp/concurrent_$i.log
done

# 检查系统日志
dmesg | grep -i "error\|race\|lock"

# 预期: 所有进程都应该能正常读取，没有死锁或竞争
```

### 9.3 恢复测试

```bash
# 测试从异常状态恢复

# 测试 1: 加载 -> 卸载 -> 重新加载
sudo insmod hids_driver.ko
sudo rmmod hids_driver
sudo insmod hids_driver.ko

# 测试 2: 中断读取进程
./rst > /tmp/test.log &
PID=$!
sleep 1
kill -9 $PID

# 重新启动读取
./rst > /tmp/test2.log &
sleep 1
killall rst

# 测试 3: 在缓冲区满时卸载
# 填满缓冲区
for i in {1..10000}; do ls /tmp >/dev/null; done
# 立即卸载
sudo rmmod hids_driver

# 检查是否有错误
dmesg | tail -20

# 预期: 所有操作都应该正常完成，没有内核 panic 或 oops
```

---

## 10. 自动化测试脚本 (Automated Test Scripts)

### 10.1 基础测试脚本

```bash
#!/bin/bash
# basic_test.sh - 基础功能自动化测试

set -e

echo "=== Elkeid Driver Basic Test ==="

# 检查模块是否已加载
if lsmod | grep -q hids_driver; then
    echo "Module already loaded, unloading..."
    sudo rmmod hids_driver
fi

# 编译
echo "Building module..."
cd driver/LKM
make clean && make

# 加载模块
echo "Loading module..."
sudo insmod hids_driver.ko

# 验证接口
echo "Verifying interfaces..."
if [ ! -e /proc/elkeid-endpoint ]; then
    echo "ERROR: /proc/elkeid-endpoint not found"
    exit 1
fi

if [ ! -e /dev/hids_driver_allowlist ]; then
    echo "ERROR: /dev/hids_driver_allowlist not found"
    exit 1
fi

# 测试白名单
echo "Testing allowlist..."
echo "Y/usr/bin/test" | sudo tee /dev/hids_driver_allowlist > /dev/null
echo "w" | sudo tee /dev/hids_driver_allowlist > /dev/null

# 测试事件读取
echo "Testing event reading..."
timeout 5 cat /proc/elkeid-endpoint | head -c 100 > /dev/null

# 卸载模块
echo "Unloading module..."
sudo rmmod hids_driver

echo "=== All tests passed ==="
```

### 10.2 压力测试脚本

```bash
#!/bin/bash
# stress_test.sh - 压力测试脚本

echo "=== Elkeid Driver Stress Test ==="

# 加载模块
sudo insmod hids_driver.ko

# 启动事件捕获
cd driver/LKM/test
./rst -q > /tmp/stress_test.log &
TEST_PID=$!

echo "Running stress test for 60 seconds..."

# 后台压力测试
stress --cpu 4 --io 4 --timeout 60s &
STRESS_PID=$!

# 事件生成
for i in {1..10000}; do
    ls /tmp >/dev/null 2>&1
    cat /etc/hostname >/dev/null 2>&1
    if [ $((i % 1000)) -eq 0 ]; then
        echo "Generated $i events..."
    fi
done &

# 等待测试完成
sleep 60

# 清理
kill $STRESS_PID 2>/dev/null
kill $TEST_PID

# 分析结果
EVENT_COUNT=$(wc -l < /tmp/stress_test.log)
echo "Total events collected: $EVENT_COUNT"

# 检查错误
if dmesg | grep -qi "error\|panic\|bug"; then
    echo "ERROR: Kernel errors detected!"
    dmesg | grep -i "error\|panic\|bug" | tail -20
    exit 1
fi

echo "=== Stress test completed ==="
```

---

## 11. 测试报告模板 (Test Report Template)

### 11.1 测试报告结构

```
# Elkeid Driver 测试报告

## 测试环境
- 操作系统: Ubuntu 20.04 LTS
- 内核版本: 5.4.0-generic
- CPU: 4 核心
- 内存: 4GB
- Driver 版本: 1.7.0.24

## 测试结果

### 编译测试
- [x] 清洁编译通过
- [x] 模块信息正确

### 功能测试
- [x] 模块加载成功
- [x] 模块卸载成功
- [x] 通信接口正常
- [x] 事件采集正常
- [x] 白名单功能正常

### 性能测试
- Hook 延迟: 平均 2.1us, TP99 3.2us
- 吞吐量: ~10000 事件/秒
- CPU 占用: < 5%

### 稳定性测试
- [x] 压力测试通过 (60 秒)
- [x] 无内存泄漏
- [x] 无内核 panic
- [x] 无死锁

### 已知问题
- 无

### 建议
- 建议在生产环境部署前进行更长时间的稳定性测试
```

---

## 12. 调试技巧 (Debugging Tips)

### 12.1 启用详细日志

```bash
# 在编译时添加调试标志
make EXTRA_CFLAGS="-DDEBUG"

# 重新加载模块
sudo rmmod hids_driver
sudo insmod hids_driver.ko

# 查看详细日志
dmesg -w | grep -i elkeid
```

### 12.2 使用 crash 工具分析

```bash
# 如果系统崩溃，使用 crash 分析
sudo apt install crash

# 获取调试符号
# (需要安装对应内核的 debugging symbols)

# 分析崩溃
sudo crash /usr/lib/debug/lib/modules/$(uname -r)/vmlinux \
            /proc/kcore

# 在 crash 中:
#> bt  # 查看堆栈
#> mod -s hids_driver /path/to/hids_driver.ko
#> dis -l hids_driver_init  # 反汇编
```

---

**文档版本**: 1.0
**最后更新**: 2024
**维护者**: Elkeid Team
