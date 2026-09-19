@echo off
chcp 65001 > nul

:: 设置默认值和处理参数
set ENV=
if not "%~1"=="" set ENV=%~1

echo 启动所有服务，使用配置文件后缀: %ENV%

:: 检查是否有管理员权限
net session >nul 2>&1
if %errorlevel% equ 0 (
    echo 检测到管理员权限，将设置防火墙规则...
    
    :: 添加Go程序防火墙规则
    :: 查找Go可执行文件路径
    where go > %temp%\go_path.txt
    set /p GO_PATH=<%temp%\go_path.txt
    del %temp%\go_path.txt
    
    if exist "%GO_PATH%" (
        :: 为Go程序添加防火墙规则
        netsh advfirewall firewall show rule name="GoRuntime-FunDex" >nul 2>&1
        if %errorlevel% neq 0 (
            echo 正在为Go运行时添加防火墙规则...
            netsh advfirewall firewall add rule name="GoRuntime-FunDex" dir=in action=allow program="%GO_PATH%" enable=yes profile=any description="Go Runtime for Fun Dex Project"
        )
        
        :: 添加服务端口规则
        netsh advfirewall firewall show rule name="FunDexPorts" >nul 2>&1
        if %errorlevel% neq 0 (
            echo 正在为Fun Dex服务端口添加防火墙规则...
            netsh advfirewall firewall add rule name="FunDexPorts" dir=in action=allow protocol=TCP localport=8080-8086 enable=yes profile=any description="Fun Dex Service Ports"
            netsh advfirewall firewall add rule name="FunDexKafka" dir=in action=allow protocol=TCP localport=9092 enable=yes profile=any description="Fun Dex Kafka Port"
        )
    )
) else (
    echo 未检测到管理员权限，跳过设置防火墙规则。
    echo 启动服务时可能会出现Windows防火墙提示，请手动允许。
)

start cmd /k "%~dp0\start_consumer.cmd %ENV%"
timeout /t 4 > nul
start cmd /k "%~dp0\start_market.cmd %ENV%"
timeout /t 4 > nul
start cmd /k "%~dp0\start_trade.cmd %ENV%"
timeout /t 4 > nul
start cmd /k "%~dp0\start_gateway.cmd %ENV%"
timeout /t 4 > nul
start cmd /k "%~dp0\start_websocket.cmd %ENV%"

echo 所有服务已启动!
pause 