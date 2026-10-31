# gowork 安装脚本：下载最新正式 release 的二进制，装进用户目录，
# 并把 PATH 与 work 集成写进当前 shell 的配置（可用 -NoModify 跳过）。
#
# 用法：
#   irm https://raw.githubusercontent.com/CosmicHz/gowork/main/install.ps1 | iex
#   .\install.ps1 -BinDir D:\bin
#   .\install.ps1 -NoModify
#   .\install.ps1 -PreRelease
#   .\install.ps1 -FromSource
#
# 默认只支持 windows 的 amd64 / arm64 预编译产物；其他情况用 -FromSource。
# 重复执行会覆盖升级，已配置过的 PATH / shell 集成不会重复追加。
[CmdletBinding()]
param(
    [string]$BinDir,
    [switch]$NoModify,
    [switch]$PreRelease,
    [switch]$FromSource
)

$ErrorActionPreference = 'Stop'

$repo = 'CosmicHz/gowork'
$repoUrl = "https://github.com/$repo.git"
$latestReleaseApi = "https://api.github.com/repos/$repo/releases/latest"
$releasesApi = "https://api.github.com/repos/$repo/releases"

if (-not $BinDir) {
    $BinDir = Join-Path $env:LOCALAPPDATA 'gowork\bin'
}

function Fail([string]$Message) {
    throw "install: $Message"
}

function Enable-Tls12 {
    # Windows PowerShell 5.1 默认可能只协商 TLS 1.0，GitHub 需要 1.2。
    try {
        [Net.ServicePointManager]::SecurityProtocol =
            [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    }
    catch {
        # 老系统没有 Tls12 枚举时交给后续请求自己报错。
    }
}

function Get-AssetArch {
    $arch = $env:PROCESSOR_ARCHITECTURE
    if ($env:PROCESSOR_ARCHITEW6432) { $arch = $env:PROCESSOR_ARCHITEW6432 }
    switch ($arch.ToUpperInvariant()) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { Fail "不支持的架构：$arch（请用 -FromSource 自行构建）" }
    }
}

