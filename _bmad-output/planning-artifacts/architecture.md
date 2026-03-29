---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
inputDocuments:
  - '_bmad-output/planning-artifacts/prd.md'
  - 'docs/prd/CC-History-PRD.md'
workflowType: 'architecture'
project_name: 'cc-history'
user_name: 'LF'
date: '2026-03-29'
status: 'complete'
completedAt: '2026-03-29'
---

# CC History — 架构决策文档

**作者：** CC History Engineer
**日期：** 2026-03-29
**基于 PRD：** `_bmad-output/planning-artifacts/prd.md`

---

## Project Context Analysis

### Requirements Overview

**功能需求概述（FR1–FR17）：**

| 阶段 | 编号 | 能力 | 架构影响 |
|------|------|------|---------|
| MVP | FR1–FR2 | 扫描 JSONL + 当前 session 检测 | 文件扫描器 + Session 检测器 |
| MVP | FR3–FR5 | Pattern 过滤 + -A/-B/-C 上下文 | 过滤引擎（grep 语义） |
| MVP | FR6–FR10 | --all 全量模式、时间过滤、--path 覆盖 | 多 session 加载器 |
| Growth | FR11–FR14 | 导出（Markdown/JSON）+ 提示词重建 | 导出器 + Prompt 提取器 |
| Vision | FR15–FR17 | TUI 全屏界面 + session 切换 + 搜索 | Bubbletea MVU 组件 |

**非功能需求关键约束：**

- NFR1–NFR3：性能约束（500ms / 2s / 200ms）→ **流式解析**，避免全量加载
- NFR5：单一静态二进制 → Go 标准 `go build`，`CGO_ENABLED=0`
- NFR7：≥80% 测试覆盖率 → `go test -cover`，testdata fixture 文件
- NFR8：JSONL 损坏不 panic → 每行独立解析，错误跳过 + stderr warning
- NFR9：多平台（Linux/macOS amd64+arm64）→ `GOOS/GOARCH` 交叉编译

**Scale & Complexity：**

- Primary domain：**CLI tool（developer tooling）**
- Complexity level：**Moderate** — 无网络/数据库，但有流式解析 + TUI + 跨平台交叉关注点
- Estimated architectural components：6 个核心包

**Technical Constraints & Dependencies（已由 PRD 确认）：**

| 依赖 | 版本 | 用途 |
|------|------|------|
| Go | 1.21+ | 语言 + 构建 |
| Cobra | latest | CLI 参数解析框架 |
| Bubbletea | latest | TUI 框架（Vision 阶段） |
| Lipgloss | latest | 终端颜色样式 |

**Cross-Cutting Concerns：**

1. **JSONL 格式隔离**：Parser 层作为防腐层，Claude Code 格式变更不影响上层
2. **流式解析**：所有加载路径均为逐行流式，避免 OOM
3. **Session 检测逻辑**：被多个命令共享（默认视图、export、prompt）
4. **颜色/样式系统**：CLI 和 TUI 共用 Lipgloss 样式定义
5. **错误处理模式**：JSONL 损坏 = warning（非 error），命令参数错误 = error + exit 1

---

## Starter Template Evaluation

### Primary Technology Domain

**CLI Tool（Go 语言）**

Go CLI 工具无通用 starter 模板（不同于 Next.js/Rails）。标准做法是直接初始化 Go module + Cobra 骨架。

### Selected Starter：`go mod init` + Cobra 骨架

**理由：**
- Go 生态无 `create-cli-app` 类工具
- Cobra 自带 `cobra-cli init` 可生成标准骨架
- Go conventions（`cmd/`/`internal/`）已是业界标准

**初始化命令：**

```bash
mkdir cc-history && cd cc-history
go mod init github.com/a2d2-dev/cc-history
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
```

**Starter 已决定的架构：**

- **语言**：Go，`go.mod` + `go.sum` 依赖锁定
- **构建**：`go build` 产出静态二进制
- **测试**：`go test ./...` 标准测试框架
- **Lint**：`go vet` + `golangci-lint`
- **项目结构**：`cmd/`（Cobra commands）+ `internal/`（业务逻辑）+ `main.go`

