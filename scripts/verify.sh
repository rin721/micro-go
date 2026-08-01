#!/usr/bin/env sh
# 完整入口执行当前地基的 unit/static 门禁。
set -eu

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
sh "$script_dir/verify-unit.sh"
