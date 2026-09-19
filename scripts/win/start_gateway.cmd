@echo off
chcp 65001 > nul

:: 设置默认值和处理参数
set ENV=
if not "%~1"=="" set ENV=-%~1

echo 使用配置文件后缀: %ENV%

cd /d %~dp0\..\..\gateway

echo 正在启动Gateway服务...
go run gateway.go -f etc/gateway%ENV%.yaml
pause 