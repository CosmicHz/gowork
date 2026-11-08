#!/bin/sh
# install.sh 的单元测试：只 source `# ---- 主流程 ----` 之前的函数，不执行安装。
set -eu

here=$(CDPATH= cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d) || { echo "FAIL: 无法创建临时目录" >&2; exit 1; }
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT INT TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

marker='# ---- 主流程 ----'
grep -Fq "$marker" "$here/install.sh" || fail "install.sh 缺少 $marker"
awk -v marker="$marker" 'index($0, marker) == 1 { exit } { print }' "$here/install.sh" >"$tmp/lib.sh"
grep -Fq 'while [ "$#" -gt 0 ]; do' "$tmp/lib.sh" && fail "测试提取到了主流程"
. "$tmp/lib.sh"

bin_dir=/test/bin

SHELL=/bin/zsh
[ "$(hook_kind_for_shell)" = zsh ] || fail "zsh 期望类型"
SHELL=/usr/bin/fish
[ "$(hook_kind_for_shell)" = fish ] || fail "fish 期望类型"
SHELL=/bin/bash
[ "$(hook_kind_for_shell)" = sh ] || fail "bash 期望类型"
SHELL=/bin/dash
[ "$(hook_kind_for_shell)" = sh ] || fail "dash 期望类型"

[ "$(path_line_for_kind zsh)" = 'export PATH="/test/bin:$PATH"' ] || fail "zsh PATH 行"
[ "$(path_line_for_kind sh)" = 'export PATH="/test/bin:$PATH"' ] || fail "sh PATH 行"
[ "$(path_line_for_kind fish)" = 'fish_add_path "/test/bin"' ] || fail "fish PATH 行"
[ "$(hook_line_for_kind zsh)" = 'eval "$(gowork init zsh)"' ] || fail "zsh hook 行"
[ "$(hook_line_for_kind sh)" = 'eval "$(gowork init sh)"' ] || fail "sh hook 行"
[ "$(hook_line_for_kind fish)" = 'gowork init fish | source' ] || fail "fish hook 行"

# mkblock FILE KIND [WITH_PATH=1]
mkblock() {
    mk_path=""
    if [ "${3:-1}" -eq 1 ]; then
        mk_path=$(path_line_for_kind "$2")
    fi
    mk_hook=$(hook_line_for_kind "$2")
    {
        printf 'outside gowork init sh\n'
        printf '%s\n' "$marker_begin"
        if [ -n "$mk_path" ]; then printf '%s\n' "$mk_path"; fi
        printf '%s\n' "$mk_hook"
        printf '%s\n' "$marker_end"
    } >"$1"
}

mkblock "$tmp/kind-zsh" zsh
[ "$(block_hook_kind "$tmp/kind-zsh")" = zsh ] || fail "检测 zsh"
mkblock "$tmp/kind-fish" fish
[ "$(block_hook_kind "$tmp/kind-fish")" = fish ] || fail "检测 fish"
mkblock "$tmp/kind-sh" sh
[ "$(block_hook_kind "$tmp/kind-sh")" = sh ] || fail "检测 sh"

printf 'eval "$(gowork init sh)"\n' >"$tmp/outside"
[ -z "$(block_hook_kind "$tmp/outside")" ] || fail "配置命令外不该命中"

{
    printf 'export PATH="/x:$PATH"\n'
    printf '%s\n' "$marker_begin"
    printf 'export PATH="/x:$PATH"\n'
    printf '%s\n' "$marker_end"
} >"$tmp/no-hook"
[ -z "$(block_hook_kind "$tmp/no-hook")" ] || fail "没有 hook 应输出空"

{
    printf '%s\r\n' "$marker_begin"
    printf '%s\r\n' 'eval "$(gowork init sh)"'
    printf '%s\r\n' "$marker_end"
} >"$tmp/crlf"
[ "$(block_hook_kind "$tmp/crlf")" = sh ] || fail "CRLF 块检测"
[ "$(block_count "$tmp/kind-sh")" = 1 ] || fail "block_count 单块"
[ "$(block_count "$tmp/outside")" = 0 ] || fail "block_count 无块"
block_has_path "$tmp/kind-sh" || fail "应有 PATH 行"
block_has_hook "$tmp/kind-sh" || fail "应有 hook 行"
block_has_path "$tmp/no-hook" || fail "no-hook 应有 PATH 行"
block_has_hook "$tmp/no-hook" && fail "no-hook 不该有 hook 行"