---

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions（阻塞实现）：**

1. JSONL 数据模型定义
2. Session 检测算法
3. 过滤引擎（grep 语义）接口设计
4. CLI 命令路由（Cobra 子命令 vs flags）

**Important Decisions（影响架构）：**

5. 流式解析策略
6. 输出格式化层（CLI vs TUI 共享）
7. TUI 组件模型（Bubbletea MVU）

**Deferred Decisions（Post-MVP）：**

8. TOML 配置文件格式（NFR10 提到 `--help`，但配置文件格式未定）
9. goreleaser 发布流程（CI 后续完善）

---

### Data Architecture

**核心数据模型（`internal/parser` 包）：**

Claude Code JSONL 每行为一个 JSON 对象，类型由 `type` 字段决定：

```go
// JSONL 原始记录类型枚举
type RecordType string
const (
    RecordTypeUser           RecordType = "user"
    RecordTypeAssistant      RecordType = "assistant"
    RecordTypeQueueOperation RecordType = "queue-operation"  // 跳过
)

// 原始 JSONL 记录（防腐层）
type RawRecord struct {
    Type      RecordType      `json:"type"`
    UUID      string          `json:"uuid"`
    ParentUUID *string        `json:"parentUuid"`
    Timestamp string          `json:"timestamp"`
    SessionID string          `json:"sessionId"`
    CWD       string          `json:"cwd"`
    GitBranch string          `json:"gitBranch"`
    Message   json.RawMessage `json:"message"`  // 延迟解析
}

// 用户消息
type UserMessage struct {
    Role    string `json:"role"`    // "user"
    Content any    `json:"content"` // string 或 []ContentBlock
}

// 助手消息
type AssistantMessage struct {
    Role       string         `json:"role"` // "assistant"
    Content    []ContentBlock `json:"content"`
    StopReason *string        `json:"stop_reason"`
    Model      string         `json:"model"`
    Usage      *TokenUsage    `json:"usage"`
}

// 消息内容块（多态）
type ContentBlock struct {
    Type  string          `json:"type"` // "text", "tool_use", "tool_result", "thinking"
    Text  string          `json:"text,omitempty"`
    // tool_use 字段
    ID    string          `json:"id,omitempty"`
    Name  string          `json:"name,omitempty"`
    Input json.RawMessage `json:"input,omitempty"`
}

// 应用层统一消息模型（由 parser 构建）
type Message struct {
    UUID      string
    SessionID string
    Timestamp time.Time
    Role      Role          // user | assistant | tool
    Text      string        // 主要内容摘要（截断到合理长度）
    ToolCalls []ToolCall    // 仅 assistant 消息有值
    RawRecord *RawRecord    // 保留原始记录用于 export
}

type Role string
const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

type ToolCall struct {
    Name     string
    Input    string // JSON 格式
    Duration *time.Duration
}

// 会话模型
type Session struct {
    ID        string
    FilePath  string
    CWD       string
    GitBranch string
    StartTime time.Time
    EndTime   time.Time
    Messages  []Message
}
```

**设计决策：**
- `RawRecord.Message` 使用 `json.RawMessage` 延迟解析，避免未知字段 panic
- `ContentBlock` 多态处理 text/tool_use/thinking/tool_result
- `Message.Text` 在 parser 层提取摘要（截断长内容），保留 `RawRecord` 用于完整导出
- `queue-operation` 类型记录在解析时**直接跳过**

---

### Authentication & Security

**不适用。** CC History 是纯本地 CLI 工具，无网络通信、无用户认证。

唯一安全相关约束：
- 不写入任何文件（只读 JSONL）
- 尊重文件系统权限（读取失败 → 友好错误，非 panic）

---

### API & Communication Patterns

**不适用（无 API）。**

**进程内模块通信：**

