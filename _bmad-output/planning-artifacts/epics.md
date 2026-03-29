---
stepsCompleted: ['step-01', 'step-02', 'step-03', 'step-04']
inputDocuments:
  - '_bmad-output/planning-artifacts/prd.md'
  - '_bmad-output/planning-artifacts/architecture.md'
workflowType: 'epics-and-stories'
project_name: 'cc-history'
status: 'complete'
completedAt: '2026-03-29'
---

# cc-history - Epic Breakdown

## Overview

本文档提供 cc-history 项目的完整 Epic 和 Story 拆分，将 PRD（FR1–FR17）和 Architecture 的需求分解为可独立交付的开发任务。

---

## Requirements Inventory

### Functional Requirements

**MVP 阶段：**

- **FR1**：扫描 `~/.claude/projects/**/*.jsonl`，解析为 Session/Message/ToolCall；损坏记录跳过并记录 warning
- **FR2**：无参运行时，优先读取 `CLAUDE_SESSION_ID` 环境变量检测当前 session；否则取最近修改的 JSONL；均失败则回退并打印提示
- **FR3**：`cc-history <pattern>` 对当前 session 做子字符串过滤，匹配组间用 `--` 分隔
- **FR4**：`-E` 启用正则表达式模式（Go 标准正则），解析错误时输出明确错误信息并以非零退出
- **FR5**：`-A N`（after）、`-B N`（before）、`-C N`（context）在匹配消息周围显示额外 N 条消息；无 pattern 时忽略
- **FR6**：`--all` 输出所有 session 消息，跨 session 按时间升序，session 切换处打印 `--- session <id>  <datetime>  <workdir> ---`；`--no-sep` 禁用分隔行
- **FR7**：`--all` 支持与 `<pattern>`、`-A`/`-B`/`-C` 组合，行为与单 session 一致
- **FR8**：`--since YYYY-MM-DD` / `--until YYYY-MM-DD` 时间范围过滤；格式错误时输出明确错误
- **FR9**：`--path <dir>` 覆盖默认数据目录；路径不存在时输出友好错误提示
- **FR10**：工具调用格式：`[tool:<工具名>]  <关键参数>  (<执行时间>)`，单行简洁输出

**Growth 阶段：**

- **FR11**：`cc-history export --format <markdown|json>` 导出当前 session；`--session <id>` 指定其他 session；`--output <file>` 指定输出文件
- **FR12**：Markdown 导出包含：会话元信息（时间、工作目录）、每条消息角色标记和完整内容、工具调用输入/输出
- **FR13**：JSON 导出为完整会话数据结构，与内部 Session/Message/ToolCall 模型一致
- **FR14**：`cc-history prompt --range <start>-<end>` 提取消息范围生成提示词模板；`--copy` 复制到剪贴板；`--output <file>` 保存文件

**Vision 阶段：**

- **FR15**：`cc-history --tui`（或 `-i`）进入全屏 TUI；显示当前 session 消息流，可上下滚动；`q` 退出，`?` 显示帮助
- **FR16**：TUI 中 user/assistant/tool 消息通过颜色和前缀区分；工具调用默认折叠，`t` 键展开/折叠
- **FR17**：TUI 中 `s` 键打开 session 选择列表，Enter 切换，Esc 取消；`/` 激活搜索，输入时实时过滤，`n`/`N` 跳转匹配项

### NonFunctional Requirements

- **NFR1**：当前 session 加载时间 ≤ 500ms（`time cc-history`）
- **NFR2**：`--all` 模式 1000 条消息加载 ≤ 2s
- **NFR3**：pattern 过滤（10000 条消息）≤ 200ms
- **NFR4**：内存峰值 ≤ 200MB（1000 条消息）
- **NFR5**：单一静态链接二进制，Linux amd64 / macOS amd64 / macOS arm64 无运行时依赖
- **NFR6**：二进制大小 ≤ 20MB（strip 后）
- **NFR7**：单元测试覆盖率 ≥ 80%（`go test -cover ./...`）
- **NFR8**：JSONL 解析错误不 panic，跳过损坏记录 + stderr warning，退出码 0
- **NFR9**：支持 Linux/macOS，xterm-256color / iTerm2 / tmux 环境验证颜色输出

### Additional Requirements

来自 Architecture 文档的技术实现要求：

