# AGENTS.md

给在本仓库工作的 agent 的项目导览：实现机制、设计取舍、以及踩过的坑。
偏事实，不重复 README 里面向用户的用法说明。

## 项目形态

- 跨 Windows / macOS / Linux（含 WSL）的项目跳板 + 环境仪表盘。
- 全部逻辑在一个 Go 二进制里（`gowork.exe` / `gowork`），无外部运行时依赖；`go.mod` 要求 Go 1.26.5。
- 只有两个依赖：`golang.org/x/term`（终端宽度判断）与 `golang.org/x/sys`（Windows 控制台、磁盘空间）。
- shell 侧只有一层壳：`gowork init <shell>` 打印函数代码，用户在配置里 `eval` / `source`。
  仓库里不存放启动器脚本；`bin/` 是构建产物目录，已被 gitignore。

为什么必须有 shell 侧：子进程改不了父进程的工作目录。gowork 把目标目录写进
`--cwd-file` 指向的临时文件，shell 函数读出来再 `cd` / `Set-Location`。
二进制单独运行时一切逻辑照常，只是不会切目录。

## 一次运行的完整链路

1. shell 函数执行 `gowork --cwd-file=<临时文件> <用户参数>`（PowerShell 还会补
   `--indent=<n>` 与 `--project` / `--workspace`）。
2. `main()`：`enableVT()` → 若 `os.Args[1] == "init"` 走 `initShell` 直接返回 →
   `rewriteIndexArgs` 改写裸 `-N` → `parseFlags`。
3. `run()`：`resolveWorkspace` → `newRenderer` → `listProjects` →
   若参数像仓库地址则 `cloneProject` 并重扫项目，否则 `resolveTarget`。
4. `renderer.render`：清屏、窗口标题、警告、logo、仪表盘、格言。
5. 成功才把目标目录写入 `--cwd-file`；退出码 0 时 shell 函数读取并切目录。
6. 失败统一走 `fail`：错误写 stderr，解析类错误退出 1，参数错误退出 2；
   **不切换目录**。

注意：生成的函数把 `--cwd-file` 放在用户参数之前。所以 `work init` 里的 `init`
是**项目名**（`flag.Arg(0)`），不会触发 init 子命令；只有直接运行
`gowork init <shell>`（`init` 位于 `argv[1]`）才输出集成代码。

## 文件地图

| 文件 | 内容 |
| --- | --- |
| `main.go` | `main`、`run`、参数解析；`rewriteIndexArgs` 与 `valueFlags` |
| `config.go` | 工作区优先级、配置文件、缓存目录；平台相关的提示文案 |
| `projects.go` | `listProjects`、`resolveTarget`、`indexPattern`、`prefixMatches`、`gitInfo` |
| `render.go` | 仪表盘渲染、ANSI、`row` / `rule` / `printRecent`、宽度计算与截断 |
| `holiday.go` | 接口请求、事件整理与挑选、缓存、节日图标 |
| `clone.go` | 地址识别、仓库名推断与校验、`git clone` 直连控制台 |
| `sysinfo.go` | `fail`、`terminalWidth`（跨平台） |
| `sysinfo_windows.go` | `enableVT`（VT + UTF-8 代码页）、`diskFreeGB`（`GetDiskFreeSpaceEx`） |
| `sysinfo_posix.go` | `enableVT` 空实现、`diskFreeGB`（`statfs` 的 `Bavail * Bsize`） |
| `init.go` | `getInitScript`（switch）、`initShell`、三份 shell 模板 |
| `install.sh` / `install.ps1` | 构建并投放二进制，打印 PATH 与 init 提示；不改用户环境 |
| `.github/workflows/ci.yml` | 三平台测试 + shell 预检 |
| `.github/workflows/release.yml` | `v*` 标签发布 |

## 行为细节

### 工作区

解析优先级：`--workspace` > `GOWORK_WORKSPACE` > `config.json`。
配置与缓存共用 `os.UserCacheDir()/gowork`：Windows `%LOCALAPPDATA%\gowork`、
macOS `~/Library/Caches/gowork`、Linux/WSL `~/.cache/gowork`。
三者都没有时报错，不猜默认路径。

### 项目扫描与匹配

- `listProjects` 读工作区一级目录：跳过 `.` 开头，文件跳过；判断用 `os.Stat` 而不是
  `DirEntry.IsDir`，因此符号链接、junction 指向目录时都算项目。