```
main.go
  └── cmd/root.go (Cobra)
       ├── 无 pattern → session.Detect() → parser.LoadSession() → output.Print()
       ├── <pattern> → filter.Apply(messages, pattern, ctx) → output.Print()
       ├── --all → parser.LoadAll() → filter.Apply() → output.Print()
       ├── export → exporter.Export(session, format, writer)
       └── --tui → tui.Start(model)
```

**数据流：**
```
文件系统 → scanner.Scan() → []*FilePath
                          → parser.ParseFile(path) → Session
                          → filter.Apply(session, opts) → []Message
                          → output.Render(messages, opts) → stdout
```

---

### Frontend Architecture（TUI — Vision 阶段）

**Bubbletea MVU 模型：**

```go
// TUI 根 Model（Bubbletea tea.Model 接口）
type AppModel struct {
    State       AppState      // loading | viewing | selecting | searching
    Sessions    []Session
    ActiveIdx   int
    Messages    []Message
    Viewport    viewport.Model  // charmbracelet/bubbles
    SearchInput textinput.Model // charmbracelet/bubbles
    SearchQuery string
    FilteredMsg []Message
    Width, Height int
}

type AppState int
const (
    StateLoading AppState = iota
    StateViewing
    StateSelecting   // session 选择列表
    StateSearching   // / 激活搜索
)

// Update 处理的 Msg 类型
// - tea.KeyMsg: q(quit), s(session list), /(search), n/N(next/prev match), t(toggle tool)
// - tea.WindowSizeMsg: 响应终端窗口变化
// - LoadedMsg: 异步加载完成
```

**组件边界：**
- `viewport.Model`：可滚动消息列表（来自 `charmbracelet/bubbles`）
- `textinput.Model`：搜索输入框
- TUI 样式定义在 `internal/tui/styles.go`（Lipgloss）

---

### Infrastructure & Deployment

**构建：**

```makefile
# Makefile
build:
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/cc-history .

build-all:
    GOOS=linux  GOARCH=amd64 go build -o dist/cc-history-linux-amd64 .
    GOOS=darwin GOARCH=amd64 go build -o dist/cc-history-darwin-amd64 .
    GOOS=darwin GOARCH=arm64 go build -o dist/cc-history-darwin-arm64 .

test:
    go test -cover ./...

lint:
    go vet ./...
```

**CI（GitHub Actions）：**

```yaml
# .github/workflows/ci.yml
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.21' }
      - run: go build ./...
      - run: go test -cover ./...
      - run: go vet ./...
```

**发布（Post-MVP）：** goreleaser 自动多平台打包，GitHub Release 附件。

---

## Implementation Patterns & Consistency Rules

### 潜在冲突点

多个 AI Agent 并行开发时可能冲突的区域：**7 个**

### Naming Patterns

**Go 代码命名规范：**

```
✅ 导出类型/函数：PascalCase   →  ParseSession, FilterEngine, ToolCall
✅ 未导出函数：camelCase       →  detectSession, buildIndex
✅ 包名：lowercase 单词        →  parser, filter, session, output, tui, export
✅ 文件名：snake_case           →  session_detector.go, filter_engine.go
✅ 常量：PascalCase 或全大写   →  RoleUser, MAX_CONTENT_LENGTH
✅ 接口名：以动词/名词描述能力 →  Scanner, Renderer, Loader（不加 I 前缀）
```

**CLI 命令/Flag 命名：**

```
✅ 子命令：小写单词    →  export, prompt
✅ Long flags：kebab-case  →  --all, --no-sep, --since, --output
✅ Short flags：单字母      →  -A, -B, -C, -E, -i
✅ 环境变量：UPPER_SNAKE    →  CLAUDE_SESSION_ID, NO_COLOR
```

**输出格式：**

```
✅ 消息行格式：  YYYY-MM-DD HH:MM:SS  [role]      内容摘要
✅ 工具调用格式：YYYY-MM-DD HH:MM:SS  [tool:名称]  关键参数  (耗时)
✅ Session 分隔：--- session <id>  <datetime>  <workdir> ---
✅ 组间分隔符：  --（与 grep 完全一致）
```

