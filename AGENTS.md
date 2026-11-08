# AGENTS.md

给在本仓库工作的 agent 的项目导览：实现机制、设计取舍、以及踩过的坑。
偏事实，不重复 README 里面向用户的用法说明。

## 项目形态

- 跨 Windows / macOS / Linux（含 WSL）的项目跳板 + 环境仪表盘。
- 全部逻辑在一个 Go 二进制里（`gowork.exe` / `gowork`），无外部运行时依赖；`go.mod` 要求 Go 1.26.5。
- 只有两个依赖：`golang.org/x/term`（终端宽度判断）与 `golang.org/x/sys`（Windows 控制台、磁盘空间）。
- 代码按领域分包：`cmd/gowork` 只把 `os.Args` 转给 `internal/cli`，其余都在 `internal/` 下（`ansi`、`cli`、`config`、`project`、`render`、`holiday`、`clone`、`shell`、`sysinfo`）。
- shell 侧只有一层壳：`gowork init <shell>` 打印函数代码（zsh 还带补全），用户在配置里 `eval` / `source`。
  仓库里不存放启动器脚本；`bin/` 是构建产物目录，已被 gitignore。

为什么必须有 shell 侧：子进程改不了父进程的工作目录。gowork 把目标目录写进
`--cwd-file` 指向的临时文件，shell 函数读出来再 `cd` / `Set-Location`。
二进制单独运行时一切逻辑照常，只是不会切目录。

## 一次运行的完整链路

1. shell 函数执行 `gowork --cwd-file=<临时文件> <用户参数>`；PowerShell 只翻译
   `-Project` / `-IndentSize` / `-Workspace`，其余参数原样透传。
2. `cmd/gowork/main.go`：只调用 `cli.Main(os.Args)`。
3. `cli.Main`：`sysinfo.EnableVT()` → `runSubcommand` 按 `os.Args[1]` 处理
   `init` / `complete` 并直接返回 → `config.Load()` →
   `NormalizeArgs`（先展开短选项，再改写裸 `-N`）→ `parseWith`。
4. `cli.Run`：`cfg.ResolveWorkspace` → `render.New` → `project.List` →
   若参数像仓库地址则 `clone.Project` 并重扫项目，否则 `project.Resolve`。
5. `render.Renderer.Render`：清屏、窗口标题、logo、仪表盘、格言，最后是提示。
6. 成功才把目标目录写入 `--cwd-file`；退出码 0 时 shell 函数读取并切目录。
7. 失败统一走 `sysinfo.Fail`：错误写 stderr，参数错误退出 2，业务错误退出 1；
   **不切换目录**。

注意：生成的函数把 `--cwd-file` 放在用户参数之前。所以 `work init` / `work complete`
是**项目名**（`flag.Arg(0)`），不会触发子命令；只有直接运行 `gowork init <shell>`
或 `gowork complete projects`（子命令位于 `argv[1]`）才走旁路。

## 目录结构

```text
cmd/gowork/            入口，只把 os.Args 交给 internal/cli
internal/
  ansi/                ANSI 前景色常量与主色调色板
  cli/                 参数解析、裸 -N 改写（RewriteIndexArgs → --recent）、编排（Main / Run）、complete 子命令
  config/              工作区优先级、配置文件、缓存目录；平台相关的提示文案
  project/             List、Resolve、IndexPattern、prefixMatches、GitInfo
  render/              仪表盘渲染、ANSI 使用、row / rule / printRecent、宽度计算与截断
  holiday/             接口请求、事件整理与挑选、缓存、节日图标、"节日"行文案
  clone/               地址识别、仓库名推断与校验、git clone 直连控制台
  shell/               Init（switch）、sh / fish / powershell / zsh 模板（zsh 含补全）
  sysinfo/             ColorEnabled、Fail、TerminalWidth（跨平台）
install.sh / install.ps1   构建并投放二进制，打印 PATH 与 init 提示；不改用户环境
.github/workflows/     ci.yml（三平台测试 + shell 预检）、release.yml（v* 标签发布）
```

