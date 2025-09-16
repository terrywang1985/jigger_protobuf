@echo off
echo 重新生成protobuf代码...

:: 进入proto目录
cd /d "%~dp0proto"

:: 检查protoc是否安装
protoc --version >nul 2>&1
if errorlevel 1 (
    echo 错误: protoc 未安装或不在PATH中
    echo 请安装protobuf编译器: https://grpc.io/docs/protoc-installation/
    pause
    exit /b 1
)

:: 创建输出目录
if not exist "..\server\src\proto" mkdir "..\server\src\proto"

echo 生成Go代码...
:: 生成Go代码
protoc --go_out=../server/src/proto ^
       --go_opt=paths=source_relative ^
       --go-grpc_out=../server/src/proto ^
       --go-grpc_opt=paths=source_relative ^
       *.proto

if errorlevel 1 (
    echo Go代码生成失败！
    pause
    exit /b 1
)

echo 生成Python代码...
:: 生成Python代码
protoc --python_out=../client ^
       *.proto

if errorlevel 1 (
    echo Python代码生成失败！
    pause
    exit /b 1
)

echo protobuf代码生成完成！

echo.
echo 注意事项：
echo 1. 请确保生成的Go文件的package声明正确
echo 2. 如果有导入问题，请检查go.mod文件  
echo 3. Python客户端可以使用生成的*_pb2.py文件

pause