# work —— gowork 的 fish 入口。
#
# 用法：在 ~/.config/fish/config.fish 里加一行，然后重开终端：
#     source /path/to/gowork/bin/work.fish
#
# 和 work.sh 同样的道理：必须是函数才能在当前会话里 cd。
# 可执行文件查找顺序：$GOWORK_BIN > PATH 里的 gowork。

function work --description 'gowork：跳转项目并渲染仪表盘'
    set -l bin gowork
    if set -q GOWORK_BIN
        set bin $GOWORK_BIN
    end

    if not type -q $bin
        echo "work: 未找到 $bin，请先构建并放进 PATH，或用 GOWORK_BIN 指定" >&2
        return 127
    end

    set -l cwd_file (mktemp)
    if test -z "$cwd_file"
        echo 'work: 无法创建临时文件' >&2
        return 1
    end

    command $bin --cwd-file=$cwd_file $argv
    set -l rc $status

    if test $rc -eq 0; and test -s $cwd_file
        cd (cat $cwd_file)
        or set rc $status
    end

    rm -f $cwd_file
    return $rc
end
