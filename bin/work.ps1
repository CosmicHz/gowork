<#
.SYNOPSIS
    work —— 项目跳板 + 环境仪表盘。

.DESCRIPTION
    本脚本是 gowork.exe 的 shell 入口，只负责一件事：在当前会话里 Set-Location。
    子进程改不了父 shell 的工作目录，所以 gowork.exe 把目标目录经临时文件回传。

    工作区根目录按优先级取：-Workspace > 环境变量 GOWORK_WORKSPACE > 配置文件
    %LOCALAPPDATA%\gowork\config.json；三者都没有会报错并要求显式配置。

.EXAMPLE
    work                回到工作区根目录
    work my-app         按唯一前缀跳转
    work -1             跳转到最近修改的第 1 个项目
    work my-app -IndentSize 4
    work -Workspace D:\code
                        临时指定工作区
    work https://github.com/octocat/Hello-World.git
                        克隆到工作区并进入（已存在则直接进入）
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Project,

    [int]$IndentSize = 3,

    [string]$Workspace
)

$exe = Join-Path $PSScriptRoot 'gowork.exe'
if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) {
    Write-Error "未找到 gowork.exe，请先构建：go build -o bin\gowork.exe ."
    $global:LASTEXITCODE = 127
    return
}

$cwdFile = [System.IO.Path]::GetTempFileName()
try {
    $exeArgs = @("--indent=$IndentSize", "--cwd-file=$cwdFile")
    if ($Project) {
        $exeArgs += "--project=$Project"
    }
    if ($Workspace) {
        $exeArgs += "--workspace=$Workspace"
    }

    # Start-Process 只是拼接命令行，所以每个参数自己加好引号。
    $argumentLine = ($exeArgs | ForEach-Object { '"' + $_.Replace('"', '\"') + '"' }) -join ' '

    # 用 Start-Process 让 gowork.exe 直接接管当前控制台：子进程自己写屏，
    # 绕开 PowerShell 的输出管线，ANSI 颜色才不会被 $PSStyle.OutputRendering 抹掉。
    $proc = Start-Process -FilePath $exe -ArgumentList $argumentLine -NoNewWindow -Wait -PassThru

    $global:LASTEXITCODE = $proc.ExitCode

    if ($proc.ExitCode -eq 0) {
        $target = Get-Content -LiteralPath $cwdFile -Raw -ErrorAction SilentlyContinue
        if ($target) {
            Set-Location -LiteralPath $target.Trim()
        }
    }
}
finally {
    Remove-Item -LiteralPath $cwdFile -Force -ErrorAction SilentlyContinue
}
