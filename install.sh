#!/bin/sh
# gowork 安装脚本：下载最新正式 release 的二进制，装进用户目录，
# 并把 PATH 与 work 集成写进当前 shell 的配置（可用 --no-modify 跳过）。
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/CosmicHz/gowork/main/install.sh | sh
#   ./install.sh --bin-dir ~/.local/bin
#   ./install.sh --no-modify
#   ./install.sh --from-source
#
# 默认只支持 linux / darwin 的 amd64 / arm64；其他平台用 --from-source。
# 重复执行会覆盖升级，已配置过的 PATH / shell 集成不会重复追加。

set -eu

repo="CosmicHz/gowork"
repo_url="https://github.com/$repo.git"
latest_api="https://api.github.com/repos/$repo/releases/latest"
releases_api="https://api.github.com/repos/$repo/releases"
bin_dir="${GOWORK_BIN_DIR:-$HOME/.local/bin}"
from_source=0
no_modify=0
pre_release=0
tmp_dir=""

marker_begin="# >>> gowork installer >>>"
marker_end="# <<< gowork installer <<<"

usage() {
    cat <<'EOF'
用法: install.sh [选项]

选项:
  --bin-dir <路径>  安装目录，默认 ~/.local/bin，也可用 GOWORK_BIN_DIR 指定
  --from-source     从源码构建（需要 Go；在仓库内用当前代码，否则浅克隆）
  --pre-release     安装最新预发布版本（默认只用正式版）
  --no-modify       只装二进制，不改 PATH，也不写 shell 配置
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

has() {
    command -v "$1" >/dev/null 2>&1
}

cleanup() {
    if [ -n "$tmp_dir" ]; then
        rm -rf "$tmp_dir"
    fi
}

fetch() {
    if has curl; then
        curl -fsSL "$1"
    elif has wget; then
        wget -qO- "$1"
    else
        die '需要 curl 或 wget 下载 release'
    fi
}

download() {
    if has curl; then
        curl -fsSL -o "$2" "$1"
    else
        wget -qO "$2" "$1"
    fi
}

is_fish() {
    [ "${SHELL##*/}" = fish ]
}

# 当前 shell 的配置文件；识别不了就返回 1。
detect_rc() {
    case "${SHELL##*/}" in
    zsh) printf '%s\n' "${ZDOTDIR:-$HOME}/.zshrc" ;;
    bash)
        if [ "$(uname -s 2>/dev/null || true)" = Darwin ]; then
            printf '%s\n' "$HOME/.bash_profile"
        else
            printf '%s\n' "$HOME/.bashrc"
        fi
        ;;
    fish) printf '%s\n' "${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" ;;
    sh | dash) printf '%s\n' "$HOME/.profile" ;;
    *) return 1 ;;
    esac
}

dir_in_path() {
    case ":$PATH:" in
    *":$bin_dir:"*) return 0 ;;
    *) return 1 ;;
    esac
}

print_manual() {
    printf '手动步骤（写入你的 shell 配置）：\n'
    if is_fish; then
        printf '  fish_add_path "%s"\n' "$bin_dir"
        printf '  gowork init fish | source\n'
    else
        printf '  export PATH="%s:$PATH"\n' "$bin_dir"
        printf '  eval "$(gowork init sh)"\n'
    fi
    printf '然后重开终端。\n'
}

# setup_env 把 PATH 与 work 集成写进当前 shell 的 rc；失败只警告，不影响安装。
setup_env() {
    rc=$(detect_rc) || rc=""
    if [ -z "$rc" ]; then
        printf '\n未识别当前 shell，未修改 PATH 或 shell 配置。\n'
        print_manual
        return
    fi

    if [ -f "$rc" ] && grep -Fq "$marker_begin" "$rc"; then
        printf '\n%s 里已有 gowork 安装块，跳过 PATH 与集成配置。\n' "$rc"
        return
    fi

    need_path=0
    if ! dir_in_path; then
        if [ ! -f "$rc" ] || ! grep -Fq "$bin_dir" "$rc"; then
            need_path=1
        fi
    fi

    need_hook=0
    if [ ! -f "$rc" ] || ! grep -Fq 'gowork init' "$rc"; then
        need_hook=1
    fi

    if [ "$need_path" -eq 0 ] && [ "$need_hook" -eq 0 ]; then
        printf '\nPATH 与 work 集成已配置，无需修改。\n'
        return
    fi

    if is_fish; then
        path_line="fish_add_path \"$bin_dir\""
        hook_line='gowork init fish | source'
    else
        path_line="export PATH=\"$bin_dir:\$PATH\""
        hook_line='eval "$(gowork init sh)"'
    fi

    rc_dir=$(dirname "$rc")
    if ! mkdir -p "$rc_dir" 2>/dev/null; then
        printf '\n无法创建 %s，未修改 PATH 或 shell 配置。\n' "$rc_dir"
        print_manual
        return
    fi

    # 逐段拼接并保留换行；命令替换会吞掉行尾换行，所以这里用字面换行。
    block="
$marker_begin"
    if [ "$need_path" -eq 1 ]; then
        block="$block
$path_line"
    fi
    if [ "$need_hook" -eq 1 ]; then
        block="$block
$hook_line"
    fi
    block="$block
$marker_end"

    # 用单条 printf 追加：Bash 对 `{ ... } >>缺失目录` 会错误地返回 0，
    # 这里需要拿到真实的写入结果。
    if ! printf '%s\n' "$block" >>"$rc" 2>/dev/null; then
        printf '\n无法写入 %s，未修改 PATH 或 shell 配置。\n' "$rc"
        print_manual
        return
    fi

    printf '\n'
    if [ "$need_path" -eq 1 ]; then
        printf '已把 %s 加入 PATH：%s\n' "$bin_dir" "$rc"
    fi
    if [ "$need_hook" -eq 1 ]; then
        printf '已把 work 集成写入：%s\n' "$rc"
    fi
    printf '重开终端后生效。\n'
}

