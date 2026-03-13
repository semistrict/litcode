#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
output_path="${1:-assets/litcode-demo.gif}"

case "$output_path" in
  /*)
    echo "output path must be relative to the repo root: $output_path" >&2
    exit 1
    ;;
esac

if ! command -v vhs >/dev/null 2>&1; then
  echo "vhs is required but was not found on PATH" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

demo_bin_dir="$tmp_dir/bin"
demo_project_dir="$tmp_dir/project"
tape_path="$tmp_dir/litcode-demo.tape"

mkdir -p "$demo_bin_dir" "$demo_project_dir/docs" "$demo_project_dir/src" "$(dirname "$repo_root/$output_path")"

(
  cd "$repo_root"
  go build -o "$demo_bin_dir/litcode" .
)

cat > "$demo_project_dir/src/hello.go" <<'EOF'
package main

func hello() string {
	return "hello"
}
EOF

cat > "$demo_project_dir/docs/overview.md" <<'EOF'
# Overview

The example function is documented below.

```go file=src/hello.go lines=1-5
package main

func hello() string {
	return "hello"
}
```
EOF

cat > "$tape_path" <<EOF
Output "$output_path"

Set Shell "bash"
Set FontSize 22
Set Width 1100
Set Height 760
Set Padding 24
Set Framerate 24
Set Theme "Catppuccin Latte"
Set WindowBar Colorful
Set TypingSpeed 55ms

Require "$demo_bin_dir/litcode"
Require "sed"

Hide
Type "cd $demo_project_dir && export PATH=$demo_bin_dir:\$PATH && clear"
Enter
Sleep 300ms
Show

Type "pwd"
Enter
Sleep 800ms

Type "litcode init"
Enter
Sleep 1500ms

Type "sed -n '1,28p' .litcode.json"
Enter
Sleep 2200ms

Type "litcode check"
Enter
Sleep 1800ms
EOF

(
  cd "$repo_root"
  rm -f "$output_path"
  vhs "$tape_path"
)

echo "Wrote $output_path"
