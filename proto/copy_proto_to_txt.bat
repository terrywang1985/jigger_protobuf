
rem 把当前目录下的所有 .proto 文件复制到当前目录下的 txt 文件夹中, 并重命名为 _proto.txt

@echo off
setlocal enabledelayedexpansion
set "source_dir=." 
set "target_dir=txt"

if not exist "%target_dir%" (
    mkdir "%target_dir%"
)

for %%f in (%source_dir%\*.proto) do (
    set "filename=%%~nf"
    set "new_filename=!filename!_proto.txt"
    copy "%%f" "%target_dir%\!new_filename!"
)
echo All .proto files have been copied and renamed to _proto.txt in the txt folder.