`internal/sysinfo` 里的平台相关文件：

| 文件 | 内容 |
| --- | --- |
| `console_windows.go` | `EnableVT`（VT + UTF-8 代码页） |
| `console_posix.go` | `EnableVT` 空实现 |
| `disk_windows.go` | `DiskFreeGB`（`GetDiskFreeSpaceEx`） |
| `disk_posix.go` | `DiskFreeGB`（`statfs` 的 `Bavail * Bsize`） |

## 行为细节

### 工作区与配置

工作区解析优先级：`--workspace` > `GOWORK_WORKSPACE` > `config.json.workspace`。
配置与缓存共用 `os.UserCacheDir()/gowork`：Windows `%LOCALAPPDATA%\gowork`、
macOS `~/Library/Caches/gowork`、Linux/WSL `~/.cache/gowork`。
三者都没有时报错，不猜默认路径。

`config.json` 还配置仪表盘：`indent`、`no_color`、`recent_limit`、`recent_name_width`、
`disk_warn_gb`、`show_disk`、`show_git`、`show_holiday`、`show_recent`、`clear_screen`、`quotes`。
`load` 先取 `DefaultConfig()` 再 `json.Unmarshal`：缺省字段保留默认值，显式 `0/false`
覆盖默认值，所以 `Config` 字段是普通值、不再用指针。`Config.Validate()` 拦越界值
（缩进 ≥3、`recent_limit` ≥1、名称宽度 ≥5、磁盘阈值 ≥0、格言非空），错误信息带实际值。
`cli.parseWith` 以配置为默认，`--indent` / `--no-color` 覆盖。配置 JSON 损坏或越界会报错，
不静默忽略。

### 项目扫描与匹配

- `project.List` 读工作区一级目录：跳过 `.` 开头，文件跳过；判断用 `os.Stat` 而不是
  `DirEntry.IsDir`，因此符号链接、junction 指向目录时都算项目。
- 按 `ModTime` 倒序；`-N` 取第 N 个（1 起），越界报 `有效范围: 1-N`。
- `project.Resolve`：完整名 → 前缀（大小写不敏感）→ 唯一命中跳转；多个命中取**名字最短**，
  并列时按字典序。返回 `project.Warning` 组（首行 Message + 缩进 Details）：前缀多命中、
  `-N` 与目录同名或同前缀的歧义。0 命中报目录不存在。
- `-N` 同名歧义：输入 `-N` 且工作区有名为 `-N` 或以 `-N` 开头的目录（如 `-1-app`）时，
  提示索引优先、列出候选，并给出 `work ./-<完整名>` 的消歧写法；`./` 是字面精确匹配，
  不做前缀匹配，所以 `work ./-114` 不会匹配 `-114514`。用 `./-N` 明确指定时不再提示。
- 空项目名表示回工作区根目录。
- `project.GitInfo`：`.git` 存在（文件或目录都算，兼容 worktree）且 PATH 有 git，才显示分支与
  `git status --porcelain` 的非空行数；空分支名显示 `detached`。

### 命令行参数

- 入口先 `NormalizeArgs`：`ExpandShortFlags` 展开类 Unix 短选项，`RewriteIndexArgs`
  把裸 `-N` 改写成 `--recent=N`，最后由 `ReorderArgs` 把项目名之后的选项移到前面。
  顺序不能反，否则 `-p -1` 的值会被当成索引。
- `--recent N` 与 `-N` 等价；裸 `-N` 仍交给 `project.Resolve` 按索引处理，因此同名/同前缀提示不受影响。
  `--recent` 与 `--project` 都是目标选择器，同时出现、或再给项目名时报错；
  `-N` 也算目标，项目名之后再写会被重排成 `--recent` 并报冲突。
