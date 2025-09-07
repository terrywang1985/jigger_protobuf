set WORKSPACE=code\luban_examples
set LUBAN_DLL=%WORKSPACE%\Tools\Luban\Luban.dll
set CONF_ROOT=..\config

dotnet %LUBAN_DLL% ^
    -t all ^
    -d json ^
	-c go-json ^
    --conf %CONF_ROOT%\luban.conf ^
    -x outputDataDir=..\server\cfg ^
	-x outputCodeDir=..\server\src\cfg_parse ^
	-x lubanGoModule=./src
pause