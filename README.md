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
   ───────────────────────────────────────────────────────
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

   ───────────────────────────────────────────────────────
     代码是写给人看的，顺便让机器运行。
```

支持 Windows、macOS 与 Linux（含 WSL）。Windows 用 PowerShell 7；
shell 集成由 `gowork init <shell>` 生成，覆盖 PowerShell、bash / zsh / POSIX sh 与 fish。
从源码构建需要 Go 1.26.5 或更新。

## 安装

### 一键安装

脚本会构建二进制、放进默认目录，然后打印需要添加的那一行；它们不改 PATH，也不改 shell 配置。

Windows（PowerShell 7）：

```powershell
irm https://raw.githubusercontent.com/CosmicHz/gowork/main/install.ps1 | iex
```

macOS / Linux：

```sh
curl -fsSL https://raw.githubusercontent.com/CosmicHz/gowork/main/install.sh | sh
```

默认安装到 `%LOCALAPPDATA%\gowork\bin`（Windows）与 `~/.local/bin`（macOS / Linux），
可以用 `-BinDir` / `--bin-dir` 改。需要 Go 1.26.5 或更新。

### 手动安装

Windows（PowerShell 7）:

```powershell
# 0. 克隆
git clone https://github.com/CosmicHz/gowork.git
cd gowork

# 1. 构建（产物落在 bin\gowork.exe）
go build -o bin\gowork.exe .\cmd\gowork

# 2. 把 bin 目录加进用户级 PATH
[Environment]::SetEnvironmentVariable('Path',
  [Environment]::GetEnvironmentVariable('Path','User') + ';' + (Join-Path $PWD 'bin'), 'User')

# 3. 配置工作区（三选一，见下节）
[Environment]::SetEnvironmentVariable('GOWORK_WORKSPACE', 'C:\Projects', 'User')

# 4. 把这一行加到 $PROFILE 末尾（echo $profile 可查看路径）：
Invoke-Expression (& { (gowork init powershell | Out-String) })

# 5. 重启终端后
work
```

macOS / Linux :

```sh
# 0. 克隆
git clone https://github.com/CosmicHz/gowork.git
cd gowork

# 1. 直接构建到 PATH 上的目录
mkdir -p ~/.local/bin
go build -o ~/.local/bin/gowork ./cmd/gowork

# 2. 把 PATH 与 shell 集成写进配置。下面以 zsh 为例，bash 换成 ~/.bashrc
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
echo 'eval "$(gowork init sh)"' >> ~/.zshrc

# fish 用户改用：
#   fish_add_path ~/.local/bin
#   echo 'gowork init fish | source' >> ~/.config/fish/config.fish

# 3. 配置工作区（三选一，见下节）
echo 'export GOWORK_WORKSPACE="$HOME/Projects"' >> ~/.zshrc

# 4. 重开终端后
work
```

`gowork init <shell>` 输出的函数会在 PATH 上找 `gowork`；不想放进 PATH 时，把
`GOWORK_BIN` 设成可执行文件的绝对路径即可。可用的 shell 是 `powershell`、`sh`
（bash / zsh / POSIX 通用）与 `fish`。

## 配置工作区

gowork 不知道该去哪里找你的项目，按下面的优先级解析：

| 优先级 | 方式 |
| --- | --- |
| 1 | 临时指定：`work -Workspace <路径>`（Windows）/ `work --workspace <路径>`（macOS / Linux） |
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

> 选项拼写随平台而变：Windows 的 shell 函数写 `-IndentSize` / `-Workspace`；
> macOS / Linux 的函数把参数原样透传给 gowork，写 `--indent` / `--workspace`。

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

`-1` 是**最近修改**的项目，`-2` 次之，按子目录的 LastWriteTime 倒序。越界时报错并提示有效范围。

> [!NOTE]
>
> 工作区里若有目录真的叫 `-1`，`-N` 语法优先：`work -1` 仍按索引解析，不会进入它。
> 要进入它请写 `work ./-1`。克隆一个名为 `-1` 的仓库也会落进这样的目录。

### 仓库地址

判定范围与 git 自己一致：

- **带协议**：`http` `https` `ssh` `git` `ftp` `ftps`，git 的 ssh 别名
  `git+ssh://`、`ssh+git://`，以及 `file://`。
- **scp 形式**：`[user@]host:path`，用户名可省略，如 `github.com:owner/repo.git`。
  项目名本身含冒号时（`name:tag`），写 `work ./name:tag` 可避免被当成地址。