- `ReorderArgs` 把第一个位置参数之后的选项（含独立值）移到前面，`--` 之后不重排。
  因此 `work my-app -n` 与 `work -n my-app` 等价；项目名与目标选项同时出现则由
  `parseWith` 报错，未知选项也会移到前面交给 flag 报错。新增选项时要同步
  `longFlags` / `valueFlags`，重排与同名提示会自动覆盖。
- 短选项：`-p` / `-i` / `-w` / `-c` / `-n` / `-h`，分别对应项目、缩进、工作区、
  cwd 文件、禁用颜色与帮助。布尔项可组合（`-ni 5`、`-ni5`），带值项吃掉组合剩余
  字符或下一个参数；整段出现未知字符时原样保留，交给 `flag` 报错。`--` 作为终止符后不展开；
  如果它是上一个带值选项的值，则继续处理后续参数。
- `--` 之后的参数都是位置参数；第一个位置参数会被标记为 `ProjectAfterTerminator`，
  `Run` 改用 `project.ResolveWithoutIndex`：仍做完整名/前缀匹配，但不把 `-N` 当索引，
  所以 `work -- -1` 是进入名为 `-1` 的项目。PowerShell 会在进入函数前丢掉 `--`，
  该写法只在直接调用二进制或 POSIX shell 中可用；pwsh 用 `./-1`。
- `cli.collisionWarnings` 汇总 CLI 语义造成的同名歧义：`init` 子命令、选项名
  （如 `-n`、`-p`、`--no-color`）与工作区目录同名。只对 `fs.Visit` 实际解析到的选项
  提示；选项重排后，位置参数之后的选项也会算，`--` 之后的位置参数不算。
  `cwd-file`（shell 自动注入）与 `help` 不提示。
  提示按用户实际拼写的选项名比较目录名，消歧写法是 `work ./<选项名>`。
  `complete` 是只给补全用的内部旁路，不参与同名提示。

### 仓库地址与克隆

- `clone.IsURL` 与 git 的识别范围对齐：`http/https/ssh/git/ftp/ftps`、`git+ssh://`、
  `ssh+git://`、`file://`，以及 `[user@]host:path` 的 scp 形式；单字母主机
  （`C:\x`、`C:/x`）按盘符处理，不算地址；裸域名不算地址。
- `clone.repoName`：去掉结尾 `/` 与 `.git`，scp 形式先按最后一个冒号切出路径，再取最后一段。
- `clone.validRepoName` 拒绝空、`.`、`..` 与 `/\:*?"<>|`，避免写出工作区。
- 目标目录已存在则直接进入（幂等），不提示、不重复克隆。
- 需要克隆时先清屏，打印地址与目标（`Renderer.CloneNotice`），再 `git clone` 直连
  stdin/stdout/stderr，让进度条、SSH 与凭据提示正常工作；失败报 `克隆失败：<地址>`，退出码 1。

### 仪表盘

- `render.New`：`tty = term.IsTerminal(stdout)`；颜色由 `sysinfo.ColorEnabled` 判定
  （tty && 未传 `--no-color` && `NO_COLOR` 为空）；accent 从青/绿/黄/紫随机选。
  默认渲染配置由 `render.DefaultOptions()` 提供，数值默认直接引用 `config.Default*` /
  `config.Min*`，不要再在 render 里复制一份。调用方从 `DefaultOptions()` 起步再覆盖字段；
  `Options` 的 `Show*` / `ClearScreen` 是正向 bool，零值表示关闭，不能拿 `Options{}` 当默认。
- `Render` 先清屏，提示放在仪表盘最下方（格言之后）：每组首行带 ⚠、明细缩进、
  组间空一行。这样既不会被清屏清掉，前缀歧义与同名冲突同时出现时也不会混在一起。
  清屏、窗口标题仅在 tty 下执行。
- 进入具体项目时，logo 倒数第二行末尾补 `on <项目名>`；放不下就另起一行；
  回工作区根目录不加。