- **ARCH-1**：`go mod init github.com/a2d2-dev/cc-history`，Go 1.21+，所有依赖通过 `go mod` 管理
- **ARCH-2**：项目结构：`cmd/`（Cobra 命令）+ `internal/`（业务逻辑）+ `testdata/`（JSONL fixture）
- **ARCH-3**：JSONL 解析使用 `bufio.Scanner` 逐行流式读取，不一次性加载整个文件
- **ARCH-4**：Parser 包只依赖标准库（`encoding/json`、`bufio`、`time`、`os`）
- **ARCH-5**：Session 检测优先级固定：`CLAUDE_SESSION_ID` → `--path` 最近修改文件 → 默认目录最近修改文件 → 回退 + warning
- **ARCH-6**：`CGO_ENABLED=0 go build -ldflags="-s -w"` 确保静态二进制
- **ARCH-7**：GitHub Actions CI：`go build ./...` + `go test -cover ./...` + `go vet ./...`
- **ARCH-8**：输出颜色检测：`NO_COLOR` 环境变量 + `terminal.IsTerminal(stdout)` 非 TTY 禁色
- **ARCH-9**：TUI 使用 Bubbletea MVU 模型（AppModel/Update/View），`charmbracelet/bubbles` 组件

### UX Design Requirements

（无独立 UX 文档，UX 需求已内嵌于 PRD User Journeys 中，不适用）

---

### FR Coverage Map

| FR | Epic | 说明 |
|----|------|------|
| FR1 | E1 | JSONL 扫描 + 解析（Story 1.2、1.3） |
| FR2 | E1 | 当前 Session 检测（Story 1.4） |
| FR3 | E2 | Pattern 子串过滤（Story 2.1） |
| FR4 | E2 | -E 正则模式（Story 2.1） |
| FR5 | E2 | -A/-B/-C 上下文行（Story 2.2） |
| FR6 | E2 | --all 全量模式 + 分隔行（Story 2.3） |
| FR7 | E2 | --all + pattern 组合（Story 2.3） |
| FR8 | E2 | --since/--until 时间过滤（Story 2.3） |
| FR9 | E1 | --path 覆盖（Story 1.2） |
| FR10 | E1 | 工具调用单行显示（Story 1.4） |
| FR11 | E3 | export 命令路由（Story 3.1） |
| FR12 | E3 | Markdown 导出（Story 3.1） |
| FR13 | E3 | JSON 导出（Story 3.1） |
| FR14 | E3 | prompt 提取命令（Story 3.2） |
| FR15 | E4 | TUI 模式入口 + 消息流（Story 4.1） |
| FR16 | E4 | TUI 颜色 + 工具调用折叠（Story 4.1） |
| FR17 | E4 | TUI session 切换 + 搜索（Story 4.2） |

---

## Epic List

### Epic 1: 项目基础 & 当前 Session 对话流

开发者可运行 `cc-history` 查看当前 Claude Code session 的完整对话流，包含 user/assistant/工具调用，支持 `--path` 覆盖数据目录。
**FRs covered：** FR1、FR2、FR9、FR10 + NFR1、NFR5、NFR6、NFR7、NFR8 + ARCH-1~8

### Epic 2: 类 grep 过滤 & 全量历史模式

开发者可通过 `<pattern>` 过滤消息、`-A/-B/-C` 查看上下文、`--all` 跨 session 检索历史，并支持 `--since/--until` 时间范围限定。
**FRs covered：** FR3、FR4、FR5、FR6、FR7、FR8 + NFR2、NFR3

### Epic 3: 导出 & 提示词重建

开发者可将 session 导出为 Markdown/JSON 文档，或从历史对话中提取提示词模板供复用。
**FRs covered：** FR11、FR12、FR13、FR14

### Epic 4: 交互式 TUI 模式

开发者可通过 `--tui`/`-i` 进入全屏交互界面，浏览消息流、切换 session、实时搜索、展开工具调用详情。
**FRs covered：** FR15、FR16、FR17 + ARCH-9

---

## Epic 1: 项目基础 & 当前 Session 对话流

**目标：** 搭建 Go 项目骨架，实现 Claude Code JSONL 解析，输出当前 session 对话流。本 Epic 完成后，`cc-history` 可运行并显示完整对话内容。

### Story 1.1: 项目骨架初始化