### Structure Patterns

**包职责边界（强制）：**

```
internal/parser/     → 只负责 JSONL 解析，不知道 CLI 参数
internal/session/    → 只负责 session 检测和加载，不知道 output 格式
internal/filter/     → 只负责消息过滤，不知道来源是 CLI 还是 TUI
internal/output/     → 只负责 CLI 渲染，不调用 parser
internal/export/     → 只负责序列化输出，依赖 parser.Session 类型
internal/tui/        → 只负责 TUI 组件，通过 filter 包复用过滤逻辑
```

**测试文件位置：**

```
✅ 单元测试与被测文件同目录，命名 *_test.go
✅ 集成测试 fixture 在 testdata/ 目录，*.jsonl 文件
✅ 不使用独立的 tests/ 顶层目录
```

### Format Patterns

**错误处理（统一模式）：**

```go
// ✅ 正确：用 fmt.Errorf 包装，保留 context
return fmt.Errorf("解析 JSONL 文件 %s: %w", path, err)

// ✅ 正确：JSONL 损坏 → warning，继续处理
fmt.Fprintf(os.Stderr, "warning: 跳过损坏记录 %s: %v\n", path, err)

// ❌ 错误：panic
panic(err)

// ❌ 错误：静默忽略
_ = err
```

**时间处理：**

```go
// ✅ 所有时间统一使用 UTC time.Time，输出时转本地时区
// ✅ JSONL timestamp 字段用 time.Parse(time.RFC3339Nano, ...) 解析
// ✅ 输出格式：time.Format("2006-01-02 15:04:05")（本地时区）
```

**内容截断：**

```go
// ✅ CLI 输出：消息内容截断到 120 字符，超出显示 "..."
// ✅ TUI 输出：视窗宽度自适应截断
// ✅ export：完整内容，不截断
const MaxDisplayLength = 120
```

### Process Patterns

**加载策略（All Agents MUST）：**

```go
// ✅ 流式逐行解析：使用 bufio.Scanner，每行独立解析
scanner := bufio.NewScanner(f)
for scanner.Scan() {
    line := scanner.Bytes()
    // 解析单行...
}

// ❌ 禁止一次性读取整个文件
content, _ := io.ReadAll(f)  // 禁止
```

**Session 检测优先级（固定顺序，不可改变）：**

```
1. CLAUDE_SESSION_ID 环境变量（精确匹配 session ID）
2. --path 指定目录下最近修改的 JSONL 文件
3. 默认 ~/.claude/projects/ 下最近修改的 JSONL 文件
4. 回退：取第一个可用 session + 打印 warning 到 stderr
```

**颜色输出：**

```go
// ✅ 检查 NO_COLOR 环境变量 和 terminal.IsTerminal(stdout)
// ✅ 非 TTY（管道输出）时禁用颜色
// ✅ 使用 lipgloss.Style，不直接写 ANSI escape codes
```

### Enforcement Guidelines

**所有 AI Agent 必须：**

- 新增包时在 `internal/` 下创建，不在根目录添加包
- Parser 包只依赖标准库（`encoding/json`、`bufio`、`time`），不依赖 Cobra 或 Bubbletea
- 每个 PR 通过 `go test -cover ./... ` 覆盖率 ≥ 80%
- 不使用 `init()` 函数（Cobra 命令注册除外）
- 不使用全局变量（依赖通过函数参数传递）

---

## Project Structure & Boundaries

### Complete Project Directory Structure