install_from_source() {
    # go.mod 在 Windows 检出时可能是 CRLF，用前缀匹配，别让 \r 破坏判断。
    if [ -f go.mod ] && grep -q '^module gowork' go.mod; then
        src_dir=$(pwd)
        printf '使用当前目录的源码：%s\n' "$src_dir"
    else
        need git
        tmp_dir=$(mktemp -d) || die '无法创建临时目录'
        printf '克隆 %s\n' "$repo_url"
        git clone --depth 1 "$repo_url" "$tmp_dir/gowork" || die "克隆失败：$repo_url"
        src_dir="$tmp_dir/gowork"
    fi

    need go
    mkdir -p "$bin_dir" || die "无法创建安装目录：$bin_dir"
    printf '构建 %s/gowork\n' "$bin_dir"
    (cd "$src_dir" && go build -trimpath -o "$bin_dir/gowork" ./cmd/gowork) || die '构建失败'
}

install_from_release() {
    os_name=$(uname -s 2>/dev/null || true)
    case "$os_name" in
    Linux) os_name=linux ;;
    Darwin) os_name=darwin ;;
    MINGW* | MSYS* | CYGWIN*) die 'Windows 请改用 install.ps1' ;;
    *) die "不支持的系统：${os_name:-unknown}（请用 --from-source 自行构建）" ;;
    esac

    machine=$(uname -m 2>/dev/null || true)
    case "$machine" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) die "不支持的架构：${machine:-unknown}（请用 --from-source 自行构建）" ;;
    esac

    # 默认用 /releases/latest（只含正式版）；--pre-release 时改用列表第一条，
    # 未认证的列表接口不返回 draft，但包含预发布。
    if [ "$pre_release" -eq 1 ]; then
        payload=$(fetch "$releases_api") || die '没有找到可安装的 release；请检查网络或 GitHub API 是否可用'
    else
        payload=$(fetch "$latest_api") || die '没有找到正式发布的 release；需要预发布版本时加 --pre-release'
    fi
    tag=$(printf '%s\n' "$payload" | grep -m1 '"tag_name"' | cut -d'"' -f4)
    [ -n "$tag" ] || die '没有找到可安装的 release；请检查网络或 GitHub API 是否可用'
    if [ "$pre_release" -eq 1 ]; then
        printf '使用预发布版本 %s\n' "$tag"
    fi

    version=${tag#v}
    asset="gowork_${version}_${os_name}_${arch}.tar.gz"
    base="https://github.com/$repo/releases/download/$tag"

    tmp_dir=$(mktemp -d) || die '无法创建临时目录'
    archive="$tmp_dir/$asset"

    printf '下载 %s\n' "$base/$asset"
    download "$base/$asset" "$archive" || die "下载失败：$base/$asset"

    printf '校验 sha256\n'
    download "$base/checksums.txt" "$tmp_dir/checksums.txt" || die '下载 checksums.txt 失败'
    want=$(grep -F "$asset" "$tmp_dir/checksums.txt" | awk '{print $1}' | head -n1)
    [ -n "$want" ] || die "checksums.txt 中没有 $asset"

    if has sha256sum; then
        got=$(sha256sum "$archive" | awk '{print $1}')
    elif has shasum; then
        got=$(shasum -a 256 "$archive" | awk '{print $1}')
    else
        die '需要 sha256sum 或 shasum 校验下载'
    fi
    [ "$got" = "$want" ] || die "sha256 校验失败：$asset"

    mkdir -p "$tmp_dir/extract" || die '无法创建解压目录'
    tar -xzf "$archive" -C "$tmp_dir/extract" || die '解压失败'
    src="$tmp_dir/extract/${asset%.tar.gz}/gowork"
    [ -f "$src" ] || die '压缩包结构异常：没有找到 gowork'

    mkdir -p "$bin_dir" || die "无法创建安装目录：$bin_dir"
    printf '安装 %s/gowork\n' "$bin_dir"
    # 先写到同目录的临时文件再改名，避免中途失败留下半个二进制。
    cp "$src" "$bin_dir/.gowork.tmp.$$" || die '复制二进制失败'
    mv "$bin_dir/.gowork.tmp.$$" "$bin_dir/gowork" || die '替换二进制失败'
    chmod 755 "$bin_dir/gowork"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
    --bin-dir)
        [ "$#" -ge 2 ] || die "--bin-dir 需要一个路径"
        bin_dir="$2"
        shift 2
        ;;
    --bin-dir=*) bin_dir="${1#*=}" && shift ;;
    --from-source) from_source=1 && shift ;;
    --pre-release) pre_release=1 && shift ;;
    --no-modify) no_modify=1 && shift ;;
    -h | --help) usage && exit 0 ;;
    *)
        printf 'install: 未知选项 %s\n\n' "$1" >&2
        usage >&2
        exit 2
        ;;
    esac
done

trap cleanup EXIT INT TERM

if [ "$from_source" -eq 1 ]; then
    install_from_source
else
    install_from_release
fi

printf '\n已安装：%s/gowork\n' "$bin_dir"

if [ "$no_modify" -eq 1 ]; then
    printf '\n未修改 PATH 或 shell 配置（--no-modify）。\n'
    print_manual
else
    setup_env
fi

printf '\n重开终端后运行 work；工作区用 GOWORK_WORKSPACE 或配置文件指定。\n'