# 替换：sh 块 → zsh
mkblock "$tmp/rewrite" sh
rewrite_last_block "$tmp/rewrite" zsh || fail "sh → zsh 替换失败"
[ "$(block_hook_kind "$tmp/rewrite")" = zsh ] || fail "替换后 hook 类型"
grep -Fq 'export PATH="/test/bin:$PATH"' "$tmp/rewrite" || fail "替换后 PATH 行"
grep -Fq 'eval "$(gowork init zsh)"' "$tmp/rewrite" || fail "替换后 zsh hook"
grep -Fq 'outside gowork init sh' "$tmp/rewrite" || fail "配置命令外被改动"

# 替换：sh 块 → fish（PATH 行也要换）
mkblock "$tmp/to-fish" sh
rewrite_last_block "$tmp/to-fish" fish || fail "sh → fish 替换失败"
[ "$(block_hook_kind "$tmp/to-fish")" = fish ] || fail "fish hook 类型"
grep -Fq 'fish_add_path "/test/bin"' "$tmp/to-fish" || fail "fish PATH 行"
grep -Fq 'gowork init fish | source' "$tmp/to-fish" || fail "fish hook 行"
grep -Fq 'export PATH=' "$tmp/to-fish" && fail "旧 PATH 行残留"

# 没有 PATH 行的块不应被凭空添加
mkblock "$tmp/only-hook" sh 0
rewrite_last_block "$tmp/only-hook" zsh || fail "only-hook 替换失败"
grep -Fq 'export PATH=' "$tmp/only-hook" && fail "不该新增 PATH 行"

# CRLF 块也能替换
{
    printf '%s\r\n' "$marker_begin"
    printf '%s\r\n' 'eval "$(gowork init sh)"'
    printf '%s\r\n' "$marker_end"
} >"$tmp/crlf-rewrite"
rewrite_last_block "$tmp/crlf-rewrite" zsh || fail "CRLF 替换失败"
[ "$(block_hook_kind "$tmp/crlf-rewrite")" = zsh ] || fail "CRLF 替换后 hook"

# 多个配置命令：默认只改最后一个
mkblock "$tmp/dup-a" sh
mkblock "$tmp/dup-b" fish
cat "$tmp/dup-a" "$tmp/dup-b" >"$tmp/dup"
[ "$(block_count "$tmp/dup")" = 2 ] || fail "重复配置命令计数"
rewrite_last_block "$tmp/dup" zsh || fail "只改最后一个失败"
[ "$(block_count "$tmp/dup")" = 2 ] || fail "只改最后一个不该去重"
[ "$(block_hook_kind "$tmp/dup")" = zsh ] || fail "最后一个应为 zsh"
[ "$(grep -Fc 'eval "$(gowork init sh)"' "$tmp/dup")" = 1 ] || fail "前一个应保持 sh"
[ "$(grep -Fc 'eval "$(gowork init zsh)"' "$tmp/dup")" = 1 ] || fail "最后一个应为 zsh hook"
[ "$(grep -Fc 'outside gowork init sh' "$tmp/dup")" = 2 ] || fail "命令外内容被改动"

# 去重只删前面的，最后一个内容不动
cat "$tmp/dup-a" "$tmp/dup-b" >"$tmp/dedupe"
dedupe_blocks "$tmp/dedupe" || fail "去重失败"
[ "$(block_count "$tmp/dedupe")" = 1 ] || fail "去重后数量"
[ "$(block_hook_kind "$tmp/dedupe")" = fish ] || fail "去重不该改最后一个的 hook"
grep -Fq 'fish_add_path "/test/bin"' "$tmp/dedupe" || fail "去重后 PATH"
grep -Fq 'gowork init fish | source' "$tmp/dedupe" || fail "去重后 fish hook"
grep -Fq 'eval "$(gowork init zsh)"' "$tmp/dedupe" && fail "去重不该改成 zsh"
[ "$(grep -Fc 'outside gowork init sh' "$tmp/dedupe")" = 2 ] || fail "去重丢了命令外内容"