- 行：时间（中文星期、24h）、位置（切换前的目标）、项目数、磁盘（可用低于
  `disk_warn_gb`（默认 10 GB）变红，取不到整行不显示）、Git（非仓库整行不显示）、
  节日、最近活跃（默认前 `recent_limit` 个，名字按显示宽度截到
  `recent_name_width` 列，后接 `MM-dd HH:mm`）。
- `rule` 宽度 = `min(终端宽度, 100) - 2*缩进`，只封顶，不设下限。
- 问候语按小时分段；底部从格言表（默认内置，可由 `quotes` 覆盖）随机取一条。
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
  `zsh`、`sh` / `bash` / `posix`、`fish`。参数缺失或未知 shell 退出码 2。
  `zsh` 输出 sh 函数体 + 补全；`sh` 仍接受 zsh，但不含补全（保持向后兼容）。
- 模板都在 `internal/shell/shell.go` 的 raw string 里：`shWorkFunc` 是三个类 Unix shell
  共用的函数体，`initSH` / `initZsh` 各自拼上头部与（zsh 的）补全；
  模板里外部命令统一加 `command` 前缀，
  PowerShell 的 cmdlet 用模块全名（`Microsoft.PowerShell.Management\...`），
  避免被用户别名或同名函数劫持。
- POSIX：`local` + `mktemp` + `command cat` + `\cd --`，返回二进制的退出码；
  找不到二进制返回 127，临时文件失败返回 1。
- fish：存在性用 `command -sq`，外部命令同样 `command` 前缀。
- zsh：在 `shWorkFunc` 后追加 `_gowork` 补全与 `compdef _gowork work`。候选项目名来自
  `gowork complete projects`（每行一个，按 `project.List` 的最近修改倒序）。
  补全用 `_arguments` 描述选项；裸 `-N` 在调用 `_arguments` 前单独拦下，否则会被当成
  未知选项。`compdef` 由 `compinit` 定义，模板用 `(( $+functions[compdef] ))` 守卫，
  没加载 compinit 时静默跳过；不自动替用户跑 compinit。
  组合短选项（`-ni5`）、紧凑取值（`-pfoo`）与 `--` 之后的字面参数不做补全。
- PowerShell：`function global:work` **不用 `param` 块**——用了会把 `--indent 5` 的
  `--indent` 绑到第一个位置参数，原生参数无法透传；没有 param 块时所有实参都在 `$args`。
  脚本只把 Windows 写法 `-Project` / `-IndentSize` / `-Workspace`（含 `-Proj` / `-Ind` /
  `-Work` 等无歧义缩写）翻译成原生长选项，其余（`--no-color`、`-n`、`-1`、项目名）原样透传。
  小写单字母 `-p` / `-i` / `-w` 保留为原生短选项，不参与缩写翻译。
  保留 `param` 块的替代方案（含 `ValueFromRemainingArguments`）验证过不行：PowerShell 仍会把
  第一个原生 token（如 `--no-color`）绑给第一个位置参数。代价是失去参数元数据、补全与
  `Get-Help`，`[int]` 类型转换移到 gowork，`-Project -IndentSize` 会把后者当值，且 PowerShell
  会在进入函数前丢掉 `--`。这些属于取舍，不要为了“像原生函数”再改回 `param` 块。
  用 `Start-Process -NoNewWindow -Wait -PassThru` 让子进程直接接管控制台，
  绕开 PowerShell 输出管线，ANSI 颜色不会被 `$PSStyle.OutputRendering` 抹掉；
  参数逐个人工加引号后再拼命令行；从 `--cwd-file` 读取目标并 `Set-Location`；
  找不到二进制时 `Write-Error` 并把 `$LASTEXITCODE` 设为 127。
  模板（含注释与提示语）必须保持纯 ASCII，原因见坑 1。