- 按 `ModTime` 倒序；`-N` 取第 N 个（1 起），越界报 `有效范围: 1-N`。
- `resolveTarget`：完整名 → 前缀（大小写不敏感）→ 唯一命中跳转；多个命中取**名字最短**，
  并列时按字典序；同时返回候选列表，渲染时在清屏后以黄色警告列出。0 命中报目录不存在。
- 空项目名表示回工作区根目录。
- `gitInfo`：`.git` 存在（文件或目录都算，兼容 worktree）且 PATH 有 git，才显示分支与
  `git status --porcelain` 的非空行数；空分支名显示 `detached`。

### 仓库地址与克隆

- `repoURL` 与 git 的识别范围对齐：`http/https/ssh/git/ftp/ftps`、`git+ssh://`、
  `ssh+git://`、`file://`，以及 `[user@]host:path` 的 scp 形式；单字母主机
  （`C:\x`、`C:/x`）按盘符处理，不算地址；裸域名不算地址。
- `repoName`：去掉结尾 `/` 与 `.git`，scp 形式先按最后一个冒号切出路径，再取最后一段。
- `validRepoName` 拒绝空、`.`、`..` 与 `/\:*?"<>|`，避免写出工作区。
- 目标目录已存在则直接进入（幂等），不提示、不重复克隆。
- 需要克隆时先清屏，打印地址与目标，再 `git clone` 直连 stdin/stdout/stderr，
  让进度条、SSH 与凭据提示正常工作；失败报 `克隆失败：<地址>`，退出码 1。

### 仪表盘

- `newRenderer`：`tty = term.IsTerminal(stdout)`；颜色 = tty && 未传 `--no-color` &&
  `NO_COLOR` 为空；accent 从青/绿/黄/紫随机选。
- `render` 先清屏再输出警告，避免警告被清掉。清屏、窗口标题仅在 tty 下执行。
- 进入具体项目时，logo 倒数第二行末尾补 `on <项目名>`；放不下就另起一行；
  回工作区根目录不加。
- 行：时间（中文星期、24h）、位置（切换前的目标）、项目数、磁盘（可用 <10 GB 变红，
  取不到整行不显示）、Git（非仓库整行不显示）、节日、最近活跃（前 4，名字按显示宽度
  截到 35 列，后接 `MM-dd HH:mm`）。
- `rule` 宽度 = `min(终端宽度, 100) - 2*缩进`，只封顶，不设下限。
- 问候语按小时分段；底部从固定的格言表随机取一条。
- `runeWidth` 是 East Asian Width 的近似：控制字符与组合字符 0 列，CJK/全角/emoji 2 列，
  其余 1 列；宽窄不确定的字符按 1 列。已知 `⚠️`（U+26A0 + U+FE0F）会少算一列，
  只影响行尾内容与含 emoji 的项目名截断。

### 节假日

- 数据源：`https://uapis.cn/api/v1/misc/holiday-calendar`，按天查询今天。
- 请求参数：`date=<今天>`、`holiday_type=all`、`include_nearby=true`、`nearby_limit=30`。
  **不传 `exclude_past`**：它按服务端真实“今天”过滤，与传入的 `date` 无关。
- 响应里当天明细在 `days[0]`（`legal_holiday_name` / `legal_holiday_type` /
  `solar_festival` / `lunar_festival`），之后的事件在 `nearby.next`，之前的在
  `nearby.previous`；`holidays` 在日视图下通常为空，不使用。
- `collectEvents` 只收 `legal_rest`、`solar_festival`、`lunar_festival`；
  忽略 `legal_workday_adjust`（调休上班）与 `solar_term`（节气）。去重后按日期、类型排序。
- 挑选：先取今天或之后最近的 `legal_rest`；没有则退回最近的节日（没有放假天数）。
- 放假天数用 `restRun` 算**整段**长度：从选中日期向前找到假期开头再向后数，
  所以假期第 3 天看到的也是完整天数。这也是一直要缓存 `nearby.previous` 的原因。
- 缓存：`holiday-cache.json`，结构 `{version: 4, events: [{date, name, type}]}`。
  存的是整理后的事件列表，不是接口原文；读取时用同一套挑选逻辑重新算。
  只有全部事件都过去（挑不出结果）才重新请求；请求失败不写缓存。
- 调用预算：正常一次运行最多一次 HTTP 请求（超时 5s），其余时间走缓存。
- 图标按名称关键字匹配（元旦🎉、春节🧧、清明🌿、劳动🔧、端午🐉、中秋🥮、国庆🇨🇳 等），
  未命中用 📅。
