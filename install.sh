#!/bin/sh
# gowork 安装脚本：构建二进制放进指定目录，再打印 shell 集成需要的那一行。
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/CosmicHz/gowork/main/install.sh | sh
#   ./install.sh --bin-dir ~/.local/bin
#
# 在仓库根目录执行时直接用当前代码构建，否则浅克隆一份到临时目录。
# 脚本不改 PATH，也不改 shell 配置，只把需要添加的内容打印出来。

set -eu

repo_url="https://github.com/CosmicHz/gowork.git"
bin_dir="${GOWORK_BIN_DIR:-$HOME/.local/bin}"
tmp_dir=""

usage() {
    cat <<'EOF'
用法: install.sh [选项]

选项:
  --bin-dir <路径>  安装目录，默认 ~/.local/bin，也可用 GOWORK_BIN_DIR 指定
  -h, --help        显示这段帮助
EOF
}

die() {
    printf 'install: %s\n' "$1" >&2
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || die "需要 $1，但没有找到"
}

cleanup() {
    [ -n "$tmp_dir" ] && rm -rf "$tmp_dir"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
    --bin-dir)
        [ "$#" -ge 2 ] || die "--bin-dir 需要一个路径"
        bin_dir="$2"
        shift 2
        ;;
    --bin-dir=*) bin_dir="${1#*=}" && shift ;;
    -h | --help) usage && exit 0 ;;
    *)
        printf 'install: 未知选项 %s\n\n' "$1" >&2
        usage >&2
        exit 2
        ;;
    esac
done

trap cleanup EXIT INT TERM

if [ -f go.mod ] && grep -qx 'module gowork' go.mod; then
    src_dir=$(pwd)
    printf '使用当前目录的源码：%s\n' "$src_dir"
else
    need git
    tmp_dir=$(mktemp -d) || die "无法创建临时目录"
    printf '克隆 %s\n' "$repo_url"
    git clone --depth 1 "$repo_url" "$tmp_dir/gowork" || die "克隆失败：$repo_url"
    src_dir="$tmp_dir/gowork"
fi

need go
mkdir -p "$bin_dir" || die "无法创建安装目录：$bin_dir"
printf '构建 %s/gowork\n' "$bin_dir"
(cd "$src_dir" && go build -trimpath -o "$bin_dir/gowork" .) || die "构建失败"

printf '\n已安装：%s/gowork\n' "$bin_dir"

case ":${PATH}:" in
*":${bin_dir}:"*) ;;
*)
    printf '\n注意：%s 不在 PATH 上，先把它加进去，例如在 shell 配置里写：\n' "$bin_dir"
    printf '  export PATH="%s:$PATH"\n' "$bin_dir"
    ;;
esac

shell_name="${SHELL:-}"
case "${shell_name##*/}" in
fish) printf '\n把这一行加到 ~/.config/fish/config.fish 末尾：\n  gowork init fish | source\n' ;;
*) printf '\n把这一行加到 shell 配置里（bash 是 ~/.bashrc，zsh 是 ~/.zshrc）：\n  eval "$(gowork init sh)"\n' ;;
esac

printf '\n重开终端后运行 work；工作区用 GOWORK_WORKSPACE 或配置文件指定。\n'