- 二进制查找：`GOWORK_BIN` 优先，否则 PATH 上的 `gowork`。
- 集成代码由二进制生成，升级 gowork 后无需重新配置 shell。

### 安装脚本

- 默认流程：从最新正式 release 下载对应平台产物（`gowork_<版本>_<系统>_<架构>.tar.gz` / `.zip`），
  用 `checksums.txt` 校验 sha256，解压后原子替换二进制；随后修改 PATH 与 shell 集成。
  `--from-source` / `-FromSource` 才走源码构建（当前目录是本仓库就直接构建，否则浅克隆，
  构建目标 `./cmd/gowork`）。
- 平台映射：`uname -s` 的 Linux / Darwin，`uname -m` 的 x86_64 / aarch64；Windows 读
  `PROCESSOR_ARCHITECTURE`（优先 `PROCESSOR_ARCHITEW6432`）。其他系统或架构报错并提示源码构建。
- release 选择：默认 `/releases/latest`（只返回正式版，draft 与预发布都被跳过）；
  `--pre-release` / `-PreRelease` 时改用 `/releases` 列表第一条（未认证列表不含 draft，
  但包含预发布）。默认路径没有正式版时报错并提示加 `--pre-release`。发布只上传版本化文件名，
  所以资产名靠 tag 拼出来；`release.yml` 按 tag 里的 `-` 自动设置 `prerelease`。
- 默认目录：`~/.local/bin`（macOS / Linux）、`%LOCALAPPDATA%\gowork\bin`（Windows）；
  `--bin-dir` / `-BinDir` 可改，shell 脚本还认 `GOWORK_BIN_DIR`。`--no-modify` / `-NoModify`
  只装二进制，跳过下面所有环境修改。
- PATH 与集成（POSIX）：按 `$SHELL` 选 rc——zsh `~/.zshrc`、bash `~/.bashrc`（macOS 为
  `~/.bash_profile`）、fish `${XDG_CONFIG_HOME:-~/.config}/fish/config.fish`、sh/dash `~/.profile`；
  写入的 hook 分别是 `init zsh` / `init sh` / `init fish`。zsh 用 `init zsh` 才有补全。
  识别不了就只打印手动步骤。写入块以 `# >>> gowork installer >>>` 标记，已有标记就整体跳过；
  目录已在 `$PATH` 或 rc 里出现时不重复追加；写入前 `mkdir -p` 父目录。
  旧块里的 `init sh` 不会被自动改成 `init zsh`，这是刻意保持的向后兼容。
- PATH 与集成（Windows）：用 `[Environment]::SetEnvironmentVariable('Path', ..., 'User')` 追加
  用户 PATH（不用 `setx`，避开长度截断），同时更新当前进程 `$env:Path`；`$PROFILE` 追加纯 ASCII
  集成块。profile 是 UTF-16 时不自动写，改为打印手动步骤，避免混合编码。
- 自动配置失败只警告并打印手动步骤，不影响二进制安装；重复执行是幂等的。
- PowerShell 里用 `throw` 而不是 `exit`：脚本常以 `irm ... | iex` 形式跑在当前会话，
  `exit` 会把终端一起关掉。
- 下载路径需要网络与 GitHub API（未认证限流 60 次/小时）；源码构建需要本机 Go。

### CI 与发布

- `ci.yml`：push 到 `main` 与所有 PR 触发；三平台矩阵（ubuntu / macos / windows），
  `fail-fast: false`；跑 `gofmt -l`（有输出即失败）、`go vet`、gotestsum。
- 测试前安装并确认 shell：Linux 装 `fish zsh dash`，macOS 装 `fish`，Windows 用自带的
  PowerShell 5.1 / 7 与 Git Bash；逐个 `command -v` / `Get-Command`，缺失直接失败。
- 测试用 `go run gotest.tools/gotestsum@v1.13.0 --format=testname -- -count=1 ./...`：
  `go run` 不依赖 `GOPATH/bin` 在 PATH 上，版本固定；摘要会列出 skipped 数量。