As a 开发者，
I want 一个可编译、可测试的 Go 项目骨架，
So that 后续所有功能都基于统一的结构开发，CI 自动验证构建。

**Acceptance Criteria:**

**Given** 全新机器只有 Go 1.21+ 和 Git
**When** 执行 `go mod init github.com/a2d2-dev/cc-history && go build ./...`
**Then** 成功编译出 `cc-history` 可执行文件，无报错

**Given** 可执行文件已构建
**When** 执行 `./cc-history --version`
**Then** 输出版本号 `0.1.0`

**Given** 可执行文件已构建
**When** 执行 `./cc-history --help`
**Then** 输出 Cobra 生成的帮助信息，包含所有已注册子命令

**Given** 代码推送到 GitHub
**When** GitHub Actions CI 触发
**Then** `go build ./...`、`go test -cover ./...`、`go vet ./...` 全部通过

**Given** `go build` 时设置 `CGO_ENABLED=0 -ldflags="-s -w"`
**When** 检查构建产物
**Then** 二进制文件在 Linux amd64 上不依赖任何动态库（`ldd cc-history` 输出 `not a dynamic executable`），大小 ≤ 20MB

---

### Story 1.2: JSONL 文件扫描器

As a 开发者，
I want 程序自动发现 `~/.claude/projects/` 下所有 JSONL 文件，
So that 后续解析步骤能拿到完整的文件列表，无需手动指定路径。

**Acceptance Criteria:**

**Given** `~/.claude/projects/` 下存在若干 `.jsonl` 文件（含子目录）
**When** 调用 `scanner.Scan("")`（空字符串表示使用默认路径）
**Then** 返回所有 `.jsonl` 文件的绝对路径列表，包含子目录中的文件

**Given** 用户传入 `--path /custom/dir` 参数
**When** 调用 `scanner.Scan("/custom/dir")`
**Then** 扫描 `/custom/dir/**/*.jsonl`，忽略默认路径

**Given** 指定目录不存在
**When** 调用 `scanner.Scan("/nonexistent")`
**Then** 返回明确错误信息（如 `目录不存在: /nonexistent`），不 panic

**Given** 目录下有 1000 个 JSONL 文件
**When** 执行扫描
**Then** 完成时间 < 100ms

**Given** 单元测试中使用 `testdata/` 目录
**When** 运行 `go test ./internal/session/...`
**Then** 覆盖：空目录、单文件、嵌套目录、文件权限拒绝等边界场景

---

### Story 1.3: JSONL 解析器

As a 开发者，
I want 程序将 JSONL 文件解析为 Go 结构体（Session/Message/ToolCall），
So that 后续显示和过滤功能使用统一的数据模型。

**Acceptance Criteria:**

**Given** 一个包含 `user`/`assistant`/`queue-operation` 类型记录的 JSONL 文件
**When** 调用 `parser.ParseFile(path)`
**Then** 返回一个 `Session`，其中 `Messages` 只包含 user 和 assistant 类型，`queue-operation` 记录被跳过

**Given** assistant 消息的 `content` 中包含 `tool_use` ContentBlock
**When** 解析该消息
**Then** `Message.ToolCalls` 包含对应 `ToolCall{Name, Input}` 数据

**Given** JSONL 文件某一行 JSON 格式损坏（不完整）
**When** 解析该文件
**Then** 程序跳过该行，向 stderr 输出 warning（如 `warning: 跳过损坏记录...`），继续解析其余行，不 panic，退出码为 0

**Given** 完全空的 JSONL 文件
**When** 解析该文件
**Then** 返回空 `Session{Messages: []}`，无错误

**Given** 使用 `bufio.Scanner` 逐行读取
**When** 文件大小超过默认 scanner buffer（64KB）
**Then** 扩展 buffer（`scanner.Buffer(buf, maxCapacity)`）确保超长行也能正常解析

**Given** 单元测试使用 `testdata/single_session.jsonl`、`testdata/corrupted.jsonl`、`testdata/empty.jsonl`
**When** 运行 `go test -cover ./internal/parser/...`
**Then** 覆盖率 ≥ 80%，所有边界用例通过

---

### Story 1.4: 当前 Session 对话流输出

As a 开发者，
I want 运行 `cc-history` 立即看到当前 session 的对话内容，
So that 不离开终端即可快速回顾本次工作内容。

**Acceptance Criteria:**