function Install-FromRelease {
    Enable-Tls12
    $arch = Get-AssetArch
    $headers = @{ 'User-Agent' = 'gowork-install' }

    # 默认用 /releases/latest（只含正式版）；-PreRelease 时改用列表第一条，
    # 未认证的列表接口不返回 draft，但包含预发布。
    if ($PreRelease) {
        try {
            $releases = Invoke-RestMethod -Uri $releasesApi -Headers $headers
        }
        catch {
            Fail '没有找到可安装的 release；请检查网络或 GitHub API 是否可用'
        }
        $release = @($releases) | Where-Object { -not $_.draft } | Select-Object -First 1
        if (-not $release) { Fail '没有找到可安装的 release；请检查网络或 GitHub API 是否可用' }
        Write-Host "使用预发布版本 $($release.tag_name)"
    }
    else {
        try {
            $release = Invoke-RestMethod -Uri $latestReleaseApi -Headers $headers
        }
        catch {
            Fail '没有找到正式发布的 release；需要预发布版本时加 -PreRelease'
        }
    }

    $tag = $release.tag_name
    $version = $tag -replace '^v', ''
    $asset = "gowork_${version}_windows_${arch}.zip"
    $base = "https://github.com/$repo/releases/download/$tag"

    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ('gowork-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
    try {
        $zip = Join-Path $tmpDir $asset
        Write-Host "下载 $base/$asset"
        Invoke-WebRequest -Uri "$base/$asset" -OutFile $zip -Headers $headers

        Write-Host '校验 sha256'
        $sumFile = Join-Path $tmpDir 'checksums.txt'
        Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $sumFile -Headers $headers
        $want = Get-Content -LiteralPath $sumFile |
            Where-Object { $_ -match [regex]::Escape($asset) } |
            ForEach-Object { ($_ -split '\s+')[0] } |
            Select-Object -First 1
        if (-not $want) { Fail "checksums.txt 中没有 $asset" }
        $got = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash
        if ($got -ne $want.ToUpperInvariant()) { Fail "sha256 校验失败：$asset" }

        $extractDir = Join-Path $tmpDir 'extract'
        Expand-Archive -LiteralPath $zip -DestinationPath $extractDir -Force
        $src = Join-Path $extractDir ("gowork_${version}_windows_${arch}\gowork.exe")
        if (-not (Test-Path -LiteralPath $src -PathType Leaf)) {
            Fail '压缩包结构异常：没有找到 gowork.exe'
        }

        New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
        $exe = Join-Path $BinDir 'gowork.exe'
        Write-Host "安装 $exe"
        # 先写临时文件再替换，避免中途失败留下半个二进制。
        $tmpExe = Join-Path $BinDir 'gowork.exe.new'
        Copy-Item -LiteralPath $src -Destination $tmpExe -Force
        try {
            Move-Item -LiteralPath $tmpExe -Destination $exe -Force
        }
        catch {
            Remove-Item -LiteralPath $tmpExe -Force -ErrorAction SilentlyContinue
            Fail "无法替换 $exe：文件可能正在运行，请关闭后重试"
        }
    }
    finally {
        Remove-Item -LiteralPath $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Install-FromSource {
    $tmpDir = $null
    try {
        $goModPath = Join-Path $PWD 'go.mod'
        $inRepo = (Test-Path -LiteralPath $goModPath -PathType Leaf) -and
            (Select-String -LiteralPath $goModPath -Pattern '^module gowork$' -Quiet)

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
    }
    finally {
        if ($tmpDir) {
            Remove-Item -LiteralPath $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

function Add-UserPath {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($null -eq $userPath) { $userPath = '' }
    $parts = @($userPath -split ';' | Where-Object { $_ -ne '' })

    $changed = $false
    if ($parts -notcontains $BinDir) {
        [Environment]::SetEnvironmentVariable('Path', (@($parts) + $BinDir) -join ';', 'User')
        $changed = $true
    }
    if (($env:Path -split ';') -notcontains $BinDir) {
        $env:Path = "$BinDir;$env:Path"
    }
    return $changed
}

# Add-ShellHook 往 $PROFILE 追加一个带标记的集成块。
# 返回 added / exists / utf16 / skipped，由调用方决定怎么提示。
function Add-ShellHook {
    $profilePath = $PROFILE
    if (-not $profilePath) { return 'skipped' }

    if (Test-Path -LiteralPath $profilePath -PathType Leaf) {
        $bytes = [System.IO.File]::ReadAllBytes($profilePath)
        if ($bytes.Length -ge 2) {
            $utf16 = ($bytes[0] -eq 0xFF -and $bytes[1] -eq 0xFE) -or
                ($bytes[0] -eq 0xFE -and $bytes[1] -eq 0xFF)
            if ($utf16) { return 'utf16' }
        }

        $text = Get-Content -LiteralPath $profilePath -Raw -ErrorAction SilentlyContinue
        if ($text -and ($text -match 'gowork installer' -or $text -match 'gowork init')) {
            return 'exists'
        }
    }
    else {
        $parent = Split-Path -Parent $profilePath
        if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    }

    $block = @(
        ''
        '# >>> gowork installer >>>'
        'Invoke-Expression (& { (gowork init powershell | Out-String) })'
        '# <<< gowork installer <<<'
        ''
    ) -join [Environment]::NewLine

    # 追加的是纯 ASCII 内容：UTF-8 与 ANSI 配置都能正确读取；
    # UTF-16 的 profile 已在上面跳过，避免写出混合编码。
    [System.IO.File]::AppendAllText(
        $profilePath,
        $block + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
    return 'added'
}

function Show-ManualSetup {
    Write-Host ''
    Write-Host '未修改 PATH 或 $PROFILE。手动步骤：'
    Write-Host "  1. 把 $BinDir 加入用户 PATH"
    Write-Host '  2. 在 $PROFILE 末尾加这一行：'
    Write-Host '     Invoke-Expression (& { (gowork init powershell | Out-String) })'
}

function Initialize-Environment {
    try {
        $changed = Add-UserPath
        if ($changed) {
            Write-Host "已把 $BinDir 加入用户 PATH（重开终端生效）"
        }
        else {
            Write-Host "PATH 已包含 $BinDir，无需修改"
        }
    }
    catch {
        Write-Host "自动修改 PATH 失败：$($_.Exception.Message)"
        Show-ManualSetup
        return
    }

    try {
        switch (Add-ShellHook) {
            'added' { Write-Host "已把 work 集成写入 $PROFILE" }
            'exists' { Write-Host 'work 集成已存在，跳过' }
            'utf16' { Write-Host '$PROFILE 是 UTF-16 编码，未自动修改'; Show-ManualSetup }
            default { Show-ManualSetup }
        }
    }
    catch {
        Write-Host "自动写入 $PROFILE 失败：$($_.Exception.Message)"
        Show-ManualSetup
    }
}

if ($FromSource) {
    Install-FromSource
}
else {
    Install-FromRelease
}

$exe = Join-Path $BinDir 'gowork.exe'
Write-Host ''
Write-Host "已安装：$exe"

if ($NoModify) {
    Write-Host ''
    Write-Host '未修改 PATH 或 $PROFILE（-NoModify）。'
    Show-ManualSetup
}
else {
    Initialize-Environment
}

Write-Host ''
Write-Host '重开终端后运行 work；工作区用 GOWORK_WORKSPACE 或配置文件指定。'