```
cc-history/
├── main.go                          # 程序入口，调用 cmd.Execute()
├── go.mod                           # Go module 定义
├── go.sum                           # 依赖锁定
├── Makefile                         # build / test / lint / release
├── README.md                        # 用户文档
├── .github/
│   └── workflows/
│       └── ci.yml                   # GitHub Actions CI
├── cmd/
│   ├── root.go                      # 根命令 + 全局 flags (--all, --path, --since, --until, -A, -B, -C, -E, --no-sep, --tui, -i)
│   ├── export.go                    # `export` 子命令 (--format, --session, --output)
│   └── prompt.go                    # `prompt` 子命令 (--range, --copy, --output)
├── internal/
│   ├── parser/
│   │   ├── types.go                 # RawRecord, Message, Session, ToolCall 数据模型
│   │   ├── parser.go                # 流式 JSONL 解析主逻辑
│   │   ├── content.go               # ContentBlock 多态解析
│   │   └── parser_test.go           # 单元测试（testdata 引用）
│   ├── session/
│   │   ├── scanner.go               # 扫描 ~/.claude/projects/ JSONL 文件列表
│   │   ├── detector.go              # 当前 session 检测逻辑（ENV → 最近修改文件）
│   │   ├── loader.go                # 按需加载 Session（单个 or 全量）
│   │   └── session_test.go
│   ├── filter/
│   │   ├── filter.go                # Pattern 过滤引擎（子串 + 正则）
│   │   ├── context.go               # -A/-B/-C 上下文行计算
│   │   ├── timerange.go             # --since/--until 时间过滤
│   │   └── filter_test.go
│   ├── output/
│   │   ├── renderer.go              # CLI 消息流渲染（Lipgloss 样式）
│   │   ├── styles.go                # Lipgloss 样式定义（颜色、前缀）
│   │   └── renderer_test.go
│   ├── export/
│   │   ├── markdown.go              # Markdown 导出实现
│   │   ├── json.go                  # JSON 导出实现
│   │   └── export_test.go
│   └── tui/                         # Vision 阶段
│       ├── model.go                 # Bubbletea AppModel + AppState
│       ├── update.go                # tea.Update() 消息处理
│       ├── view.go                  # tea.View() 渲染
│       ├── styles.go                # TUI Lipgloss 样式
│       └── tui_test.go
└── testdata/
    ├── single_session.jsonl         # 单 session fixture（含 user/assistant/tool_use）
    ├── multi_session/               # 多 session fixtures
    │   ├── session_a.jsonl
    │   └── session_b.jsonl
    ├── corrupted.jsonl              # 包含损坏行的 fixture
    └── empty.jsonl                  # 空文件 fixture
```

### Architectural Boundaries

**数据边界：**

```
[文件系统 JSONL] → session.Scanner → []*os.FileInfo
                → session.Loader  → []Session
                → parser.Parser   → Session{Messages}
                → filter.Engine   → []Message（过滤后）
                → output.Renderer → stdout（CLI）
                → tui.Model       → terminal（TUI）
                → exporter        → io.Writer（file/stdout）
```

**包依赖图（严格遵守，无循环）：**

```
cmd/ → internal/session, internal/filter, internal/output, internal/export, internal/tui
internal/session → internal/parser
internal/filter  → internal/parser
internal/output  → internal/parser
internal/export  → internal/parser
internal/tui     → internal/session, internal/filter, internal/parser
internal/parser  → stdlib only（encoding/json, bufio, time, os）
```

### Requirements to Structure Mapping

| FR | 实现位置 |
|----|---------|
| FR1（JSONL 扫描） | `internal/session/scanner.go` |
| FR2（当前 session 检测） | `internal/session/detector.go` |
| FR3（pattern 过滤） | `internal/filter/filter.go` |
| FR4（-E 正则） | `internal/filter/filter.go` |
| FR5（-A/-B/-C 上下文） | `internal/filter/context.go` |
| FR6（--all 全量） | `internal/session/loader.go` + `cmd/root.go` |
| FR7（--all + pattern 组合） | `internal/filter/filter.go`（复用） |
| FR8（--since/--until） | `internal/filter/timerange.go` |
| FR9（--path 覆盖） | `internal/session/scanner.go` |
| FR10（工具调用显示） | `internal/output/renderer.go` |
| FR11（export 命令） | `cmd/export.go` |
| FR12（Markdown 导出） | `internal/export/markdown.go` |
| FR13（JSON 导出） | `internal/export/json.go` |
| FR14（prompt 提取） | `cmd/prompt.go` |
| FR15–FR17（TUI） | `internal/tui/` |