**Given** 当前环境设置了 `CLAUDE_SESSION_ID=<session-id>` 且对应 JSONL 文件存在
**When** 执行 `cc-history`
**Then** 输出该 session 的完整消息流，按时间顺序排列

**Given** `CLAUDE_SESSION_ID` 未设置，`~/.claude/projects/` 下有多个 JSONL 文件
**When** 执行 `cc-history`
**Then** 输出最近修改的 JSONL 所对应 session 的消息流

**Given** 无法确定当前 session（目录为空）
**When** 执行 `cc-history`
**Then** 向 stderr 输出提示信息（如 `未能检测当前 session，显示最近一个 session`），显示最近可用 session

**Given** session 含 user/assistant/tool 消息
**When** 输出消息流
**Then** 每条消息格式为 `2026-03-29 10:30:01  [user]       内容摘要（≤120字符）`；工具调用格式为 `2026-03-29 10:30:06  [tool:Read]  middleware/auth.go  (12ms)`

**Given** 运行时设置 `NO_COLOR=1` 或 stdout 非 TTY（管道输出）
**When** 输出消息流
**Then** 无 ANSI 颜色转义码

**Given** 当前 session 文件大小正常（<10MB）
**When** 执行 `time cc-history`
**Then** 加载并输出第一条消息的时间 ≤ 500ms

---

## Epic 2: 类 grep 过滤 & 全量历史模式

**目标：** 在 Epic 1 基础上，实现 pattern 过滤、`-A`/`-B`/`-C` 上下文参数、`--all` 全量历史，以及时间范围过滤。本 Epic 完成后 cc-history 成为强大的历史检索工具。

### Story 2.1: Pattern 过滤与正则支持

As a 开发者，
I want 通过 `cc-history <pattern>` 过滤当前 session 消息，
So that 快速找到包含特定关键词或正则匹配的对话片段。

**Acceptance Criteria:**

**Given** 当前 session 包含 10 条消息，其中 3 条包含 "auth"
**When** 执行 `cc-history auth`
**Then** 仅输出含 "auth" 的 3 条消息，共 3 行；不连续匹配组间用 `--` 单独一行分隔

**Given** pattern 为 `auth`，session 中无匹配消息
**When** 执行 `cc-history auth`
**Then** 无输出（空），退出码为 0

**Given** 执行 `cc-history -E "token.*(expired|invalid)"`
**When** 存在消息内容匹配该正则
**Then** 正确输出匹配行，行为与子串匹配版本一致

**Given** 执行 `cc-history -E "[invalid"`（非法正则）
**When** 程序启动
**Then** 向 stderr 输出明确错误信息（如 `正则表达式错误: missing closing ]`），退出码非零

**Given** 单元测试中构造 `[]Message` 数组
**When** 调用 `filter.Apply(messages, pattern, FilterOptions{Regex: false})`
**Then** 返回正确的匹配消息列表（含分隔符信息），覆盖：空 pattern、无匹配、全匹配、多组不连续匹配

---

### Story 2.2: -A/-B/-C 上下文参数

As a 开发者，
I want 在每个匹配消息周围显示前后 N 条消息，
So that 理解匹配内容的完整对话背景，行为与 GNU grep 一致。

**Acceptance Criteria:**

**Given** session 有消息 [1,2,3,4,5]，消息 3 匹配 pattern
**When** 执行 `cc-history -B 1 -A 1 <pattern>`
**Then** 输出消息 [2, 3, 4]（匹配前 1 条 + 匹配行 + 匹配后 1 条）

**Given** `-C 2` 等价于 `-A 2 -B 2`
**When** 执行 `cc-history -C 2 <pattern>`
**Then** 输出每个匹配消息前后各 2 条消息

**Given** 多个匹配项，上下文窗口有重叠（相邻匹配距离 < N）
**When** 输出
**Then** 重叠部分不重复，合并为连续消息块；组间用 `--` 分隔

**Given** 匹配项在消息列表开头或结尾（上下文不足 N 条）
**When** 输出
**Then** 显示实际可用消息（不越界），不报错

**Given** 无 pattern 参数，仅传 `-A 3`
**When** 执行 `cc-history -A 3`
**Then** `-A 3` 被忽略，输出完整消息流（与无参数行为一致）

---

### Story 2.3: --all 全量历史模式 & 时间过滤