- `release.yml`：push `v*` 标签触发；先在 ubuntu 跑 vet + 测试，再交叉编译
  linux/darwin/windows × amd64/arm64 六个产物（`./cmd/gowork`）；`-trimpath -ldflags '-s -w'`；
  tar.gz（Windows 为 zip），命名 `gowork_<版本>_<系统>_<架构>`；
  汇总 `checksums.txt`，用 `softprops/action-gh-release` 发布并自动生成 release notes。
- `release.yml` 用 `contains(github.ref_name, '-')` 自动把 alpha / beta / rc 标成 prerelease；
  否则 `/releases/latest` 会把测试版当成正式版发给安装脚本。
- 发布不依赖 GoReleaser。

## 测试

```sh
gofmt -l .      # 应为空
go vet ./...
go test ./...
```

- **在沙箱外跑测试**：沙箱内 `git` 子进程受限，`TestGitInfo` 等会假失败。
- `internal/cli/cli_test.go` 的 `TestOptionsAfterProjectAreParsed` 钉住参数重排：
  项目名之后的 `-n` 必须生效。`TestParseRejectsProjectWithTargetFlag` 钉住
  项目名与 `--project` / `--recent` / `-N` 的冲突。
- `internal/shell/shell_test.go`：
  - 各 shell 名称、别名、大小写、错误参数；
  - `TestScriptSyntax` 把生成的代码交给 bash / zsh / dash / fish / pwsh / powershell
    解析，缺哪个就 skip；bash 系用 `-n`，PowerShell 用 `Parser::ParseFile`。
    其中 `{"zsh","zsh"}` 专门解析带补全的 zsh 模板。
  - `TestZshTemplateIncludesCompletion` / `TestShTemplateHasNoCompletion` 钉住
    「zsh 有补全、sh 没有」的分工。
  - `TestZshCompletionRegisters` 在真实 zsh 里先 `compinit` 再执行模板，确认
    `_comps[work] == _gowork`；没装 zsh 就 skip。
  - PowerShell 检查写临时文件（UTF-8 带 BOM），**不能走 stdin**（见坑 1）。
  - `TestCheckPowerShellDetectsError` 用坏脚本确认检查不是空转。
- `internal/cli/complete_test.go`：`writeProjectNames` 的输出顺序（最近修改在前）、
  隐藏目录/文件跳过、工作区不存在时无输出；`completeSubcommand` 的用法错误与
  环境变量指定工作区后的成功路径；`TestRunSubcommand` 钉住子命令只在 `argv[1]` 生效，
  shell 注入 `--cwd-file` 后 `complete` / `init` 按项目名处理。
- `internal/project/project_test.go`：
  - `TestListFollowsDirLink` 直接建目录符号链接；Windows 上失败时启用
    `SeCreateSymbolicLinkPrivilege` 重试，仍建不了就 skip，不退回目录联接。
  - `TestListFollowsJunction`（仅 Windows）固定用 `mklink /J`，
    保证符号链接用例因权限跳过时，「链接指向目录算项目」仍然有覆盖。
- `internal/sysinfo/sysinfo_test.go` 的 `TestColorEnabled` 钉住颜色开关的三个条件。
- `internal/cli/cli_test.go` 覆盖裸 `-N` 改写，含各带值选项不应被改写的情况。
- 跳过项取决于环境：缺 zsh / dash / fish 会跳过对应的语法检查；Windows 上建不了
  符号链接时跳过该用例。CI 会确保解释器存在。

## 踩过的坑

