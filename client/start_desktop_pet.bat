@echo off
title 桌面宠物游客登录启动器
echo.
echo ==========================================
echo           桌面宠物游客登录版本
echo ==========================================
echo.
echo 功能特性:
echo   * 游客快速登录，无需手机验证
echo   * 多玩家在线互动
echo   * 实时聊天功能  
echo   * 动作同步显示
echo.
echo 正在检查环境...

REM 检查Python是否安装
python --version >nul 2>&1
if errorlevel 1 (
    echo [错误] 未找到Python，请先安装Python 3.7+
    pause
    exit /b 1
)

echo [✓] Python环境正常

REM 检查必要模块
echo 正在检查必要模块...
python -c "import websockets, requests, PIL, pynput" >nul 2>&1
if errorlevel 1 (
    echo [!] 检测到缺少必要模块，正在安装...
    pip install websockets requests pillow pynput
    if errorlevel 1 (
        echo [错误] 模块安装失败，请手动安装:
        echo pip install websockets requests pillow pynput
        pause
        exit /b 1
    )
)

echo [✓] 依赖模块正常

REM 检查服务器状态
echo 正在检查服务器状态...
curl -s -m 3 http://localhost:8081/health >nul 2>&1
if errorlevel 1 (
    echo [!] 登录服务器未启动 (端口8081)
    echo 请先启动服务器，或者选择继续（将以离线模式运行）
    echo.
    set /p choice="是否继续启动客户端? (y/n): "
    if /i not "%choice%"=="y" (
        echo 程序退出
        pause
        exit /b 1
    )
) else (
    echo [✓] 登录服务器正常
)

echo.
echo 启动桌面宠物客户端...
echo.

REM 切换到客户端目录
cd /d "%~dp0"

REM 启动客户端
python desktop_pet_guest.py

REM 如果出错，显示错误信息
if errorlevel 1 (
    echo.
    echo [错误] 程序启动失败
    echo 可能的原因:
    echo   1. 缺少 spritesheet.png 文件
    echo   2. 服务器未正常运行
    echo   3. 网络连接问题
    echo.
    echo 解决方案:
    echo   1. 确保 spritesheet.png 文件存在
    echo   2. 启动所有必要的服务器
    echo   3. 检查防火墙设置
    echo.
)

pause