- 已知限制：新一年放假安排公布前，接口只有公历/农历节日，没有法定放假数据；
  此时只显示节日兜底，不做往年规律预测。

### shell 集成

- 入口：`gowork init <shell>`；`shell` 大小写不敏感，`powershell` / `pwsh`、
  `sh` / `bash` / `zsh` / `posix`、`fish`。参数缺失或未知 shell 退出码 2。
- 三份模板都在 `init.go` 的 raw string 里；模板里外部命令统一加 `command` 前缀，
  PowerShell 的 cmdlet 用模块全名（`Microsoft.PowerShell.Management\...`），
  避免被用户别名或同名函数劫持。
- POSIX：`local` + `mktemp` + `command cat` + `\cd --`，返回二进制的退出码；
  找不到二进制返回 127，临时文件失败返回 1。
- fish：存在性用 `command -sq`，外部命令同样 `command` 前缀。
- PowerShell：`function global:work`，参数 `-Project` / `-IndentSize` / `-Workspace`；
  用 `Start-Process -NoNewWindow -Wait -PassThru` 让子进程直接接管控制台，
  绕开 PowerShell 输出管线，ANSI 颜色不会被 `$PSStyle.OutputRendering` 抹掉；
  参数逐个人工加引号后再拼命令行；从 `--cwd-file` 读取目标并 `Set-Location`；
  找不到二进制时 `Write-Error` 并把 `$LASTEXITCODE` 设为 127。
- 二进制查找：`GOWORK_BIN` 优先，否则 PATH 上的 `gowork`。
- 集成代码由二进制生成，升级 gowork 后无需重新配置 shell。

### 安装脚本

- `install.sh` / `install.ps1`：若当前目录是本仓库（`go.mod` 里 `module gowork`）就直接
  构建，否则 `git clone --depth 1` 到临时目录再构建。
- 默认目录：`~/.local/bin`（macOS / Linux）、`%LOCALAPPDATA%\gowork\bin`（Windows）；
  `--bin-dir` / `-BinDir` 可改，shell 脚本还认 `GOWORK_BIN_DIR`。
- 只打印 PATH 提示与 init 那一行，不改 PATH、不改 `$PROFILE`。
- PowerShell 里用 `throw` 而不是 `exit`：脚本常以 `irm ... | iex` 形式跑在当前会话，
  `exit` 会把终端一起关掉。
- 目前从源码构建，需要本机有 Go；安装脚本不下载预编译产物。

### CI 与发布

- `ci.yml`：push 到 `main` 与所有 PR 触发；三平台矩阵（ubuntu / macos / windows），
  `fail-fast: false`；跑 `gofmt -l`（有输出即失败）、`go vet`、gotestsum。
- 测试前安装并确认 shell：Linux 装 `fish zsh dash`，macOS 装 `fish`，Windows 用自带的
  PowerShell 5.1 / 7 与 Git Bash；逐个 `command -v` / `Get-Command`，缺失直接失败。
- 测试用 `go run gotest.tools/gotestsum@v1.13.0 --format=testname -- -count=1 ./...`：
  `go run` 不依赖 `GOPATH/bin` 在 PATH 上，版本固定；摘要会列出 skipped 数量。
- `release.yml`：push `v*` 标签触发；先在 ubuntu 跑 vet + 测试，再交叉编译
  linux/darwin/windows × amd64/arm64 六个产物；`-trimpath -ldflags '-s -w'`；
  tar.gz（Windows 为 zip），命名 `gowork_<版本>_<系统>_<架构>`；
  汇总 `checksums.txt`，用 `softprops/action-gh-release` 发布并自动生成 release notes。
- 发布不依赖 GoReleaser。

## 测试

```sh
gofmt -l .      # 应为空
go vet ./...
go test ./...
```

- **在沙箱外跑测试**：沙箱内 `git` 子进程受限，`TestGitInfo` 等会假失败。
- `init_test.go`：
  - 各 shell 名称、别名、大小写、错误参数；
  - `TestInitScriptSyntax` 把生成的代码交给 bash / zsh / dash / fish / pwsh / powershell
    解析，缺哪个就 skip；bash 系用 `-n`，PowerShell 用 `Parser::ParseFile`。
  - PowerShell 检查写临时文件（UTF-8 带 BOM），**不能走 stdin**（见坑 1）。
  - `TestCheckPowerShellDetectsError` 用坏脚本确认检查不是空转。