**不认**裸写域名，如 `github.com/owner/repo`——git 自己也把它当本地路径。
同理，单字母主机（`C:\x`、`C:/x`）按盘符处理，不会被当成地址。
这两类都会当作项目名去匹配，匹配不到就报「目录不存在」。

识别为地址后，按下面的规则落到本地：

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
| Git | 当前分支（空则显示 `detached`）· 改动数量（`git status --porcelain` 的非空行数） | 非仓库或无 git → 整行不显示 |
| 节日 | 见下 | 显示 `数据获取失败⚠️` |
| 最近活跃 | 按修改时间倒序前 4 个；名字按显示宽度截到 35 列（超出则留 32 列加 `...`），后接 `MM-dd HH:mm` | 没有项目 → 整段不显示 |

标题与问候语：

- 窗口标题设为 `WORK | <项目名>`。
- 进入具体项目时，logo 倒数第二行末尾补 `on <项目名>`（亮白色）；回工作区根目录时不加。
  放不下时（缩进 + logo 35 列 + 2 空格 + 后缀 > 终端宽度）改为另起一行。
- 问候语按小时分段：0-6 点「夜深了」、6-12 点「早上好」、12-14 点「中午好」、
  14-18 点「下午好」、18-24 点「晚上好」。
- 底部随机一条格言。

### 节假日

