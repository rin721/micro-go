#!/usr/bin/env sh
# 完整入口顺序执行快速门禁和真实协议集成门禁。
set -eu

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
sh "$script_dir/verify-unit.sh"
sh "$script_dir/verify-integration.sh"
