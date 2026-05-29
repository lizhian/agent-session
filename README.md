# Agent Session

Claude Code、Codex、OpenCode 与 Pi Coding Agent 的交互式 session 选择器。一个 Go 二进制文件通过不同命令名分发：`c` 打开 Claude Code，`cx` 打开 Codex，`oc` 打开 OpenCode，`p` 打开 Pi Coding Agent。

- `c`：浏览并恢复 Claude Code sessions
- `cx`：浏览并恢复 Codex sessions
- `oc`：浏览并恢复 OpenCode sessions
- `p`：浏览并恢复 Pi Coding Agent sessions

## 功能

- 浏览当前目录下的 Claude Code、Codex、OpenCode、Pi Coding Agent sessions。
- 从 `0. new` 新建 session，或选择已有 session 恢复。
- 按首条/末条 user 消息、路径、时间等信息搜索 sessions。
- 显示短 session ID、相对更新时间、消息数量、首条/末条 user 消息。
- 使用 `Tab` 切换权限模式，并记住上次选择。
- 通过工作区视图切换到其他项目目录。
- 在选择器内配置模型 provider、默认模型和 provider 相关模型列表。
- 从 session 列表按 `Left` 进入全局 manager，管理 skills 安装源、全局启用和当前项目启用。
- 单二进制 + 命令分发：`c`/`cx`/`oc`/`p` -> `agent-session`。
- 零运行时依赖：运行时不需要 Node.js、sqlite3 或 Go。

## 安装

macOS/Linux：

```bash
curl -fsSL https://raw.githubusercontent.com/lizhian/agent-session/main/install.sh | sh
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/lizhian/agent-session/main/install.ps1 | iex
```

安装脚本会从 GitHub Releases 下载预编译二进制，创建 `c`、`cx`、`oc`、`p` 命令，并把安装目录加入 PATH。脚本会检查本机是否已安装对应 agent CLI；如果缺少 `claude`、`codex`、`opencode` 或 `pi`，安装时会给出提示，实际运行对应命令前仍需要先安装原始 agent CLI。

安装后重新加载 shell：

```bash
source ~/.zshrc    # 或 source ~/.bashrc
```

Windows 下重启 PowerShell，或执行：

```powershell
. $PROFILE
```

## 使用

```bash
c               # 打开 Claude Code session 选择器
cx              # 打开 Codex session 选择器
oc              # 打开 OpenCode session 选择器
p               # 打开 Pi Coding Agent session 选择器
```

也可以直接通过主二进制分发：

```bash
agent-session c
agent-session cx
agent-session oc
agent-session p
```

`agent-session cc` 仍作为旧版本兼容入口保留，但安装脚本不再创建 `cc` 命令。

选择器编号从 `0` 开始：选择 `0` 新建 session，选择 `1+` 恢复已有 session。

### 交互操作

- 输入文字：搜索当前视图。
- `Up`/`Down` 或 `k`/`j`：移动选择。
- `Enter`：打开选中的 session、工作区或配置项。
- `Space`：在 session 列表中预览 transcript；在多选配置中切换选中状态。
- `Tab`：在 session 列表中切换权限模式。
- `Right` 或 `l`：从 session 列表进入工作区视图；从工作区视图进入配置视图。
- `Left` 或 `h`：从 session 列表进入 manager，或返回上一层。
- `Esc`：返回上一层；在 session 列表中退出。
- `Ctrl-C`：退出。

## Skills manager

在 session 列表中按 `Left` 进入 manager，再进入 `Skills manager`。

- `Install source`：输入 `owner/repo` 形式的 GitHub source，通过 `npx skills add <source>` 安装到 Agent Session 自己的 skills 仓库。
- `Global skills`：按 source 分组显示所有已安装 skills，`Space` 暂存选中，`Enter` 批量保存到 `~/.agents/skills`。
- `Project skills`：按 source 分组显示所有已安装 skills，`Space` 暂存选中，`Enter` 批量保存到当前项目的 `/.agents/skills`。
- 已安装 source 详情：支持镜像更新和删除 source；更新会同步移除 source 中已不存在的 skill，并自动清理对应全局/项目软链。

