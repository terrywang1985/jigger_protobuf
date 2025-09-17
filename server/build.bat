@echo off

echo ================================
echo    Build jigger_protobuf Servers
echo ================================

REM Create bin directory if it doesn't exist
if not exist bin mkdir bin

REM Set environment variables
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64

REM Build each server
echo.
echo [1/3] Building Game Server...
cd src\servers\game
go build -o ..\..\..\bin\game-server.exe .
if %errorlevel% neq 0 (
    echo Error: Game Server build failed
    exit /b 1
)
cd ..\..\..

echo.
echo [2/3] Building Battle Server...
cd src\servers\battle
go build -o ..\..\..\bin\battle-server.exe .
if %errorlevel% neq 0 (
    echo Error: Battle Server build failed
    exit /b 1
)
cd ..\..\..

echo.
echo [3/3] Building Login Server...
cd src\servers\login
go build -o ..\..\..\bin\login-server.exe .\loginserver.go
if %errorlevel% neq 0 (
    echo Error: Login Server build failed
    exit /b 1
)
cd ..\..\..

echo.
echo ================================
echo    All servers built successfully!
echo ================================

echo.
echo Build results:
dir bin\*.exe

echo.
echo Tips:
echo - Executables are located in the bin\ directory
echo - Configuration files are in the cfg\ directory
echo - Make sure to run servers from the server\ directory