As a 开发者，
I want 通过 `--all` 查看所有 session 的完整历史，并支持时间范围限定，
So that 在全量历史中检索特定时期的对话，支持跨 session 搜索。

**Acceptance Criteria:**

**Given** `~/.claude/projects/` 下有多个 session JSONL 文件
**When** 执行 `cc-history --all`
**Then** 输出所有 session 的消息，跨 session 按时间升序排列；每个 session 开始处打印分隔行 `--- session <id>  <datetime>  <workdir> ---`

**Given** 执行 `cc-history --all --no-sep`
**When** 输出
**Then** 不显示 session 分隔行，消息流连续输出

**Given** 执行 `cc-history --all auth`
**When** 包含多个 session
**Then** 先合并所有 session 消息，再按 pattern 过滤，行为与单 session 过滤一致（含组间 `--` 分隔符）

**Given** 执行 `cc-history --all -C 2 "token expired"`
**When** 输出
**Then** `--all` 与 `-A`/`-B`/`-C` 正确组合，上下文跨 session 边界时不越过 session 分隔

**Given** 执行 `cc-history --since 2026-03-01 --until 2026-03-29`
**When** 目录中有跨越该时间范围的多个 session
**Then** 仅输出时间戳在范围内的消息

**Given** 执行 `cc-history --since 2026-13-01`（非法日期）
**When** 程序启动
**Then** 向 stderr 输出明确错误信息（如 `--since 日期格式错误，应为 YYYY-MM-DD`），退出码非零

**Given** 1000 条消息全量加载
**When** 执行 `time cc-history --all`
**Then** 完成时间 ≤ 2s

---

## Epic 3: 导出 & 提示词重建

**目标：** 提供会话导出（Markdown/JSON）和提示词模板提取功能，让开发者可将对话内容整理为文档或复用模板。

### Story 3.1: 会话导出（Markdown/JSON）

As a 开发者，
I want 将 Claude Code session 导出为 Markdown 或 JSON 文件，
So that 整理对话记录分享给团队，或归档知识沉淀。

**Acceptance Criteria:**

**Given** 当前 session 有完整消息
**When** 执行 `cc-history export --format markdown`
**Then** 输出 Markdown 内容到 stdout，包含：会话元信息（时间、工作目录）、每条消息的角色标记和完整内容、工具调用的输入参数和返回结果

**Given** 执行 `cc-history export --format markdown --output session.md`
**When** 命令完成
**Then** 文件 `session.md` 创建成功，内容与 stdout 输出一致；终端打印 `已导出到 session.md（N 条消息，X KB）`

**Given** 执行 `cc-history export --format json`
**When** 输出
**Then** JSON 结构与内部 `Session` 模型一致，所有字段完整序列化，可通过 `jq` 解析

**Given** 执行 `cc-history export --session <session-id>`
**When** 该 session ID 存在
**Then** 导出指定 session，而非当前 session

**Given** 执行 `cc-history export --session nonexistent-id`
**When** 命令运行
**Then** 向 stderr 输出 `session 不存在: nonexistent-id`，退出码非零

**Given** 执行 `cc-history export --format pdf`（不支持的格式）
**When** 命令运行
**Then** 向 stderr 输出 `不支持的格式: pdf（支持: markdown, json）`，退出码非零

---

### Story 3.2: 提示词模板提取

As a 开发者，
I want 从历史对话中提取消息范围生成提示词模板，
So that 复用成功的提示词模式，提升工作效率。

**Acceptance Criteria:**

**Given** 当前 session 有 10 条消息
**When** 执行 `cc-history prompt --range 1-3`
**Then** 输出消息 1-3 的提示词模板到 stdout，格式包含：工作目录、日期、用户输入内容（完整）、助手响应摘要

**Given** 执行 `cc-history prompt --range 2-5 --output prompt.md`
**When** 命令完成
**Then** 文件 `prompt.md` 创建成功，内容为提取的提示词模板

**Given** 执行 `cc-history prompt --range 2-5 --copy`
**When** 系统剪贴板可用（`xclip`/`pbcopy` 存在）
**Then** 模板内容复制到剪贴板，终端打印 `已复制到剪贴板`

**Given** 执行 `cc-history prompt --range 2-5 --copy`
**When** 剪贴板工具不可用（无 `xclip`/`pbcopy`）
**Then** 向 stderr 输出 warning `剪贴板工具不可用，请手动复制`，同时将内容输出到 stdout