# 拆开的配置命令（一个只有 PATH、一个只有 hook）：只改最后一个
{
    printf '%s\n' "$marker_begin"
    printf '%s\n' "$(path_line_for_kind sh)"
    printf '%s\n' "$marker_end"
    printf '%s\n' "$marker_begin"
    printf '%s\n' "$(hook_line_for_kind sh)"
    printf '%s\n' "$marker_end"
} >"$tmp/split"
rewrite_last_block "$tmp/split" zsh || fail "拆开的配置命令更新失败"
[ "$(block_count "$tmp/split")" = 2 ] || fail "拆开的配置命令不该去重"
[ "$(block_hook_kind "$tmp/split")" = zsh ] || fail "拆开的配置命令更新后 hook"
grep -Fq 'export PATH="/test/bin:$PATH"' "$tmp/split" || fail "前面的 PATH 行被改动"

# 最后一个没有 hook：只给它补上，前面的配置命令不动
{
    printf '%s\n' "$marker_begin"
    printf '%s\n' "$(hook_line_for_kind sh)"
    printf '%s\n' "$marker_end"
    printf '%s\n' "$marker_begin"
    printf '%s\n' "$(path_line_for_kind sh)"
    printf '%s\n' "$marker_end"
} >"$tmp/last-no-hook"
rewrite_last_block "$tmp/last-no-hook" zsh || fail "最后一个缺 hook 时替换失败"
[ "$(block_count "$tmp/last-no-hook")" = 2 ] || fail "补 hook 时不该去重"
[ "$(block_hook_kind "$tmp/last-no-hook")" = zsh ] || fail "最后一个应补上 zsh hook"
[ "$(grep -Fc 'eval "$(gowork init sh)"' "$tmp/last-no-hook")" = 1 ] || fail "前面的配置命令被改动"

# setup_env 端到端：只在没有终端时跑，避免阻塞。
if ( : </dev/tty ) 2>/dev/null; then
    echo "SKIP: 有可用终端，跳过 setup_env 非交互用例"
else
    confirm_action "test" || fail "无终端应默认同意"
    confirm_action_no "test" && fail "无终端去重应默认拒绝"

    home="$tmp/home"
    mkdir -p "$home"

    mkblock "$home/.zshrc" sh
    HOME="$home" ZDOTDIR="$home" SHELL=/bin/zsh setup_env >/dev/null
    [ "$(block_hook_kind "$home/.zshrc")" = zsh ] || fail "setup_env zsh 未升级"

    mkblock "$tmp/multi-a" sh
    mkblock "$tmp/multi-b" fish
    cat "$tmp/multi-a" "$tmp/multi-b" >"$home/.zshrc"
    HOME="$home" ZDOTDIR="$home" SHELL=/bin/zsh setup_env >/dev/null
    [ "$(block_count "$home/.zshrc")" = 2 ] || fail "setup_env 默认不该去重"
    [ "$(block_hook_kind "$home/.zshrc")" = zsh ] || fail "setup_env 应只改最后一个"
    [ "$(grep -Fc 'eval "$(gowork init sh)"' "$home/.zshrc")" = 1 ] || fail "setup_env 改了前面的配置命令"

    mkblock "$home/.zshrc" zsh
    before=$(cat "$home/.zshrc")
    HOME="$home" ZDOTDIR="$home" SHELL=/bin/zsh setup_env >/dev/null
    [ "$(cat "$home/.zshrc")" = "$before" ] || fail "匹配时不该改动"

    fish_cfg="$home/.config/fish/config.fish"
    mkdir -p "$home/.config/fish"
    mkblock "$fish_cfg" sh
    HOME="$home" XDG_CONFIG_HOME="$home/.config" SHELL=/usr/bin/fish setup_env >/dev/null
    [ "$(block_hook_kind "$fish_cfg")" = fish ] || fail "setup_env fish 未升级"
    grep -Fq 'fish_add_path "/test/bin"' "$fish_cfg" || fail "setup_env fish PATH"

    case "$(uname -s)" in
    Darwin) bashrc="$home/.bash_profile" ;;
    *) bashrc="$home/.bashrc" ;;
    esac
    mkblock "$bashrc" sh
    before=$(cat "$bashrc")
    HOME="$home" SHELL=/bin/bash setup_env >/dev/null
    [ "$(cat "$bashrc")" = "$before" ] || fail "bash 匹配时不该改动"
fi

echo "PASS"
