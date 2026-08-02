#!/usr/bin/env sh
# 在复制后的目标目录中替换模板身份；任何校验或命令失败都立即停止。
set -eu

if [ "$#" -lt 2 ] || [ "$#" -gt 4 ]; then
  echo "usage: $0 <module-path> <application-name> [target-directory] [--skip-verify]" >&2
  exit 2
fi

# 位置参数依次表示目标身份、目标目录和可选验证开关。
module_path="$1"
application_name="$2"
target_directory="${3:-.}"
skip_verify="${4:-}"
# Go Module path 至少包含一个斜杠，避免明显无效身份进入批量替换。
case "$module_path" in
  */*) ;;
  *) echo "module path must contain at least one slash" >&2; exit 2 ;;
esac
# 应用名限制为小写字母开头及小写字母、数字、连字符。
case "$application_name" in
  ""|*[!a-z0-9-]*|[!a-z]*) echo "application name must start with a lowercase letter and contain lowercase letters, digits, or hyphens" >&2; exit 2 ;;
esac
# 第四参数只接受唯一显式开关，拼写错误不能被当作默认验证。
if [ "$skip_verify" != "" ] && [ "$skip_verify" != "--skip-verify" ]; then
  echo "unknown fourth argument: $skip_verify" >&2
  exit 2
fi

# 在子 shell 中解析物理绝对路径；cd 失败由 set -e 立即终止。
target="$(cd "$target_directory" && pwd -P)"
# 根目录或缺少 go.mod/AGENTS.md 的目录都不是安全项目副本。
if [ "$target" = "/" ] || [ ! -f "$target/go.mod" ] || [ ! -f "$target/AGENTS.md" ]; then
  echo "target is not a micro-go project copy: $target" >&2
  exit 1
fi

# 当前 Module 必须是模板值或与目标值一致，保证初始化可幂等重跑但不覆盖第三方项目。
template_module='github.com/rin721/micro-go'
current_module="$(sed -n 's/^module[[:space:]]\{1,\}//p' "$target/go.mod")"
if [ "$current_module" != "$template_module" ] && [ "$current_module" != "$module_path" ]; then
  echo "refusing to replace unexpected module path: $current_module" >&2
  exit 1
fi

# 转义 sed 替换文本中的 & 和分隔符，避免 Module path 改写替换表达式。
escaped_module="$(printf '%s' "$module_path" | sed 's/[&|]/\\&/g')"
# 只按目标目录内部的目录名 prune；不能用 */tmp/* 匹配绝对路径，否则目标本身位于
# /tmp 或仓库 tmp 下时会把整个项目静默排除，这正是 CI 和本地验收最常见的位置。
find "$target" \( -type d \( -name '.git' -o -name 'tmp' \) -prune \) -o \
  \( -type f \( -name '*.go' -o -name '*.md' -o -name '*.yaml' -o -name '*.yml' -o -name 'go.mod' -o -name '*.ps1' -o -name '*.sh' \) \
  ! -path "$target/scripts/init-project.ps1" ! -path "$target/scripts/init-project.sh" -print \) | while IFS= read -r file; do
    # 每个文件使用独立临时文件完成转换，原文件只在内容变化后覆盖。
    temporary="$(mktemp)"
    sed "s|$template_module|$escaped_module|g" "$file" > "$temporary"
    if [ "${file##*.}" = "md" ]; then
      # Markdown 额外替换面向用户的模板显示名。
      sed "s|micro-go|$application_name|g" "$temporary" > "$temporary.name"
      mv "$temporary.name" "$temporary"
    fi
    if [ "$file" = "$target/internal/bootstrap/bootstrap.go" ]; then
      # Bootstrap 只替换准确默认应用常量。
      sed "s|defaultApplicationName = \"micro-go\"|defaultApplicationName = \"$application_name\"|" "$temporary" > "$temporary.name"
      mv "$temporary.name" "$temporary"
    fi
    if [ "$file" = "$target/config/app.yaml" ]; then
      # YAML 只替换 application.name 的模板值。
      sed "s|^\([[:space:]]*name:[[:space:]]*\)micro-go[[:space:]]*$|\1$application_name|" "$temporary" > "$temporary.name"
      mv "$temporary.name" "$temporary"
    fi
    # cmp 避免未变化文件产生时间戳和 Diff 噪声。
    if ! cmp -s "$file" "$temporary"; then
      cp "$temporary" "$file"
    fi
    # 临时文件不属于项目资产，每轮处理完成立即清理。
    rm -f "$temporary"
  done

# 全量残留扫描保证没有 Go import 继续引用模板 Module。
if grep -R -l --include='*.go' "$template_module" "$target" --exclude-dir=.git --exclude-dir=tmp >/dev/null 2>&1; then
  echo "template module path remains in Go source" >&2
  exit 1
fi

# 默认在子 shell 的目标目录执行依赖、构建、测试和 Bootstrap 启停验证。
if [ "$skip_verify" != "--skip-verify" ]; then
  (
    cd "$target"
    go mod tidy
    go build ./...
    go test -count=1 ./...
    go test -count=1 ./internal/bootstrap -run '^TestRunBuildsAndStopsApplication$'
  )
fi

# 只有替换、残留扫描和所有启用验证成功后输出完成信息。
echo "Initialized $application_name with module $module_path in $target"
