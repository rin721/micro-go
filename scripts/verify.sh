#!/usr/bin/env sh
# 完整入口执行当前地基的 unit/static 门禁。
# -e 在子命令失败时退出，-u 拒绝未定义变量造成的空路径。
set -eu

# 通过物理脚本目录定位 unit 入口，允许用户从任意工作目录调用本文件。
script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# 显式使用 sh 保持脚本在最小 POSIX 环境中可运行。
sh "$script_dir/verify-unit.sh"
