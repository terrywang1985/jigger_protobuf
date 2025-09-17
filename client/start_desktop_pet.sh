#!/bin/bash

# 桌面宠物游客登录启动脚本
# 支持Linux和macOS系统

echo "=========================================="
echo "         桌面宠物游客登录版本"
echo "=========================================="
echo
echo "功能特性:"
echo "  ✓ 游客快速登录，无需手机验证"
echo "  ✓ 多玩家在线互动"
echo "  ✓ 实时聊天功能"
echo "  ✓ 动作同步显示"
echo

# 检查Python是否安装
echo "正在检查环境..."
if ! command -v python3 &> /dev/null; then
    if ! command -v python &> /dev/null; then
        echo "[错误] 未找到Python，请先安装Python 3.7+"
        exit 1
    else
        PYTHON_CMD="python"
    fi
else
    PYTHON_CMD="python3"
fi

echo "[✓] Python环境正常"

# 检查pip是否可用
if ! command -v pip3 &> /dev/null; then
    if ! command -v pip &> /dev/null; then
        echo "[错误] 未找到pip，请先安装pip"
        exit 1
    else
        PIP_CMD="pip"
    fi
else
    PIP_CMD="pip3"
fi

# 检查必要模块
echo "正在检查必要模块..."
$PYTHON_CMD -c "import websockets, requests, PIL, pynput" 2>/dev/null
if [ $? -ne 0 ]; then
    echo "[!] 检测到缺少必要模块，正在安装..."
    $PIP_CMD install websockets requests pillow pynput
    if [ $? -ne 0 ]; then
        echo "[错误] 模块安装失败，请手动安装:"
        echo "$PIP_CMD install websockets requests pillow pynput"
        exit 1
    fi
fi

echo "[✓] 依赖模块正常"

# 检查服务器状态
echo "正在检查服务器状态..."
if command -v curl &> /dev/null; then
    curl -s -m 3 http://localhost:8081/health >/dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo "[!] 登录服务器未启动 (端口8081)"
        echo "请先启动服务器，或者选择继续（将以离线模式运行）"
        echo
        read -p "是否继续启动客户端? (y/n): " choice
        if [ "$choice" != "y" ] && [ "$choice" != "Y" ]; then
            echo "程序退出"
            exit 1
        fi
    else
        echo "[✓] 登录服务器正常"
    fi
else
    echo "[!] 未找到curl命令，跳过服务器检查"
fi

echo
echo "启动桌面宠物客户端..."
echo

# 切换到脚本所在目录
cd "$(dirname "$0")"

# 启动客户端
$PYTHON_CMD desktop_pet_guest.py

# 检查启动结果
if [ $? -ne 0 ]; then
    echo
    echo "[错误] 程序启动失败"
    echo "可能的原因:"
    echo "  1. 缺少 spritesheet.png 文件"
    echo "  2. 服务器未正常运行"
    echo "  3. 网络连接问题"
    echo "  4. 权限问题（Linux/macOS可能需要额外权限）"
    echo
    echo "解决方案:"
    echo "  1. 确保 spritesheet.png 文件存在"
    echo "  2. 启动所有必要的服务器"
    echo "  3. 检查防火墙设置"
    echo "  4. 在Linux/macOS上可能需要安装额外的依赖:"
    echo "     Ubuntu/Debian: sudo apt-get install python3-tk python3-dev"
    echo "     CentOS/RHEL: sudo yum install tkinter python3-devel"
    echo "     macOS: 通常随Python自带，如有问题请重新安装Python"
    echo
fi

echo "按回车键退出..."
read