**Given** 执行 `cc-history prompt --range 100-200`，session 只有 10 条消息
**When** 命令运行
**Then** 向 stderr 输出明确错误信息（如 `范围超出消息数量（当前 session 共 10 条消息）`），退出码非零

---

## Epic 4: 交互式 TUI 模式

**目标：** 实现 Bubbletea 全屏 TUI 界面，提供可滚动消息流、工具调用折叠/展开、session 切换和实时搜索。

### Story 4.1: TUI 基础框架 & 消息流视图

As a 开发者，
I want 通过 `--tui` 或 `-i` 进入全屏交互界面浏览当前 session 消息，
So that 在无法使用管道工具的环境中，通过键盘导航方便地浏览对话历史。

**Acceptance Criteria:**

**Given** 执行 `cc-history --tui`
**When** 程序启动
**Then** 进入全屏 Bubbletea TUI，显示当前 session 的可滚动消息流；user/assistant/tool 消息通过颜色和前缀（`[user]`/`[assistant]`/`[tool:名称]`）视觉区分

**Given** TUI 已启动
**When** 按 `↑`/`↓` 或 `j`/`k` 键
**Then** 消息列表上下滚动，滚动流畅

**Given** TUI 消息流中包含工具调用
**When** 默认状态下
**Then** 工具调用显示为折叠的单行摘要：`[tool:Read]  path/to/file  (12ms)`

**Given** 光标停在折叠的工具调用行
**When** 按 `t` 键
**Then** 展开显示工具调用的完整输入参数和返回结果；再次按 `t` 折叠

**Given** TUI 处于任意状态
**When** 按 `q` 键
**Then** 退出 TUI，回到普通终端，退出码 0

**Given** TUI 处于任意状态
**When** 按 `?` 键
**Then** 显示帮助覆盖层，列出所有可用快捷键；任意键关闭帮助

**Given** 终端窗口尺寸变化
**When** 接收到 `tea.WindowSizeMsg`
**Then** TUI 自适应新的宽高，消息内容重新排版，不崩溃

---

### Story 4.2: TUI Session 切换 & 实时搜索

As a 开发者，
I want 在 TUI 中切换不同 session 并实时搜索消息，
So that 无需退出 TUI 即可在多个 session 之间导航和检索。

**Acceptance Criteria:**

**Given** TUI 处于消息流视图
**When** 按 `s` 键
**Then** 打开 session 选择列表，显示所有可用 session（按时间倒序），每行格式：`<datetime>  <session-id-前8位>  <workdir>`

**Given** session 选择列表已打开
**When** 按 `↑`/`↓` 移动光标后按 `Enter`
**Then** 切换到选中 session，回到消息流视图，显示新 session 的消息

**Given** session 选择列表已打开
**When** 按 `Esc`
**Then** 关闭列表，回到原 session 消息流视图，不切换

**Given** TUI 处于消息流视图
**When** 按 `/` 键
**Then** 底部显示搜索输入框，进入搜索模式

**Given** 搜索模式已激活，用户输入 "auth"
**When** 每次输入字符时
**Then** 实时过滤消息流，仅显示包含 "auth" 的消息（含上下文高亮）

**Given** 搜索模式中有多个匹配结果
**When** 按 `n` 键
**Then** 跳转到下一个匹配项；按 `N` 跳转到上一个匹配项

**Given** 搜索模式已激活
**When** 按 `Esc`
**Then** 退出搜索模式，恢复完整消息流视图，清空搜索词

---

## Architecture Validation ✅

| 检查项 | 状态 |
|--------|------|
| FR1–FR17 全部映射到 Story | ✅ |
| E1 Story 1.1 包含项目骨架初始化（对应 ARCH-1~7） | ✅ |
| Stories 不依赖未来 Stories | ✅ |
| 每个 Epic 独立可交付 | ✅（E1→E2→E3→E4 递进但每阶段独立） |
| 无「创建所有表/包」类 Story | ✅（Go CLI 无数据库） |
| NFR5/NFR6（单一二进制）覆盖 | ✅ Story 1.1 AC |
| NFR7（80% 覆盖率）覆盖 | ✅ 每个 parser/filter Story 均有测试 AC |
| NFR8（损坏不 panic）覆盖 | ✅ Story 1.3 AC |
