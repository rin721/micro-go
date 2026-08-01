#!/usr/bin/env sh
# 在复制后的目标目录中替换模板身份；任何校验或命令失败都立即停止。
set -eu

if [ "$#" -lt 2 ] || [ "$#" -gt 4 ]; then
  echo "usage: $0 <module-path> <application-name> [target-directory] [--skip-verify]" >&2
  exit 2
fi

module_path="$1"
application_name="$2"
target_directory="${3:-.}"
skip_verify="${4:-}"
case "$module_path" in
  */*) ;;
  *) echo "module path must contain at least one slash" >&2; exit 2 ;;
esac
case "$application_name" in
  ""|*[!a-z0-9-]*|[!a-z]*) echo "application name must start with a lowercase letter and contain lowercase letters, digits, or hyphens" >&2; exit 2 ;;
esac
if [ "$skip_verify" != "" ] && [ "$skip_verify" != "--skip-verify" ]; then
  echo "unknown fourth argument: $skip_verify" >&2
  exit 2
fi

target="$(cd "$target_directory" && pwd -P)"
if [ "$target" = "/" ] || [ ! -f "$target/go.mod" ] || [ ! -f "$target/AGENTS.md" ]; then
  echo "target is not a micro-go project copy: $target" >&2
  exit 1
fi

template_module='github.com/rin721/micro-go'
current_module="$(sed -n 's/^module[[:space:]]\{1,\}//p' "$target/go.mod")"
if [ "$current_module" != "$template_module" ] && [ "$current_module" != "$module_path" ]; then
  echo "refusing to replace unexpected module path: $current_module" >&2
  exit 1
fi

escaped_module="$(printf '%s' "$module_path" | sed 's/[&|]/\\&/g')"
# 只按目标目录内部的目录名 prune；不能用 */tmp/* 匹配绝对路径，否则目标本身位于
# /tmp 或仓库 tmp 下时会把整个项目静默排除，这正是 CI 和本地验收最常见的位置。
find "$target" \( -type d \( -name '.git' -o -name 'tmp' \) -prune \) -o \
  \( -type f \( -name '*.go' -o -name '*.md' -o -name '*.yaml' -o -name '*.yml' -o -name 'go.mod' -o -name '*.ps1' -o -name '*.sh' \) \
  ! -path "$target/scripts/init-project.ps1" ! -path "$target/scripts/init-project.sh" -print \) | while IFS= read -r file; do
    temporary="$(mktemp)"
    sed "s|$template_module|$escaped_module|g" "$file" > "$temporary"
    if [ "${file##*.}" = "md" ]; then
      sed "s|micro-go|$application_name|g" "$temporary" > "$temporary.name"
      mv "$temporary.name" "$temporary"
    fi
    if [ "$file" = "$target/internal/bootstrap/bootstrap.go" ]; then
      sed "s|defaultApplicationName = \"micro-go\"|defaultApplicationName = \"$application_name\"|" "$temporary" > "$temporary.name"
      mv "$temporary.name" "$temporary"
    fi
    if [ "$file" = "$target/config/app.yaml" ]; then
      sed "s|^\([[:space:]]*name:[[:space:]]*\)micro-go[[:space:]]*$|\1$application_name|" "$temporary" > "$temporary.name"
      mv "$temporary.name" "$temporary"
    fi
    if ! cmp -s "$file" "$temporary"; then
      cp "$temporary" "$file"
    fi
    rm -f "$temporary"
  done

if grep -R -l --include='*.go' "$template_module" "$target" --exclude-dir=.git --exclude-dir=tmp >/dev/null 2>&1; then
  echo "template module path remains in Go source" >&2
  exit 1
fi

if [ "$skip_verify" != "--skip-verify" ]; then
  (
    cd "$target"
    go mod tidy
    go build ./...
    go test -count=1 ./...
    go test -count=1 ./internal/bootstrap -run '^TestRunBuildsAndStopsApplication$'
  )
fi

echo "Initialized $application_name with module $module_path in $target"
