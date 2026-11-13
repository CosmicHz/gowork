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
shell 集成由 `gowork init <shell>` 生成，覆盖 PowerShell、bash / POSIX sh、fish，
以及 zsh（有自动补全支持）。从源码构建需要 Go 1.26.5 或更新。

## 安装

### 一键安装

Windows（PowerShell 7）：

```powershell
irm https://raw.githubusercontent.com/CosmicHz/gowork/main/install.ps1 | iex
```

macOS / Linux：

```sh
curl -fsSL https://raw.githubusercontent.com/CosmicHz/gowork/main/install.sh | sh
```

脚本会：

1. 下载**最新正式 release** 的预编译二进制，并用 release 里的 `checksums.txt` 校验 sha256；
2. 装进用户目录；
3. 把该目录加入用户 PATH，并把 `work` 集成写进当前 shell 的配置。

重复执行会覆盖升级，已配置过的 PATH 与 shell 集成不会重复追加。如果已有配置命令里的
shell 集成命令与当前 shell 不匹配（例如 zsh 里还是 `init sh`），脚本会询问是否换成匹配当前
shell 的形式（默认换；没有可交互终端时也按默认处理）。如果配置里有多个 gowork 配置命令，
默认只处理最后一个（因为只有最后一个生效）。先问是否去重（只删前面的，不动最后一个），
再问是否把最后一个换成匹配当前 shell 的形式。

#### 带参数运行

需要指定安装目录、安装预发布版本、只装二进制或从源码构建时，先把脚本下载下来再执行：

Windows：

```powershell
irm https://raw.githubusercontent.com/CosmicHz/gowork/main/install.ps1 -OutFile install.ps1
.\install.ps1 -PreRelease
Remove-Item .\install.ps1
```

macOS / Linux：

```sh
curl -fsSL https://raw.githubusercontent.com/CosmicHz/gowork/main/install.sh -o install.sh
sh install.sh --pre-release
rm install.sh
```

| 目的 | Windows | macOS / Linux |
| --- | --- | --- |
| 安装到指定目录 | `-BinDir <路径>` | `--bin-dir <路径>` |
| 安装最新预发布版本 | `-PreRelease` | `--pre-release` |
| 只装二进制，不改环境 | `-NoModify` | `--no-modify` |
| 从源码构建 | `-FromSource` | `--from-source` |

不指定 `-BinDir` / `--bin-dir` 时，默认安装到 `%LOCALAPPDATA%\gowork\bin`（Windows）与
`~/.local/bin`（macOS / Linux）。自动修改的位置：

| 环境 | 修改 |
| --- | --- |
| zsh | `~/.zshrc` |
| bash | `~/.bashrc`（macOS 为 `~/.bash_profile`） |
| fish | `~/.config/fish/config.fish` |
| Windows | 用户 PATH + `$PROFILE` |

