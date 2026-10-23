# gowork

> 你的工作需要一场开机仪式。

gowork是一个**会迎接你**的 cd。它帮助你简单、快速、炫酷地进入工作区内的项目文件夹，并在它们之间跳转。

```text
PS C:\Projects\gowork> work awe
   ██╗    ██╗ ██████╗ ██████╗ ██╗  ██╗
   ██║    ██║██╔═══██╗██╔══██╗██║ ██╔╝
   ██║ █╗ ██║██║   ██║██████╔╝█████╔╝
   ██║███╗██║██║   ██║██╔══██╗██╔═██╗
    ╚███╔███╔╝╚██████╔╝██║  ██║██║  ██╗  on awesome-project
     ╚══╝╚══╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝
   ──────────────────────────────────────────────────────────────────────────
   下午好，效率正高 🚀

      时间        2026-10-17  星期六  16:17:18
      位置        C:\Projects\awesome-project
      项目        67 个项目
      磁盘        114.0 GB 可用
      Git         chore/important-feature  ·  5 项未提交
      节日        元旦🎉  还有 76 天  (放假 3 天)

     最近活跃
    1  gowork                              10-11 21:59
    2  openclaw                            10-11 21:57
    3  deepseek-harness                    10-11 21:55
    4  lolcat                              10-11 21:17

   ──────────────────────────────────────────────────────────────────────────
     代码是写给人看的，顺便让机器运行。
```

支持 Windows、macOS 与 Linux（含 WSL）。Windows 用 PowerShell 7，macOS / Linux 用 bash 或 zsh；
从源码构建需要 Go 1.26.5 或更新。

## 安装

### Windows（PowerShell 7）

```powershell
# 0. 克隆
git clone https://github.com/CosmicHz/gowork.git
cd gowork

# 1. 构建（产物落在 bin\gowork.exe）
go build -o bin\gowork.exe .

# 2. 把 bin 目录加进用户级 PATH
[Environment]::SetEnvironmentVariable('Path',
  [Environment]::GetEnvironmentVariable('Path','User') + ';' + (Join-Path $PWD 'bin'), 'User')

# 3. 配置工作区（三选一，见下节）
[Environment]::SetEnvironmentVariable('GOWORK_WORKSPACE', 'C:\Projects', 'User')

# 4. 开始使用！（重启终端后）
work
```

### macOS / Linux（bash、zsh）

```sh
# 0. 克隆
git clone https://github.com/CosmicHz/gowork.git
cd gowork

# 1. 构建（产物是 bin/gowork，没有 .exe 后缀）
go build -o bin/gowork .

# 2. 把可执行文件放进 PATH（下面假设 ~/.local/bin 已在你 PATH 里）
install -m 755 bin/gowork ~/.local/bin/gowork

# 3. 装 shell 入口：必须是函数才能改当前会话的目录，所以是 source 而不是执行
echo ". $PWD/bin/work.sh" >> ~/.zshrc   # bash 用户写 ~/.bashrc；fish 用户改成 source bin/work.fish

# 4. 配置工作区（三选一，见下节）
echo 'export GOWORK_WORKSPACE="$HOME/Projects"' >> ~/.zshrc

# 5. 开始使用！（重启终端后）
work
```

不想动 PATH 的话，也可以设 `GOWORK_BIN=/path/to/gowork` 直接指向可执行文件。

## 配置工作区

gowork 不知道该去哪里找你的项目，按下面的优先级解析：

| 优先级 | 方式 |
| --- | --- |
| 1 | `work -Workspace <路径>`（PowerShell）/ `work --workspace <路径>`（bash、zsh），临时指定 |
| 2 | 环境变量 `GOWORK_WORKSPACE` |
| 3 | 配置文件：Windows `%LOCALAPPDATA%\gowork\config.json`、macOS `~/Library/Caches/gowork/config.json`、Linux/WSL `~/.cache/gowork/config.json` |