1. **PowerShell 从 stdin 读脚本会按控制台代码页解码**。GBK 代码页下 UTF-8 中文注释被解坏，
   换行被吞、行号错位，`ParseInput` 会报假的 `Unexpected token '}'`。
   修法：写 UTF-8 带 BOM 的临时文件，用 `Parser::ParseFile`。与 PowerShell 5.1 / 7 无关。
   同一根因还有第二处：`gowork init powershell` 的输出经过管道（`| Out-String`）时，pwsh
   也按控制台代码页解码，中文会吞掉换行，把 `function global:work {` 并进注释，profile 里的
   `Invoke-Expression` 报 `Unexpected token '}'`。所以这份模板的注释与提示语必须保持纯 ASCII，
   `TestPowerShellTemplateIsASCII` 会拦；其余 shell 的模板不经过这层解码，不受影响。
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
   `cacheVersion`（当前 4）；版本不匹配的缓存直接丢弃。
7. **裸写 `-N` 会被 `flag` 包当成未知选项**。`cli.RewriteIndexArgs` 在解析前改写成
   `--recent=N`；新增带值的选项必须同步加进 `cli.valueFlags`，否则 `--flag -1` 会被误改。
   解析阶段的 `project.Resolve` 仍按索引优先，因此工作区里真有同名目录时只提示、
   不改变行为；`./-1` 是精确写法，直接二进制或 POSIX shell 也可以用 `-- -1`
   （PowerShell 会丢掉 `--`，pwsh 用 `./-1`）。
8. **子命令只在 `argv[1]` 时生效**（`init`、`complete`）。shell 函数总把 `--cwd-file`
   放最前面，所以 `work init` / `work complete` 是项目名；不要据此认为「`work init`
   会打印脚本」。
9. **PowerShell 输出管线会吞 ANSI**。必须用 `Start-Process -NoNewWindow` 让子进程直接写主机；
   也因此 `$x = work` 捕获不到仪表盘（这是预期行为）。
10. **沙箱内跑测试会假失败**。凡是起 `git` 子进程或依赖临时目录权限的用例，都要在沙箱外执行。
11. **`go test` 不汇总 skip**。本地缺 shell 时会静默跳过；CI 用前置检查 + gotestsum 解决。
12. **`install.ps1` 不能用 `exit`**。`irm | iex` 在当前会话执行，`exit` 会关终端；用 `throw`。
13. **行尾规则按文件类型区分**：`.go` / `.sh` 必须 LF，`.ps1` 必须 CRLF；
    `.gitattributes` 只保留这些必要规则。
14. **Bash 的 `{ ... } >>缺失目录` 会返回 0**。追加 rc 时不能靠分组重定向判断失败，否则会
    误报成功且什么都没写；改用单条 `printf` 追加，并在前面 `mkdir -p` 父目录。
15. **Windows 检出的 `go.mod` 是 CRLF**。`grep -qx 'module gowork' go.mod` 匹配不到，
    会让源码模式误判为「不在仓库」而去克隆；改用前缀匹配，并由 `.gitattributes` 强制
    `go.mod` / `go.sum` 为 LF。
16. **EXIT trap 的最后一条命令决定退出码**。`[ -n "$tmp_dir" ] && rm -rf ...` 在变量为空时
    返回 1，会把成功退出的脚本变成失败；trap 里用 `if`，不要用 `&&`。
17. **zsh 补全里的裸 `-N` 必须先拦下**。`_arguments` 会把 `-1` 当未知选项，
   所以模板在调用 `_arguments` 前用 `[[ $words[CURRENT] == -[0-9]* ]]` 单独给
   `-1..-N`。另外 `compdef` 只有加载 `compinit` 后才存在：不要自动替用户跑 compinit，
   也不要在未加载时打印错误，否则会污染 shell 启动。补全候选来自
   `gowork complete projects`，失败时靠 `2>/dev/null` 静默。

## 改动时的连带点

- 加 shell：`internal/shell/shell.go` 的 switch、模板、`shell_test.go` 用例、CI 安装与预检、README 的平台说明。
  若 shell 自带补全（如 zsh），同步 `complete` 子命令的候选类型与 `TestScriptSyntax` 的解析用例。
  PowerShell 系模板的输出必须纯 ASCII（`TestPowerShellTemplateIsASCII` 会拦），原因见坑 1。