- 数据源是 `uapis.cn` 的[万年历接口](https://uapis.cn/docs/api-reference/get-misc-holiday-calendar)，按天查询。
  一次请求就能拿到当天与前后一段时间的节日，正常运行时每次启动最多查询一次。
- 优先显示下一个**放假日**：今天放假就直接显示今天，否则显示最近的法定放假日。
  放假天数按整段假期计算，所以假期中途看到的也是完整天数。节气不算节日，调休上班也不算放假。
- 法定放假安排还没公布时，显示最近的公历或农历节日，不显示放假天数。
- 文案分档：`就是今天！快乐！` / `就在明天！` / `还有 N 天，倒计时开始！`（N ≤ 3）/ `还有 N 天`；
  统计到放假天数时追加 `(放假 N 天)`。
- 获取失败时显示 `数据获取失败⚠️`，并且**不写缓存**，下次启动仍会重试。
- 缓存文件在用户缓存目录：Windows `%LOCALAPPDATA%\gowork\holiday-cache.json`、
  macOS `~/Library/Caches/gowork/holiday-cache.json`、Linux `~/.cache/gowork/holiday-cache.json`。
  缓存里是整理后的节日列表（日期、名称、类型）；只要其中还有未来的节日就直接复用，
  全部过去后才重新查询——接口有调用频率限制，不宜每次都请求。

### 退出码与失败行为

| 码 | 含义 |
| --- | --- |
| 0 | 成功（并写入目标目录，shell 函数据此 `Set-Location` / `cd`） |
| 1 | 业务错误：目录不存在、索引越界、克隆失败、未配置工作区、推断不出仓库名 |
| 2 | 参数错误，例如 `--indent` 小于 3、`init` 指定了不支持的 shell。三种 shell 一致：PowerShell 的 `-IndentSize` 只负责整数类型转换，范围由 gowork 判定 |

失败时统一**不切换目录**，错误以红字写到 stderr。解析阶段的失败不清屏（保留现场），
克隆失败时屏幕已清（因为要显示克隆过程）。

### 直接调用可执行文件

构建出来的可执行文件（Windows 是 `bin\gowork.exe`，macOS / Linux 是 `bin/gowork`）可以单独跑，
只是不会切换目录，调试时有用：

```powershell
.\bin\gowork.exe --workspace C:\Projects --project my-app --no-color
```

```sh
./bin/gowork --workspace ~/Projects --project my-app --no-color
```

`-N` 索引三种入口都认：裸写的 `-N` 会先被改写成 `--project=-N`，所以直接跑 `-1` 也行
（否则 Go 的 flag 包会把它当未知选项）。
`--no-color` 只关闭颜色；清屏和窗口标题由「stdout 是否是真实终端」决定，与它无关。
`--cwd-file <路径>` 把目标目录写进文件。（shell 函数就是靠它回传的）

`gowork init <shell>` 把 shell 集成代码打印到 stdout，可以直接查看，也可以像安装步骤那样
接进 shell 配置：

```sh
./bin/gowork init sh
```

## 目录结构

```text
gowork/
├── cmd/gowork/          入口，只把 os.Args 交给 internal/cli
├── internal/
│   ├── ansi/            ANSI 前景色常量与主色调色板
│   ├── cli/             参数解析、裸 -N 改写、编排与退出码
│   ├── config/          工作区解析、配置文件与缓存目录
│   ├── project/         扫描工作区、-N 索引、前缀匹配、git 状态
│   ├── render/          仪表盘渲染、宽度对齐、克隆提示
│   ├── holiday/         节假日查询、缓存与「节日」行文案
│   ├── clone/           仓库地址识别、仓库名推断、git clone
│   ├── shell/           gowork init 的三份 shell 集成模板
│   └── sysinfo/         终端宽度、颜色开关、失败输出、VT 与磁盘空间
├── install.sh           macOS / Linux 安装脚本：构建并放到 PATH 目录
├── install.ps1          Windows 安装脚本，同上
└── .github/workflows/   CI 与发布
```

平台相关的只有 `sysinfo` 里的 `EnableVT` 与 `DiskFreeGB`，用构建标签隔开；
其余包与平台无关，可在任意平台 `go test`。

## 开发

```sh
gofmt -l .      # 应为空
go vet ./...    # 静态检查
go test ./...   # 单元测试（macOS / Linux 上可直接跑）

# 交叉编译
GOOS=windows GOARCH=amd64 go build -o bin/gowork.exe ./cmd/gowork
GOOS=darwin  GOARCH=arm64 go build -o bin/gowork-darwin-arm64 ./cmd/gowork
```

平台相关的只有 `sysinfo` 里的 `EnableVT` 与 `DiskFreeGB`，用构建标签隔开；其余包与平台无关，
所以在 macOS / Linux 上 `go test` 就能覆盖目标解析、前缀匹配、`-N` 索引、宽度截断、
节假日文案等纯逻辑，不必等到 Windows 上才能验。

CI 在 Windows、macOS、Linux 三个平台各跑一遍 `gofmt`、`go vet` 与 `go test`（用 gotestsum
输出，摘要里会列出跳过数量，配置见 [.github/workflows/ci.yml](.github/workflows/ci.yml)）。测试会用 bash、zsh、dash、
fish 与 PowerShell 分别解析生成的集成代码，Windows 上还会额外用 PowerShell 5.1；
某个 shell 没装就跳过。CI 会在跑测试前逐个确认这些解释器都在，装失败直接报错，
避免静默失去覆盖。

## 为什么需要 shell 集成

子进程改不了父 shell 的工作目录，所以 gowork 把目标目录写进 `--cwd-file` 指定的临时文件，
由 shell 侧读取后切换目录。这一层无法省略，但它足够薄：不含任何业务逻辑，
删掉它工具本身依然能独立运行（只是不会切目录）。

集成代码由 `gowork init <shell>` 生成，不需要在仓库里维护启动器文件：

- `powershell`：函数用 `Start-Process -NoNewWindow` 让 `gowork.exe` 直接接管当前控制台。
  这样绕开了 PowerShell 的输出管线，ANSI 颜色不会被 `$PSStyle.OutputRendering` 抹掉；
  输出直接落在主机上、不进管道，所以 `$x = work` 捕获不到仪表盘。
  生成的代码保持纯 ASCII：pwsh 会按控制台代码页解码 `gowork init powershell` 的管道输出，
  中文在 GBK 环境下会被解坏并吞掉换行。
- `sh` 与 `fish`：生成一个**函数**，以便修改父 shell 的目录。
  gowork 是直接调用的（不接管道、不做命令替换），这样一来它的 stdout 才是真终端，
  因此清屏、窗口标题和颜色才会生效。

## 已知限制

- **字符宽度**：`internal/render` 的 `runeWidth` 是 UAX #11 East Asian Width 的近似。零宽字符交给标准库的
  真实 Unicode 类别判断，宽字符区段手工列举，歧义宽度一律按 1 列——它本就取决于终端配置。
  只服务行标签补白与「最近活跃」名字列两处对齐。
- **节假日数据源**：`uapis.cn` 的[万年历接口](https://uapis.cn/docs/api-reference/get-misc-holiday-calendar)。
  按天查询一次就能拿到当天与前后一段时间的节日，整理后写入缓存，避免频繁调用有频率限制的接口；
  缓存里的节日全部过去后才重新查询。
- **法定排期未公布**：新一年的放假安排公布前，接口只提供公历/农历节日，没有放假天数。
  此时只显示最近的节日兜底，不按往年规律预测；要补全得自己推算，目前没做。
