@echo off
:: 设置编码为 UTF-8 防止中文乱码
chcp 65001 > nul

echo 正在请求管理员权限并启动文档生成脚本...
:: 使用 PowerShell 运行同目录下的 generate_docs.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0generate_docs.ps1"

echo.
echo 任务已完成！按任意键退出...
pause