Agent Session 把 skills 安装仓库保存在 `~/.agent-session/skills/`，并使用 `~/.agent-session/install-skills.json` 作为权威 catalog。启用状态通过软链投影到全局或当前项目目录。

使用 `Skills manager` 需要本机可运行 `npx skills`。

### 权限模式

| 模式 | Claude Code | Codex | OpenCode | Pi Coding Agent |
| --- | --- | --- | --- | --- |
| 默认 | `claude` | `codex` | `opencode` | `pi` |
| 自动 | `claude --enable-auto-mode` | `codex --full-auto` | 不支持 | 不支持 |
| 完全 | `claude --dangerously-skip-permissions` | `codex --dangerously-bypass-approvals-and-sandbox` | `OPENCODE_PERMISSION="allow" opencode` | 不支持 |

Claude Code 和 Codex 支持默认、自动、完全三种模式；OpenCode 目前只支持默认和完全模式；Pi Coding Agent 目前只支持默认模式。

### 命令参数

```bash
c [--cwd <path>] [--claude-home <path>]
cx [--cwd <path>] [--codex-home <path>]
oc [--cwd <path>] [--opencode-data-home <path>]
p [--cwd <path>] [--pi-home <path>] [--pi-session-dir <path>]
```

参数：

- `--cwd <path>`：指定要浏览 sessions 的项目目录，默认是当前目录。
- `--claude-home <path>`：Claude Code 配置目录，默认读取 `CLAUDE_HOME`，否则使用 `~/.claude`。
- `--codex-home <path>`：Codex 配置目录，默认读取 `CODEX_HOME`，否则使用 `~/.codex`。
- `--opencode-data-home <path>`：OpenCode 数据目录，默认读取 `OPENCODE_DATA_HOME`，否则使用 `~/.local/share/opencode`。
- `--pi-home <path>`：Pi Coding Agent 配置目录，默认读取 `PI_CODING_AGENT_DIR`，否则使用 `~/.pi/agent`。
- `--pi-session-dir <path>`：Pi Coding Agent session 目录，默认读取 `PI_CODING_AGENT_SESSION_DIR`，否则使用 `<pi-home>/sessions`。
- `-h`/`--help`：显示帮助。

## 配置视图

在 session 列表中按 `Right` 进入工作区视图，再按 `Right` 进入配置视图。

- Claude Code：切换 model provider，并配置 Opus、Sonnet、Haiku 模型。
- Codex：切换 `config.toml` 中的 model provider；切换后会尝试通过 `codex-threadripper` 同步 threads，如果该命令不存在则跳过同步。
- OpenCode：配置 provider 可用模型、默认模型和 small model。
- Pi Coding Agent：暂不提供配置项。

## 项目结构

```text
cmd/agent-session/main.go     # 二进制入口，按命令名分发到 provider
internal/
  provider/                   # provider 共享的接口
  claude/                     # Claude Code provider：JSONL、settings.json、模型 provider
  codex/                      # Codex provider：JSONL、config.toml、thread 同步
  opencode/                   # OpenCode provider：SQLite、JSONC 配置、模型 provider
  pi/                         # Pi Coding Agent provider：JSONL、session dir
  picker/                     # bubbletea TUI，多视图状态机
  render/                     # ANSI 样式、CJK 宽度、表格格式化
  session/                    # JSONL 解析、配置读写、权限模式、命令运行、transcript
```

## 开发

```bash
go test ./...
go vet ./...
go build -ldflags="-s -w" -o agent-session ./cmd/agent-session/
```

本地运行：

```bash
./agent-session c
./agent-session cx
./agent-session oc
./agent-session p
```

## 发布

推送 `v*` tag 会触发 GitHub Actions release pipeline：

```bash
git tag v0.0.1
git push origin --tags
```

CI 会交叉编译 darwin-arm64、darwin-amd64、linux-amd64、linux-arm64、windows-amd64，并创建包含二进制和 `checksums.txt` 的 GitHub Release。