- `projects_test.go`：
  - `TestListProjectsFollowsDirLink` 直接建目录符号链接；Windows 上失败时启用
    `SeCreateSymbolicLinkPrivilege` 重试，仍建不了就 skip，不退回目录联接。
  - `TestListProjectsFollowsJunction`（仅 Windows）固定用 `mklink /J`，
    保证符号链接用例因权限跳过时，「链接指向目录算项目」仍然有覆盖。
- 跳过项取决于环境：缺 zsh / dash / fish 会跳过对应的语法检查；Windows 上建不了
  符号链接时跳过该用例。CI 会确保解释器存在。

## 踩过的坑

1. **PowerShell 从 stdin 读脚本会按控制台代码页解码**。GBK 代码页下 UTF-8 中文注释被解坏，
   换行被吞、行号错位，`ParseInput` 会报假的 `Unexpected token '}'`。
   修法：写 UTF-8 带 BOM 的临时文件，用 `Parser::ParseFile`。与 PowerShell 5.1 / 7 无关。
2. **Windows 建符号链接需要特权**。普通用户默认没有 `SeCreateSymbolicLinkPrivilege`；
   管理员进程里该特权也常是 Disabled；开发者模式的免提权标志也可能不生效。
   测试先直接建，失败则启用 `SeCreateSymbolicLinkPrivilege` 重试，仍不行就 skip，
   不退回目录联接；另有一个固定用目录联接的 Windows 用例，保证「链接指向目录算项目」
   仍有覆盖。生产代码只用 `os.Stat`，两种链接都会跟随。
3. **节假日接口的 `exclude_past` 按服务端当天过滤**，与传入的 `date` 无关。我们永远查今天，
   所以不传它；调试时不要拿历史日期去验证这个参数。
4. **日视图的当天信息在 `days[0]`**，`holidays` 通常为空；之后的事件在 `nearby.next`，
   之前的事件在 `nearby.previous`。
5. **假期中途只向后数会少算天数**。必须用 `nearby.previous` 找假期开头，`restRun` 才给出
   整段长度；缓存也必须保留 previous。
6. **缓存要存整理后的数据**，不要原样存接口响应；数据结构变化时递增
   `holidayCacheVersion`（当前 4）；版本不匹配的缓存直接丢弃。
7. **裸写 `-N` 会被 `flag` 包当成未知选项**。`rewriteIndexArgs` 在解析前改写成
   `--project=-N`；新增带值的选项必须同步加进 `valueFlags`，否则 `--flag -1` 会被误改。
8. **`init` 子命令只在 `argv[1]` 时生效**。shell 函数总把 `--cwd-file` 放最前面，
   所以 `work init` 是项目名；不要据此认为「`work init` 会打印脚本」。
9. **PowerShell 输出管线会吞 ANSI**。必须用 `Start-Process -NoNewWindow` 让子进程直接写主机；
   也因此 `$x = work` 捕获不到仪表盘（这是预期行为）。
10. **沙箱内跑测试会假失败**。凡是起 `git` 子进程或依赖临时目录权限的用例，都要在沙箱外执行。
11. **`go test` 不汇总 skip**。本地缺 shell 时会静默跳过；CI 用前置检查 + gotestsum 解决。
12. **`install.ps1` 不能用 `exit`**。`irm | iex` 在当前会话执行，`exit` 会关终端；用 `throw`。
13. **行尾规则按文件类型区分**：`.go` / `.sh` 必须 LF，`.ps1` 必须 CRLF；
    `.gitattributes` 只保留这些必要规则。

## 改动时的连带点

- 加 shell：`init.go` 的 switch、模板、`init_test.go` 用例、CI 安装与预检、README 的平台说明。
- 加平台相关函数：放 `sysinfo_windows.go` / `sysinfo_posix.go`，用构建标签隔开；
  其余逻辑必须能在任意平台 `go test`。
- 改节假日接口或字段：同步 `parseHoliday` / `collectEvents`、缓存结构并递增版本、相关测试、README。
- 加命令行选项：若是带值选项，同步 `main.go` 的 `valueFlags`；`--help` 里补说明。
- 改渲染：`render_test.go` 覆盖文案与宽度；注意清屏顺序和 tty 门控。

## 已知限制

- 字符宽度是近似值，emoji + 变体选择符可能差一列（只影响对齐与截断）。
- 新一年法定排期公布前只有节日兜底，不预测放假安排。
- 安装脚本只支持源码构建，需要本机 Go；不会下载 release 产物。
- Windows 上的 fish 不在 CI 覆盖范围（Linux / macOS 覆盖）。
