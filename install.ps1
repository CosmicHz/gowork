# gowork 安装脚本：构建 gowork.exe 放进指定目录，再打印 shell 集成需要的那一行。
#
# 用法：
#   irm https://raw.githubusercontent.com/CosmicHz/gowork/main/install.ps1 | iex
#   .\install.ps1 -BinDir D:\bin
#
# 在仓库根目录执行时直接用当前代码构建，否则浅克隆一份到临时目录。
# 脚本不改 PATH，也不改 $PROFILE，只把需要添加的内容打印出来。
[CmdletBinding()]
param(
    [string]$BinDir
)

$ErrorActionPreference = 'Stop'

$repoUrl = 'https://github.com/CosmicHz/gowork.git'
if (-not $BinDir) {
    $BinDir = Join-Path $env:LOCALAPPDATA 'gowork\bin'
}

function Fail([string]$Message) {
    throw "install: $Message"
}

$goModPath = Join-Path $PWD 'go.mod'
$inRepo = (Test-Path -LiteralPath $goModPath -PathType Leaf) -and
    (Select-String -LiteralPath $goModPath -Pattern '^module gowork$' -Quiet)

$tmpDir = $null
if ($inRepo) {
    $srcDir = $PWD.Path
    Write-Host "使用当前目录的源码：$srcDir"
}
else {
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Fail '需要 git，但没有找到' }
    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ('gowork-' + [guid]::NewGuid().ToString('N'))
    Write-Host "克隆 $repoUrl"
    git clone --depth 1 $repoUrl (Join-Path $tmpDir 'gowork')
    if ($LASTEXITCODE -ne 0) { Fail "克隆失败：$repoUrl" }
    $srcDir = Join-Path $tmpDir 'gowork'
}

try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Fail '需要 Go，但没有找到' }
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $exe = Join-Path $BinDir 'gowork.exe'

    Write-Host "构建 $exe"
    Push-Location $srcDir
    try {
        go build -trimpath -o $exe .\cmd\gowork
        if ($LASTEXITCODE -ne 0) { Fail '构建失败' }
    }
    finally {
        Pop-Location
    }

    Write-Host ''
    Write-Host "已安装：$exe"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $BinDir) {
        Write-Host ''
        Write-Host "注意：$BinDir 不在用户 PATH 上，先把它加进去："
        Write-Host "  [Environment]::SetEnvironmentVariable('Path',"
        Write-Host "    [Environment]::GetEnvironmentVariable('Path','User') + ';$BinDir', 'User')"
    }

    Write-Host ''
    Write-Host '把这一行加到 $PROFILE 末尾（echo $profile 可查看路径）：'
    Write-Host '  Invoke-Expression (& { (gowork init powershell | Out-String) })'
    Write-Host ''
    Write-Host '重开终端后运行 work；工作区用 GOWORK_WORKSPACE 或配置文件指定。'
}
finally {
    if ($tmpDir) {
        Remove-Item -LiteralPath $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