三者都没有时会报错，并按当前平台给出对应写法。

```powershell
# 环境变量（用户级，持久）
[Environment]::SetEnvironmentVariable('GOWORK_WORKSPACE', 'C:\Projects', 'User')

# 或者配置文件
$dir = "$env:LOCALAPPDATA\gowork"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
'{ "workspace": "C:\\Projects" }' | Set-Content "$dir\config.json" -Encoding utf8
```

```sh
# 环境变量（写进 ~/.zshrc 或 ~/.bashrc 才持久）
export GOWORK_WORKSPACE="$HOME/Projects"

# 或者配置文件
mkdir -p ~/.cache/gowork
printf '{ "workspace": "%s/Projects" }\n' "$HOME" > ~/.cache/gowork/config.json
```

## 用法

### 命令一览

| 命令 | 行为 |
| --- | --- |
| `work` | 回到工作区根目录 |
| `work my-app` | 跳转到 `my-app` |
| `work my-a` | 前缀匹配，唯一命中就跳过去 （[详细说明](#项目名匹配)） |
| `work my` | 多个命中时跳最短匹配，并在清屏后列出全部候选 （[详细说明](#项目名匹配)） |
| `work -1` | 按最近修改排序，跳第 N 个 |
| `work <仓库地址>` | 克隆到工作区并进入 |
| `work my-app -IndentSize 5` | 左侧缩进改为 5（默认 3，最小 3） |
| `work my-app -Workspace D:\code` | 本次改用它作为工作区 |

> 选项拼写随启动器而变：PowerShell 启动器把参数翻译成 gowork 的选项，所以写 `-IndentSize` / `-Workspace`；
> bash、zsh 的 `work` 把参数原样透传给 gowork，所以写 `--indent` / `--workspace`。

### 项目名匹配

假设工作区为 `C:\Projects`，含 `my-app`、`my-service` 等项目。
执行`work <名称>`时，程序按顺序尝试：

1. **完整名**：工作区文件夹内，`<名称>` 存在且是目录 → 直接跳转。
   如 `work my-app` 会跳转到 `C:\Projects\my-app`。
2. **前缀匹配**（大小写不敏感）：取所有以名称开头的项目。
   - 命中 1 个 → 跳转到它。
     如 `work my-a` 会跳转到 `C:\Projects\my-app`。
   - 命中多个 → 跳转到**名字最短**的那个；长度并列时取字典序靠前的目录名。
     与此同时清屏后列出全部匹配到的目录名。
     如 `work my` 会跳转到 `C:\Projects\my-app`，并显示

     ```text
       ⚠ 前缀 'my' 匹配到多个项目，跳转到最短匹配：
           my-app
           my-service
     ```

   - 命中 0 个 → 报错 `目录不存在：<完整路径>`。

3. 解析失败则**不清屏、不切换目录**，退出码 1。

工作区里以 `.` 开头的目录会被跳过；指向目录的符号链接算项目。

### `-N` 索引

`-1` 是**最近修改**的项目，`-2` 次之，按子目录的 LastWriteTime 倒序。越界时报 `项目索引超出范围，有效范围: 1-<总数>` 。

### 仓库地址

只认两种形式：带协议的 URL（`http` `https` `ssh` `git` `ftp` `ftps`）与 scp 形式
（`git@host:owner/repo.git`）。裸写 `github.com/foo/bar` **不认**——它会被当成项目名去匹配，
然后报「目录不存在」。这是刻意的，不猜，避免误判。

- 本地目录名 = 地址最后一段路径去掉 `.git`；scp 形式先按冒号切出路径部分。
- 名称非法（空、`.`、`..`，或含路径分隔符与通配符）时报 `无法从地址推断仓库名`，
  **不会**跑到工作区外面去写。
- **目录已存在则静默直接进入**，不重复克隆——所以重复执行是幂等的。
- 需要克隆时：先清屏，打印地址与目标，然后 git 直连控制台（进度条、SSH 与凭据提示都正常）。
  成功后重新扫描工作区，仪表盘会把新仓库排到「最近活跃」第 1 位。
  失败时报 `克隆失败：<地址>`，退出码 1，git 的原始输出保留在屏幕上。

### 仪表盘

| 行 | 含义 | 取不到时 |
| --- | --- | --- |
| 时间 | 本地时间、中文星期、24 小时制 | — |
| 位置 | 切换后的目录（渲染发生在切换之前，所以显示的正是目标） | — |
| 项目 | 工作区下的项目数 | — |
| 磁盘 | 目标所在卷对**当前用户**可用的空间；小于 10 GB 显示为红色 | 整行不显示 |
| Git | 当前分支（空则显示 `detached`）· `git status --porcelain` 的非空行数 | 非仓库或无 git → 整行不显示 |
| 节日 | 见下 | 显示 `数据获取失败⚠️` |
| 最近活跃 | 按修改时间倒序前 4 个；名字按显示宽度截到 35 列（超出则留 32 列加 `...`），后接 `MM-dd HH:mm` | 没有项目 → 整段不显示 |

标题与问候语：

- 窗口标题设为 `WORK | <项目名>`。
- 进入具体项目时，logo 倒数第二行末尾补 `on <项目名>`（亮白色）；回工作区根目录时不加。
  放不下时（缩进 + logo 35 列 + 2 空格 + 后缀 > 终端宽度）改为另起一行。
- 问候语按小时分段：小于 6 点「夜深了」、小于 12「早上好」、小于 14「中午好」、
  小于 18「下午好」、其余「晚上好」。
- 底部随机一条格言。

### 节假日

- 取的是**下一个放假日**。接口语义是「严格晚于给定日期」，所以查询用**昨天**的日期——
  否则正好放假当天会被算成「还有 1 天」。
- 文案分档：`就是今天！快乐！` / `就在明天！` / `还有 N 天，倒计时开始！`（N ≤ 3）/ `还有 N 天`；
  接口给出放假天数时追加 `(放假 N 天)`。
- 取不到时显示 `数据获取失败⚠️`，并且**不写缓存**，下次运行仍会重试。
- 缓存位置与跨年空窗期见「已知问题」。

### 退出码与失败行为

| 码 | 含义 |
| --- | --- |
| 0 | 成功（并写入目标目录，启动器据此 `Set-Location` / `cd`） |
| 1 | 业务错误：目录不存在、索引越界、克隆失败、未配置工作区、推断不出仓库名 |
| 2 | 参数错误（仅直接调 exe 时可达，例如 `--indent` 小于 3） |

失败时统一**不切换目录**，错误以红字写到 stderr。解析阶段的失败不清屏（保留现场），
克隆失败时屏幕已清（因为要显示克隆过程）。

### 直接调用可执行文件

`gowork`（Windows 上是 `gowork.exe`）可以单独跑，只是不会切换目录，调试时有用：

```powershell
gowork.exe --workspace C:\Projects --project my-app --no-color
```

```sh
./bin/gowork --workspace ~/Projects --project my-app --no-color
```

`-N` 索引三种入口都认：裸写的 `-N` 会先被改写成 `--project=-N`，所以 `gowork -1` 直接跑也行
（否则 Go 的 flag 包会把它当未知选项）。
`--no-color` 只关闭颜色；清屏和窗口标题由「stdout 是否是真实终端」决定，与它无关。
`--cwd-file <路径>` 把目标目录写进文件——启动器就是靠它回传的。

## 目录结构

| 文件 | 职责 |
| --- | --- |
| `main.go` | 入口、参数解析、编排 |
| `config.go` | 工作区解析与环境变量 / 配置文件读取 |
| `projects.go` | 扫描工作区、`-N` 索引、前缀匹配、git 状态 |
| `render.go` | 仪表盘渲染、ANSI 配色、按显示宽度对齐 |
| `holiday.go` | 节假日查询与磁盘缓存 |
| `clone.go` | 仓库地址识别、克隆 |
| `sysinfo.go` | 跨平台部分：终端宽度、错误输出 |
| `sysinfo_windows.go` | Windows：控制台 VT/UTF-8 模式、卷可用空间 |
| `sysinfo_posix.go` | macOS / Linux：`statfs` 可用空间；VT 无需开启 |
| `bin/work.ps1` | PowerShell 入口：读取目标目录并 `Set-Location` |
| `bin/work.sh` | bash / zsh 入口：source 后在当前会话 `cd` |
| `bin/work.fish` | fish 入口，同上 |

## 开发

```sh
gofmt -l .      # 应为空
go vet ./...    # 静态检查
go test ./...   # 单元测试（macOS / Linux 上可直接跑）

# 交叉编译
GOOS=windows GOARCH=amd64 go build -o bin/gowork.exe .
GOOS=darwin  GOARCH=arm64 go build -o bin/gowork-darwin-arm64 .
```

平台相关的只有 `enableVT` 与 `diskFreeGB` 两个函数，用构建标签隔开；其余逻辑与平台无关，
所以在 macOS / Linux 上 `go test` 就能覆盖目标解析、前缀匹配、`-N` 索引、宽度截断、
节假日文案等纯逻辑，不必等到 Windows 上才能验。

## 为什么需要一个 shell 启动器

子进程改不了父 shell 的工作目录，所以 gowork 把目标目录写进 `--cwd-file` 指定的临时文件，
由启动器读取后切换目录。启动器是架构上无法省略的一步，但它足够薄：不含任何业务逻辑，
删掉它工具本身依然能独立运行（只是不会切目录）。

三个平台各一份，做的事完全一样：

- `bin/work.ps1`：用 `Start-Process -NoNewWindow` 让 `gowork.exe` 直接接管当前控制台。
  这样绕开了 PowerShell 的输出管线，ANSI 颜色不会被 `$PSStyle.OutputRendering` 抹掉；
  输出直接落在主机上、不进管道，所以 `$x = work` 捕获不到仪表盘。
- `bin/work.sh`（bash / zsh）与 `bin/work.fish`：写成**函数**而不是脚本——脚本是子进程，
  照样改不了父 shell 的目录。gowork 是直接调用的（不接管道、不做命令替换），
  它的 stdout 才是真终端，清屏、窗口标题和颜色才会生效。

## 已知问题

- **平台**：Windows、macOS、Linux（含 WSL）。平台相关的只有两个函数，用构建标签分开：
  `sysinfo_windows.go` 读控制台模式与卷空间，`sysinfo_posix.go` 用 `statfs` 取文件系统可用空间。
- **POSIX 清屏**：清屏会附带 `\x1b[3J` 清回滚缓冲，少数老终端不认这一条，忽略即可，不影响布局。
- **字符宽度**：`render.go` 的 `runeWidth` 是 UAX #11 East Asian Width 的近似。零宽字符交给标准库的
  真实 Unicode 类别判断，宽字符区段手工列举，歧义宽度一律按 1 列——它本就取决于终端配置。
  只服务行标签补白与「最近活跃」名字列两处对齐。
- **节假日数据源**：`date.appworlds.cn`。接口语义是「严格晚于给定日期」的下一个放假日，因此用昨天
  去查，否则正好放假那天会算成「还有 1 天」。数据取得失败时只显示「数据获取失败」，不缓存以便重试。
  新一年的排期通常在 11—12 月公布，在那之前跨年的查询会返回空。
- **缓存**：用户缓存目录下的 `gowork/holiday-cache.json`（Windows 是 `%LOCALAPPDATA%\gowork\`，
  macOS 是 `~/Library/Caches/gowork/`，Linux/WSL 是 `~/.cache/gowork/`），缓存的节日日期还没过去就复用。
