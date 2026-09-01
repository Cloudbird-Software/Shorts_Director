#!/bin/sh
# C2 调用包装：operator <op> --contract-version 1（LocalRunner 直接 exec 本脚本）
exec python3 "$(dirname "$0")/main.py" "$@"
