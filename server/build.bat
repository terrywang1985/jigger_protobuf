@echo off
chcp 65001 > nul

echo === 构建 jigger_protobuf 服务器 ===

REM 创建 bin 目录（如果不存在）
if not exist bin mkdir bin

REM 设置环境变量
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64

REM 构建各个服务器
echo 🔨 构建 Game Server...
cd src\servers\game
go build -o ..\..\..\bin\game-server.exe .
if %errorlevel% neq 0 (
    echo ❌ Game Server 构建失败
    exit /b 1
)
cd ..\..\..

echo 🔨 构建 Battle Server...
cd src\servers\battle
go build -o ..\..\..\bin\battle-server.exe .
if %errorlevel% neq 0 (
    echo ❌ Battle Server 构建失败
    exit /b 1
)
cd ..\..\..

echo 🔨 构建 Login Server...
cd src\servers\login
go build -o ..\..\..\bin\login-server.exe .\loginserver.go
if %errorlevel% neq 0 (
    echo ❌ Login Server 构建失败
    exit /b 1
)
cd ..\..\..

echo ✅ 所有服务器构建完成！

echo.
echo === 构建结果 ===
dir bin\*.exe

echo.
echo 💡 可执行文件位于 bin\ 目录
echo 💡 配置文件位于 cfg\ 目录
echo 💡 运行服务器前请确保在 server\ 目录下执行