追加的内容包在 `# >>> gowork installer >>>` 标记里，想撤销时删掉整块即可。
预编译产物覆盖 Windows / macOS / Linux 的 amd64 与 arm64；其他平台或想从源码构建，
见[从源码构建](#从源码构建两种方式)。

### 手动安装（下载 release）

在 [Releases](https://github.com/CosmicHz/gowork/releases) 页下载对应系统的产物：

- macOS / Linux：`gowork_<版本>_<darwin|linux>_<amd64|arm64>.tar.gz`
- Windows：`gowork_<版本>_windows_<amd64|arm64>.zip`

同页的 `checksums.txt` 可以校验下载（可选），把 `0.1.0` 换成实际版本：

Windows：

```powershell
$ver = '0.1.0'
Get-FileHash ".\gowork_${ver}_windows_amd64.zip" -Algorithm SHA256
```

macOS：

```sh
ver="0.1.0"
shasum -a 256 "gowork_${ver}_darwin_arm64.tar.gz"   # Intel 用 darwin_amd64
```

Linux：

```sh
ver="0.1.0"
sha256sum "gowork_${ver}_linux_amd64.tar.gz"
```

把输出的哈希与 `checksums.txt` 里对应文件的 sha256 比对即可。

解压后把二进制放到用户目录：

Windows：

```powershell
$ver = '0.1.0'
Expand-Archive ".\gowork_${ver}_windows_amd64.zip" -DestinationPath .
New-Item -ItemType Directory -Force -Path "$env:LOCALAPPDATA\gowork\bin" | Out-Null
Copy-Item ".\gowork_${ver}_windows_amd64\gowork.exe" "$env:LOCALAPPDATA\gowork\bin\gowork.exe"
```

macOS：

```sh
ver="0.1.0"
tar -xzf "gowork_${ver}_darwin_arm64.tar.gz"   # Intel 用 darwin_amd64
mkdir -p ~/.local/bin
cp "gowork_${ver}_darwin_arm64/gowork" ~/.local/bin/gowork
chmod +x ~/.local/bin/gowork
```

Linux：

```sh
ver="0.1.0"
tar -xzf "gowork_${ver}_linux_amd64.tar.gz"
mkdir -p ~/.local/bin
cp "gowork_${ver}_linux_amd64/gowork" ~/.local/bin/gowork
chmod +x ~/.local/bin/gowork
```

然后手动把目录加入 PATH、写入 shell 集成（下面以 PowerShell、zsh 与 bash 为例）：

```powershell
[Environment]::SetEnvironmentVariable('Path',
  [Environment]::GetEnvironmentVariable('Path','User') + ';' + "$env:LOCALAPPDATA\gowork\bin", 'User')

# 在 $PROFILE 末尾加：
Invoke-Expression (& { (gowork init powershell | Out-String) })
```

```sh
# zsh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
echo 'eval "$(gowork init zsh)"' >> ~/.zshrc
```

```sh
# bash / POSIX sh（macOS 的 bash 用 ~/.bash_profile）
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
echo 'eval "$(gowork init sh)"' >> ~/.bashrc
```

fish 用户改用 `fish_add_path ~/.local/bin` 和 `gowork init fish | source`。

### 从源码构建（两种方式）

需要 Go 1.26.5 或更新。

#### 方式一：手动构建

Windows：

```powershell
git clone https://github.com/CosmicHz/gowork.git
cd gowork
New-Item -ItemType Directory -Force -Path bin | Out-Null
go build -o bin\gowork.exe .\cmd\gowork

# 把 bin 目录加入用户 PATH
[Environment]::SetEnvironmentVariable('Path',
  [Environment]::GetEnvironmentVariable('Path','User') + ';' + (Join-Path $PWD 'bin'), 'User')

# 在 $PROFILE 末尾加：
Invoke-Expression (& { (gowork init powershell | Out-String) })
```

macOS / Linux（以 zsh 为例）：

```sh
git clone https://github.com/CosmicHz/gowork.git
cd gowork
mkdir -p ~/.local/bin
go build -o ~/.local/bin/gowork ./cmd/gowork

echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
echo 'eval "$(gowork init zsh)"' >> ~/.zshrc
```

#### 方式二：用安装脚本构建

`-FromSource` / `--from-source` 不会下载 release：在仓库内用当前代码构建，否则浅克隆仓库；
仍然会自动配置 PATH 与 shell 集成（`-NoModify` / `--no-modify` 可跳过）。

```powershell
irm https://raw.githubusercontent.com/CosmicHz/gowork/main/install.ps1 -OutFile install.ps1
.\install.ps1 -FromSource
Remove-Item .\install.ps1
```

```sh
curl -fsSL https://raw.githubusercontent.com/CosmicHz/gowork/main/install.sh -o install.sh
sh install.sh --from-source
rm install.sh
```

`gowork init <shell>` 输出的函数会在 PATH 上找 `gowork`；不想放进 PATH 时，把
`GOWORK_BIN` 设成可执行文件的绝对路径即可。可填入的 shell 名称有 `powershell`、
`zsh`、`sh`（bash / POSIX，zsh 也可用）与 `fish`。

## 配置

### 工作区

为了知道该去哪里找你的项目，gowork 按下面的优先级解析：

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

### 配置项

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `workspace` | 无 | 工作区根目录；命令行与环境变量优先 |
| `indent` | `3` | 左侧缩进空格数，最小 3；`--indent` 优先 |
| `no_color` | `false` | 禁用颜色；`--no-color` 优先 |
| `recent_limit` | `4` | 「最近活跃」列出的项目数，最小 1 |
| `recent_name_width` | `35` | 「最近活跃」项目名截断宽度（显示列），最小 5 |
| `disk_warn_gb` | `10` | 可用空间低于该值时标红；`0` 表示不标红 |
| `show_disk` | `true` | 是否显示「磁盘」行 |
| `show_git` | `true` | 是否显示「Git」行 |
| `show_holiday` | `true` | 是否显示「节日」行 |
| `show_recent` | `true` | 是否显示「最近活跃」 |
| `clear_screen` | `true` | 是否清屏（仅真实终端） |
| `quotes` | 内置格言 | 底部格言的候选数组，随机取一条 |

```json
{
  "workspace": "C:\\Projects",
  "indent": 4,
  "recent_limit": 6,
  "disk_warn_gb": 20,
  "show_git": false,
  "clear_screen": false,
  "quotes": ["保持简单。", "先跑起来，再变好。"]
}
```

配置文件损坏（JSON 语法错误或数值越界）会直接报错，不会静默使用部分配置。

## 用法

### 命令一览

| 命令 | 行为 |
| --- | --- |
| `work` | 回到工作区根目录 |
| `work my-app` | 跳转到 `my-app` |
| `work my-a` | 前缀匹配，唯一命中就跳过去 （[详细说明](#项目名匹配)） |
| `work my` | 多个命中时跳最短匹配，并在仪表盘下方列出全部候选 （[详细说明](#项目名匹配)） |
| `work -1` | 按最近修改排序，跳第 N 个 |
| `work <仓库地址>` | 克隆到工作区并进入 |
| `work -IndentSize 5 my-app` | 左侧缩进改为 5（默认 3，最小 3） |
| `work -Workspace D:\code my-app` | 本次改用它作为工作区 |

### 选项一览

目标选择器只能三选一：**位置项目名**、**`--project`**、**`--recent` / `-<N>`**。

#### 目标选择器

| 写法 | Windows PowerShell | 说明 | 范例 |
| --- | --- | --- | --- |
| `work <项目名>` | 不变 | 完整项目名、唯一前缀或仓库地址 | `work my-project` |
| `work -p <目标>` / `work --project <目标>` | `-Project <目标>`（`-Proj` / `-P`） | 与位置项目名互斥 | `work --project my-pro` / `work --project -1` |
| `work -<N>` / `work --recent <N>` | 不变 | 最近修改的第 N 个（1 起）；与位置项目名、`--project` 互斥 | `work -1` / `work --recent 1` |

`--project -<N>` （或 `--project=-<N>`）等价于 `--recent <N>` 和 `-<N>`

目标选择器之间是互斥的，同时传入多个选择器（即便彼此等价）会报错。

#### 其余选项

| 短选项 | 长选项 | 取值 | Windows PowerShell | 说明 |
| --- | --- | --- | --- | --- |
| `-i` | `--indent` | `<N>` | `-IndentSize`（`-Ind` / `-I`） | 左侧缩进空格数，最小 3 |
| `-w` | `--workspace` | `<目录>` | `-Workspace`（`-Work` / `-W`） | 本次使用的工作区 |
| `-n` | `--no-color` | 无 | 透传 `--no-color` / `-n` | 禁用 ANSI 颜色 |
| `-h` | `--help` | 无 | 透传 `--help` / `-h` | 输出帮助 |
| `-c` | `--cwd-file` | `<文件>` | 透传 | 成功后写入目标目录；供 shell 集成内部使用 |

Windows 的 `work` 同时接受上表列出的 PowerShell 风格和 getopt 风格选项；macOS / Linux 不支持 PowerShell 风格选项。

带参数的长选项同时接受 `--opt value` 和 `--opt=value`；布尔选项写 `--opt` 或
`--opt=<true|false>`，不要写 `--no-color false` 这样的选项。长选项必须用双横线；
单横线只表示短选项（`-n`、`-p foo`、`-pfoo`、`-ni5`）。

短选项可以组合：`-ni 5` 等价于 `--no-color --indent 5`，`-ni5` 等价于
`--no-color --indent=5`。带值选项后面的剩余字符会整体作为它的值（如 `-p-1` 等价于 `--project=-1`）。

选项可以写在项目名之前或之后：`work my-app -n` 与 `work -n my-app` 等价。

`--` 之后的参数不再按选项解析，而是当作项目名：`work -- --no-color` 会去找名为
`--no-color` 的项目，不会关闭颜色；项目名只能写一个，多写会报错。

> [!important]
> 在 PowerShell 中， `--` 无效。
> 例如进入名为 `-1` 的目录可以写 `work ./-1` ，不能写 `work -- -1` 。
> 未来的版本可能修复这个问题。

### 项目名匹配

假设工作区为 `C:\Projects`，含 `my-app`、`my-service` 等项目。
执行`work <名称>`时，程序按顺序尝试：

1. **完整名**：工作区文件夹内，`<名称>` 存在且是目录 → 直接跳转。
   如 `work my-app` 会跳转到 `C:\Projects\my-app`。
2. **前缀匹配**（大小写不敏感）：取所有以名称开头的项目。
   - 命中 1 个 → 跳转到它。
     如 `work my-a` 会跳转到 `C:\Projects\my-app`。
   - 命中多个 → 跳转到**名字最短**的那个；长度并列时取字典序靠前的目录名。
     与此同时在仪表盘下方列出全部匹配到的目录名。
     如 `work my` 会跳转到 `C:\Projects\my-app`，并在仪表盘末尾显示

     ```text
       ⚠ 前缀 'my' 匹配到多个项目，跳转到最短匹配：
           my-app
           my-service
     ```

   - 命中 0 个 → 报错，如 `目录不存在：C:\Projects\my-project`。

3. 解析失败则**不清屏、不切换目录**，退出码 1。

工作区里以 `.` 开头的目录会被跳过；指向目录的符号链接算项目。

### `-<N>` 索引

`-1` 是**最近修改**的项目，`-2` 次之，按子目录的 LastWriteTime 倒序。越界时报错并提示有效范围。
等价的显式写法是 `--recent N`（或 `--recent=N`），例如 `work --recent 1`。

### `-<N>` 与目录同名

工作区里若有目录叫 `-1`，或有目录以 `-1` 开头（如 `-1-app`），`work -1` 仍按索引解析，不会按名字匹配或进入这些目录。
要进入同名目录请写 `work ./-1`（macOS / Linux 也可用 `work -- -1`）；
要按名字进入请写完整名，如 `work ./-1-app`。

如果的确出现这种情况，仪表盘下方会给出提示。

### `init` 与目录同名

`gowork init <shell>` 是输出 shell 集成代码的子命令，但 shell 函数总把 `--cwd-file` 放在
最前面，所以 `work init` 里的 `init` 是项目名。工作区里若真有名为 `init` 的目录，
`work init` 会进入它，并在仪表盘下方提示两者的区别；想完全避免歧义可以写 `work ./init`。

### 选项名与目录同名

工作区里若有目录叫 `-n`、`-p`、`--no-color` 等，而本次命令又确实把这些写法解析成了选项，
仪表盘下方会提示当前按选项处理，并给出消歧写法：

```text
   ⚠ "-n" 同时是工作区里的目录名与命令行选项；当前按选项解析
      要进入该目录请写 work ./-n
```

`--cwd-file` 由 shell 自动注入，`-h` / `--help` 会在打印帮助后直接退出，所以这两类不参与提示。`--` 之后的参数也不参与。

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
| 磁盘 | 目标所在卷对**当前用户**可用的空间；低于 `disk_warn_gb`（配置项，默认 10 GB）显示为红色 | 整行不显示 |
| Git | 当前分支（空则显示 `detached`）· 改动数量（`git status --porcelain` 的非空行数） | 非仓库或无 git → 整行不显示 |
| 节日 | 见下 | 显示 `数据获取失败⚠️` |
| 最近活跃 | 按修改时间倒序前 `recent_limit`（配置项，默认 4） 个；名字按显示宽度截到 `recent_name_width`（配置项，默认 35，超出则留 3 列给 `...`） 列，后接 `MM-dd HH:mm` | 没有项目 → 整段不显示 |

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

`-<N>` 索引三种入口都认：裸写的 `-<N>` 会先被改写成 `--recent=N`，所以直接跑 `-1` 也行
（否则 pflag 会把它当未知 shorthand）；也可以显式写 `--recent 1`。
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
│   ├── cli/             参数解析、编排与退出码
│   ├── config/          工作区解析、配置文件与缓存目录
│   ├── project/         扫描工作区、最近项目索引、前缀匹配、git 状态
│   ├── render/          仪表盘渲染、宽度对齐、克隆提示
│   ├── holiday/         节假日查询、缓存与「节日」行文案
│   ├── clone/           仓库地址识别、仓库名推断、git clone
│   ├── shell/           gowork init 的 shell 集成模板（sh / fish / powershell / zsh）
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
所以在 macOS / Linux 上 `go test` 就能覆盖目标解析、前缀匹配、`-<N>` 索引、宽度截断、
节假日文案等纯逻辑，不必等到 Windows 上才能验。

CI 在 Windows、macOS、Linux 三个平台各跑一遍 `gofmt`、`go vet` 与 `go test`（用 gotestsum
输出，摘要里会列出跳过数量，配置见 [.github/workflows/ci.yml](.github/workflows/ci.yml)）。测试会用 bash、zsh、dash、
fish 与 PowerShell 分别解析生成的集成代码，Windows 上还会额外用 PowerShell 5.1；
某个 shell 没装就跳过。CI 会在跑测试前逐个确认这些解释器都在，装失败直接报错，
避免静默失去覆盖。

## 为什么需要 shell 集成

gowork 作为子进程改不了父 shell 的工作目录，所以需要一个 shell 函数帮助其做到这一点。
gowork 会把目标目录写进 `--cwd-file` 指定的临时文件，由 shell 侧读取后切换目录。
这一层无法省略，但不含任何业务逻辑，删掉它工具本身依然能独立运行（只是不会切目录）。

集成代码由 `gowork init <shell>` 生成，不需要在仓库里维护启动器文件：

- `powershell`：函数用 `Start-Process -NoNewWindow` 让 `gowork.exe` 直接接管当前控制台。
  这样绕开了 PowerShell 的输出管线，ANSI 颜色不会被 `$PSStyle.OutputRendering` 抹掉；
  输出直接落在主机上、不进管道，所以 `$x = work` 捕获不到仪表盘。
  生成的代码保持纯 ASCII：pwsh 会按控制台代码页解码 `gowork init powershell` 的管道输出，
  中文在 GBK 环境下会被解坏并吞掉换行。
- `sh` 与 `fish`：gowork 是直接调用的（不接管道、不做命令替换），这样一来它的 stdout 才是真终端，因此清屏、窗口标题和颜色才会生效。
- `zsh`：在 sh 函数体之后追加补全脚本。`_gowork` 通过 `gowork complete projects`
  动态取项目名（最近修改的在前），并描述全部选项；`-p` 补项目、`-w` 补目录、
  `-i` / `--recent` 补数字、裸 `-N` 补 `-1`…`-N`。补全依赖 zsh 的 `compinit`，
  没加载时静默跳过。oh-my-zsh / prezto 等框架通常已经加载它；手动配置的 zsh 需要在
  `eval "$(gowork init zsh)"` 之前有一句 `autoload -Uz compinit && compinit`。

## 已知限制

- **字符宽度**：`internal/render` 的 `runeWidth` 是 UAX #11 East Asian Width 的近似。零宽字符交给标准库的真实 Unicode 类别判断，宽字符区段手工列举，歧义宽度一律按 1 列——它本就取决于终端配置。只服务行标签补白与「最近活跃」名字列两处对齐。
- **节假日数据源**：`uapis.cn` 的[万年历接口](https://uapis.cn/docs/api-reference/get-misc-holiday-calendar)。
  按天查询一次就能拿到当天与前后一段时间的节日，整理后写入缓存，避免频繁调用有频率限制的接口；
  缓存里的节日全部过去后才重新查询。
- **法定排期未公布**：新一年的放假安排公布前，接口只提供公历/农历节日，没有放假天数。
  此时只显示最近的节日兜底，不按往年规律预测；要补全得自己推算，目前没做。
- **PowerShell 的 `--`**：PowerShell 脚本会在解析参数时丢掉 `--`，所以终止符与
  `work -- -1` 只在 macOS / Linux 中可用；pwsh 请用 `work ./-1`。