### Integration Points

**外部数据源：**

- `~/.claude/projects/**/*.jsonl`（只读，文件系统）
- `CLAUDE_SESSION_ID`（环境变量）
- `NO_COLOR`（环境变量）
- `os.Stdout`（输出目标）

**内部通信：**
- 所有模块间通过函数参数传递（无全局 state）
- `filter.Engine` 接受 `[]Message` 返回 `[]Message`，与来源无关
- `output.Renderer` 接受 `[]Message` + `RenderOptions`，不依赖 parser 内部实现

---

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility：**
- Go 1.21 + Cobra + Bubbletea + Lipgloss 全部兼容，无版本冲突
- `encoding/json` 流式解析与 `bufio.Scanner` 配合完美
- Lipgloss 是 Bubbletea 生态配套，CLI 和 TUI 可共用样式定义

**Pattern Consistency：**
- 所有模块遵循「依赖入参、返回值」而非全局 state，支持并发和测试
- 命名规范（PascalCase/camelCase）与 Go 标准一致
- 错误处理统一使用 `fmt.Errorf + %w` 包装链

**Structure Alignment：**
- `cmd/` 只负责解析参数和路由，核心逻辑在 `internal/`
- 包依赖图无循环，严格分层

### Requirements Coverage Validation ✅

**FR Coverage：**
- FR1–FR17 全部映射到具体包和文件（见上方 mapping 表）
- Growth/Vision 阶段 FR 有预留包（export/tui），不影响 MVP 构建

**NFR Coverage：**
- NFR1–NFR3（性能）：流式解析 + 不全量加载 → 满足
- NFR5（单一二进制）：`CGO_ENABLED=0 go build -ldflags="-s -w"` → 满足
- NFR7（80% 覆盖率）：testdata fixtures + `_test.go` 并行 → 满足
- NFR8（损坏不 panic）：每行独立解析 + 跳过机制 → 满足
- NFR9（多平台）：交叉编译 Makefile → 满足

### Implementation Readiness Validation ✅

- 所有关键决策文档化，版本明确（Go 1.21+、Cobra latest 等）
- 数据模型结构体定义完整，可直接作为 Story 1.3 的实现参考
- 包依赖图清晰，无循环，AI Agent 独立实现各包不会冲突
- Session 检测优先级明确（固定 4 步顺序）

### Gap Analysis Results

**无 Critical Gap。**

**Minor Gaps（Post-MVP 处理）：**
- TOML 配置文件格式未定（MVP 阶段纯 CLI flags）
- goreleaser 配置文件（发布阶段添加）
- Windows 支持（PRD NFR9 未列 Windows amd64 为强制目标）

### Architecture Readiness Assessment

**Overall Status：READY FOR IMPLEMENTATION**

**Confidence Level：High**

**Key Strengths：**
- 数据模型明确（与真实 JSONL 格式对应验证）
- 包边界清晰，MVP/Growth/Vision 阶段独立可交付
- 过滤引擎设计复用于 CLI 和 TUI，避免重复实现

**First Implementation Priority：**

```bash
# Story 1.1: 项目骨架初始化
go mod init github.com/a2d2-dev/cc-history
cobra-cli init
```

---

## Next Steps

`bmad-create-epics-and-stories` 工作流的输入：
- PRD：`_bmad-output/planning-artifacts/prd.md`
- Architecture：`_bmad-output/planning-artifacts/architecture.md`（本文档）

Epic 划分参考：
- **E1**：项目基础设施 & 数据加载（Story 1.1–1.4）→ `cmd/`, `internal/parser/`, `internal/session/`, `internal/output/`
- **E2**：类 grep 过滤 & 全量历史模式（Story 2.1–2.3）→ `internal/filter/`
- **E3**：导出 & 提示词重建（Story 3.1–3.2）→ `internal/export/`, `cmd/export.go`, `cmd/prompt.go`
- **E4**：TUI 交互界面（Story 4.1–4.2）→ `internal/tui/`
