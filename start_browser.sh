#!/bin/bash

GoogleChromePath="/var/lib/linglong/layers/95cf712fbf6179ef4f8f709edcfcb5f88909894f29194fc19602eee2f45a2f6e/files/bin/google/chrome/google-chrome"

CDP_PORT=9224
BinFilePath=""
if command -v google-chrome >/dev/null 2>&1; then
    BinFilePath="google-chrome"
elif [ -n "$GoogleChromePath" ] && [ -f "$GoogleChromePath" ]; then
    BinFilePath="$GoogleChromePath"
else
        echo "找不到google-chrome。google-chrome命令不存在且未配置GoogleChromePath变量。请确保安装了google-chrome。请使google-chrome命令存在或者配置google-chrome的二进制路径到GoogleChromePath变量。"
        exit 1
fi

PowerCommand="${BinFilePath} --remote-debugging-port=${CDP_PORT} --user-data-dir=/tmp/cdp-search-google-skill-profile --no-first-run --no-default-browser-check"

# 不设置这个会报错：
# [1860866:1860866:0517/144305.608364:ERROR:ui/ozone/platform/x11/ozone_platform_x11.cc:256] Missing X server or $DISPLAY
# [1860866:1860866:0517/144305.608400:ERROR:ui/aura/env.cc:246] The platform failed to initialize.  Exiting.
export DISPLAY=":0"

if lsof -i :${CDP_PORT} >/dev/null 2>&1; then
    echo "端口${CDP_PORT}已被占用，无法启动浏览器。"
    exit 1
fi

nohup $PowerCommand > /dev/null 2>&1 &
BROWSER_PID=$!

echo "调试端口${CDP_PORT}。浏览器的PID: $BROWSER_PID"