- 加平台相关函数：放 `internal/sysinfo` 的 `*_windows.go` / `*_posix.go`，用构建标签隔开；
  其余逻辑必须能在任意平台 `go test`。
- 改节假日接口或字段：同步 `parseHoliday` / `collectEvents`、缓存结构并递增版本、相关测试、README。
- 加命令行选项：在 `internal/cli` 的 `Parse` 里注册；若是带值选项，同步 `valueFlags`；
  加短选项时同步 `shortFlags` / `shortBoolFlags`；同时加进 `longFlags`（重排与同名提示依赖它）；
  `--help` 里补说明。
- 改渲染：`internal/render/render_test.go` 覆盖文案与宽度；注意清屏顺序和 tty 门控。
  默认值统一走 `render.DefaultOptions()` + `config.Default*` / `config.Min*`，不要在 render 里复制常量。
- 改入口或新增子命令：`cmd/gowork/main.go` 只做转发，逻辑落在 `internal/cli`；
  子命令统一在 `runSubcommand` 里按 `args[1]` 分派（`Main` 只调用它）。shell 函数总把
  `--cwd-file` 注入到 `argv[1]`，所以 `work <名字>` 永远到不了子命令——这是刻意的：
  加子命令默认不需要改 shell 模板。只有当子命令必须能通过 `work` 调用时，才要改所有
  shell 模板（sh / zsh / fish / powershell），并补对应 shell 测试。
  新增只读旁路时优先复用 `config.Load` + `project.List`。
- 改 release 产物命名或打包结构：同步 `install.sh` / `install.ps1` 的资产名拼接、解压路径，
  以及 `checksums.txt` 的校验逻辑。
- 改安装脚本的 PATH / shell 配置行为：同步 README 的「自动修改的位置」、本节说明与标记块格式。

## 已知限制

- 字符宽度是近似值，emoji + 变体选择符可能差一列（只影响对齐与截断）。
- 新一年法定排期公布前只有节日兜底，不预测放假安排。
- 安装脚本默认下载最新 release，只覆盖 linux / darwin / windows 的 amd64 / arm64；
  其他平台或需要最新源码时用 `--from-source` / `-FromSource`，此时需要本机 Go。
- 一键脚本默认只装最新正式版；加 `--pre-release` / `-PreRelease` 才装最新预发布版本。
- 一键脚本只自动配置当前 shell（`$SHELL` 对应的 rc / PowerShell 的 `$PROFILE`）；
  其他 shell 需要手动加同样的两行；`--no-modify` / `-NoModify` 可以完全不改环境。
- Windows 上的 fish 不在 CI 覆盖范围（Linux / macOS 覆盖）。
- **PowerShell 参数对应（待补）**：目前 PowerShell 只翻译 `-Project` / `-IndentSize` /
  `-Workspace`（含无歧义缩写），其余原生参数（`--recent`、`--no-color`、短选项等）靠透传，
  已经可用。目标是再给所有原生参数提供 pwsh 风格别名（如 `-Recent` / `-NoColor`），
  并与原生参数一一对应；实现时同步模板、测试与 README。
- **颜色开关的参数穿透（待改进）**：`sysinfo.ColorEnabled(isTTY, noColor)` 是唯一判定，仪表盘与
  `sysinfo.Fail` 共用，tty 按各自的流判断。但 `--no-color` 只能从 `cli` 传进 `Fail`，为此多了一个
  `bool` 参数，目前只有 `cli.Main -> sysinfo.Fail` 一层，先接受。若错误输出还要别的配置，改为传
  `Options` 或收一个统一配置，不要继续加布尔参数；也不要改用包级变量（与 `Options` 双轨）
  或 `os.Setenv("NO_COLOR", ...)`（会污染 `git clone` 等子进程）。
