#!/bin/sh
# work —— gowork 的 POSIX shell 入口（bash / zsh / dash）。
#
# 本文件是被 source 的，不是执行的；上面那行 shebang 只是给编辑器与静态检查的提示。
#
# 用法：在 ~/.bashrc 或 ~/.zshrc 里加一行，然后重开终端（或在当前会话 source）：
#     . /path/to/gowork/bin/work.sh
#
# 为什么必须是一个函数、而不是可执行脚本：子进程改不了父 shell 的工作目录。
# 所以 gowork 把目标目录写进 --cwd-file 指定的临时文件，这里读出来 cd。
#
# 注意 gowork 是直接调用的（不接管道、不做命令替换），它的 stdout 才是真终端，
# 清屏、窗口标题和 ANSI 颜色才会生效；写成 $(work) 那样捕获输出会把它们全关掉。
#
# 可执行文件查找顺序：$GOWORK_BIN > PATH 里的 gowork。

work() {
    local bin cwd_file rc
    bin=${GOWORK_BIN:-gowork}

    if ! command -v "$bin" >/dev/null 2>&1; then
        printf 'work: 未找到 %s，请先构建并放进 PATH，或用 GOWORK_BIN 指定\n' "$bin" >&2
        return 127
    fi

    cwd_file=$(mktemp) || {
        printf 'work: 无法创建临时文件\n' >&2
        return 1
    }

    command "$bin" --cwd-file="$cwd_file" "$@"
    rc=$?

    if [ "$rc" -eq 0 ] && [ -s "$cwd_file" ]; then
        # 反斜杠让命令名不被别名替换（POSIX 行为），否则用户若 alias 过 cd 就会跑偏；
        \cd -- "$(cat "$cwd_file")" || rc=$?
    fi

    rm -f "$cwd_file"
    return "$rc"
}
