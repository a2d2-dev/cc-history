# CC History - Product Requirements Document

## 文档信息

| 字段 | 内容 |
|------|------|
| **项目名称** | CC History |
| **文档版本** | 1.2.0 |
| **创建日期** | 2026-03-29 |
| **负责人** | CC History Engineer |
| **文档状态** | Draft |

---

## 1. 背景与目标

### 1.1 背景

Claude Code 是一个强大的 AI 辅助开发工具，但在使用过程中存在以下痛点：

- **历史回溯困难**: 用户难以查找和回顾之前的会话内容
- **提示词复用不便**: 无法从历史记录中快速重建和复用成功的提示词
- **工具调用追溯**: 缺乏对工具调用历史的可视化查看
- **子引擎协作分析**: 无法清晰查看子工程师（sub-engine）的工作过程

### 1.2 项目目标

构建一个 Claude Code 会话历史查看器，实现：

- ✅ 加载并展示 Claude Code 会话数据（用户输入、Claude 输出、工具调用、子引擎数据）
- ✅ 按时间顺序展示会话历史
- ✅ 支持会话过滤和搜索
- ✅ 支持从历史记录重建提示词
- ✅ 提供直观的 CLI/TUI 界面

### 1.3 成功指标

- 能够加载至少 1000 条历史记录而不影响性能
- 搜索响应时间 < 500ms
- 支持导出历史记录为 Markdown/JSON 格式

---

## 2. 用户故事

### 2.1 核心用户故事

| ID | 故事描述 | 优先级 |
|----|---------|--------|
| **US-001** | 作为一个开发者，我想要查看所有 Claude Code 会话列表，以便选择我感兴趣的会话 | P0 |
| **US-002** | 作为一个开发者，我想要查看单个会话的完整对话历史，包括工具调用，以便回顾完整的开发过程 | P0 |
| **US-003** | 作为一个开发者，我想要按时间顺序查看会话中的所有消息，以便理解对话的上下文流动 | P0 |
| **US-004** | 作为一个开发者，我想要搜索包含特定关键词的会话，以便快速找到相关信息 | P1 |
| **US-005** | 作为一个开发者，我想要从历史记录中提取并重建提示词，以便在新的会话中复用 | P1 |
| **US-006** | 作为一个开发者，我想要查看工具调用的详细信息（输入、输出、执行时间），以便调试和优化 | P1 |
| **US-007** | 作为一个开发者，我想要导出会话历史为 Markdown/JSON，以��文档化或分享 | P2 |
| **US-008** | 作为一个开发者，我想要按日期、项目、工具类型等维度过滤会话，以便更好地组织历史记录 | P2 |

### 2.2 用户角色

| 角色 | 描述 | 主要需求 |
|------|------|---------|
| **开发者** | 使用 Claude Code 进行日常开发 | 快速查找历史、复用提示词、追溯工具调用 |
| **技术负责人** | 需要审查团队的 Claude Code 使用情况 | 统计分析、使用模式识别 |
| **知识管理者** | 需要将有价值的对话沉淀为文档 | 导出功能、提示词提取 |

---

## 3. 功能需求

### 3.1 会话数据加载 (FR-001)

**需求描述**: 系统应能够从 Claude Code 的数据存储中加载会话数据

**输入**:
- Claude Code 会话数据路径（默认: `~/.claude/sessions/`）
- 可选的会话 ID 或时间范围过滤

**输出**:
- 解析后的会话数据结构

**验收标准**:
- [ ] 支持读取 Claude Code 的会话存储格式
- [ ] 能够解析用户输入、Claude 输出、工具调用、子引擎数据
- [ ] 处理损坏或不完整的会话文件

### 3.2 会话列表展示 (FR-002)

**需求描述**: 显示所有可用的会话概览列表

**显示字段**:
- 会话 ID
- 开始时间
- 持续时间
- 消息数量
- 最后消息摘要
- 项目/工作目录

**交互**:
- 支持上下滚动
- 支持选择会话查看详情

**验收标准**:
- [ ] 列表加载时间 < 2s（1000 个会话）
- [ ] 支持键盘导航
- [ ] 支持鼠标点击选择

### 3.3 会话详情展示 (FR-003)

**需求描述**: 显示单个会话的完整对话历史

**显示内容**:
- 用户输入（带时间戳）
- Claude 输出（带时间戳）
- 工具调用（展开/折叠）
  - 工具名称
  - 调用参数
  - 返回结果
  - 执行时长
- 子工程师数据（如适用）

**UI 元素**:
- 时间线视图
- 不同消息类型的视觉区分
- 代码高亮显示

**验收标准**:
- [ ] 按时间顺序展示
- [ ] 长消息支持折叠/展开
- [ ] 代码块语法高亮

### 3.4 搜索和过滤 (FR-004)

**需求描述**: 支持按关键词、时间范围、工具类型等条件搜索会话

**搜索维度**:
- 消息内容（用户输入 + Claude 输出）
- 工具名称
- 时间范围
- 文件路径

**验收标准**:
- [ ] 搜索响应时间 < 500ms
- [ ] 支持正则表达式
- [ ] 高亮匹配结果

### 3.5 提示词重建 (FR-005)

**需求描述**: 从历史记录中提取提示词并生成可复用的格式

**功能**:
- 选择一段对话历史
- 提取用户输入和 Claude 输出
- 生成包含上下文的提示词模板
- 复制到剪贴板或保存为文件

**验收标准**:
- [ ] 保留对话上下文
- [ ] 支持选择消息范围
- [ ] 生成的提示词格式规范

### 3.6 数据导出 (FR-006)

**需求描述**: 将会话历史导出为标准格式

**支持格式**:
- Markdown（带格式化）
- JSON（原始数据）
- 文本文件（纯文本）

**验收标准**:
- [ ] 导出的 Markdown 可读性强
- [ ] JSON 数据结构完整
- [ ] 支持批量导出

---

## 4. 非功能需求

### 4.1 性能需求

| 指标 | 要求 |
|------|------|
| 会话列表加载时间 | < 2s (1000 个会话) |
| 搜索响应时间 | < 500ms |
| 内存占用 | < 500MB (加载 1000 个会话) |
| UI 渲染帧率 | 60 FPS |

### 4.2 可用性需求

- 支持 Linux/macOS/Windows
- 支持主流终端
- 键盘快捷键直观易记

### 4.3 可维护性需求

- 代码模块化，易于扩展
- 完善的错误处理
- 清晰的日志输出

### 4.4 兼容性需求

- 兼容 Claude Code 的会话数据格式
- 支持 Python 3.10+
- 支持主流终端类型

---

## 5. 技术方案

### 5.1 技术栈

| 层级 | 技术选择 | 理由 |
|------|---------|------|
| **开发语言** | Go 1.21+ | 编译为单一二进制，无运行时依赖，分发简单 |
| **TUI 框架** | Bubbletea | Go 生态主流 TUI 框架，支持丰富的交互界面 |
| **数据读取** | 直接解析 JSONL | 读取 Claude Code 原生存储格式，无需中间层 |
| **配置管理** | TOML | 人类可读的配置格式 |
| **CLI 框架** | Cobra | Go 生态标准 CLI 框架 |

### 5.2 架构设计

```
┌─────────────────────────────────────────────────────────┐
│                      CC History CLI                     │
├─────────────────────────────────────────────────────────┤
│  ┌───────────────┐  ┌───────────────┐  ┌──────────────┐ │
│  │ Session List  │  │Session Detail │  │  Search Bar  │ │
│  │  View (TUI)   │  │  View (TUI)   │  │    (TUI)     │ │
│  └───────────────┘  └───────────────┘  └──────────────┘ │
│  ┌──────────────────────────────────────────────────────┐ │
│  │          Plain Text Output (default mode)            │ │
│  └──────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────┤
│  ┌───────────────┐  ┌───────────────┐  ┌──────────────┐ │
│  │  Data Loader  │  │  Data Parser  │  │   Searcher   │ │
│  └───────────────┘  └───────────────┘  └──────────────┘ │
├─────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────┐  ┌────────────────────┐ │
│  │  Claude Code JSONL Files     │  │      Config        │ │
│  │  (~/.claude/projects/...)    │  │                    │ │
│  └──────────────────────────────┘  └────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 5.3 数据模型

直接解析 Claude Code 原生 JSONL 文件，无需中间数据库存储。

#### Session (会话)

```go
type Session struct {
    ID               string    // 会话唯一标识（目录名）
    StartedAt        time.Time // 开始时间
    EndedAt          time.Time // 结束时间
    WorkingDirectory string    // 工作目录
    MessageCount     int       // 消息数量
    TotalTokens      int       // 总 token 数
}
```

#### Message (消息)

```go
type Message struct {
    ID        string     // 消息唯一标识
    SessionID string     // 所属会话
    Timestamp time.Time  // 时间戳
    Role      string     // user | assistant | system
    Content   string     // 内容
    ToolCalls []ToolCall // 工具调用列表
}
```

#### ToolCall (工具调用)

```go
type ToolCall struct {
    ID         string         // 调用唯一标识
    MessageID  string         // 所属消息
    Name       string         // 工具名称
    Arguments  map[string]any // 调用参数
    Result     string         // 返回结果
    DurationMs int            // 执行时长
}
```

### 5.4 核心模块

| 模块 | 职责 |
|------|------|
| **cmd/** | CLI 入口，Cobra 命令路由 |
| **internal/loader** | 扫描和加载 Claude Code JSONL 文件 |
| **internal/parser** | 解析会话数据结构 |
| **internal/searcher** | 内存内关键词搜索 |
| **internal/tui** | Bubbletea TUI 视图组件 |
| **internal/exporter** | 数据导出功能 |
| **internal/prompt** | 提示词重建功能 |

---

## 6. UI/UX 设计

### 6.1 两种输出模式

CC History 提供两种互补的输出模式：

#### 模式一：简洁列表模式（默认）

直接在终端输出会话列表，类似于 bash `history` 命令，无需交互。适合快速查找和脚本集成。

```
$ cc-history
  1  2026-03-29 10:30  Fix auth middleware bug          [45 msgs]  /data/my-project
  2  2026-03-28 15:22  Add unit tests for API           [23 msgs]  /data/my-project
  3  2026-03-28 09:15  Refactor database layer          [67 msgs]  /data/other-project
  4  2026-03-27 14:00  Setup CI/CD pipeline             [12 msgs]  /data/other-project
  ...

$ cc-history 3           # 查看第 3 个会话摘要
$ cc-history --search "auth"  # 搜索包含 auth 的会话
```

#### 模式二：交互式 TUI 模式（`--tui` 或 `-i` 参数）

使用 Bubbletea 的全屏交互界面，支持导航、搜索和详情查看。

```
$ cc-history --tui
```

### 6.2 TUI 主界面布局

```
┌─────────────────────────────────────────────────────────────────┐
│  CC History                                  [🔍 Search] [⚙️]  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────┐ ┌───────────────────────────────────┐  │
│  │   Sessions (234)    │ │  Session: 2026-03-29-abc123       │  │
│  ├─────────────────────┤ │  Started: 2 hours ago             │  │
│  │ 📄 Today            │ │  Messages: 45                     │  │
│  │   └─ Fix auth bug   │ │                                   │  │
│  │ 📄 Yesterday        │ │  ┌─────────────────────────────┐  │  │
│  │   └─ Add tests      │ │  │ 👤 User [10:30:15]          │  │  │
│  │ 📄 This Week        │ │  │ How do I fix the auth       │  │  │
│  │   └─ Refactor API   │ │  │ middleware?                 │  │  │
│  │                     │ │  ├─────────────────────────────┤  │  │
│  │ [Load More...]      │ │  │ 🤖 Assistant [10:30:16]     │  │  │
│  │                     │ │  │ I'll help you fix the auth  │  │  │
│  │                     │ │  │ middleware...               │  │  │
│  │                     │ │  │                             │  │  │
│  └─────────────────────┘ │  │ 🔧 Bash: grep -r "auth"    │  │  │
│                          │  │     (125ms)                  │  │  │
│                          │  └─────────────────────────────┘  │  │
│                          │                                   │  │
│                          │  [Copy Prompt] [Export] [↑/↓ Nav] │  │
│                          └───────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│  [?] Help  [q] Quit  [/] Search  [e] Export  [p] Copy Prompt  │
└─────────────────────────────────────────────────────────────────┘
```

### 6.3 快捷键（TUI 模式）

| 快捷键 | 功能 |
|--------|------|
| `q` | 退出 |
| `/` | 搜索 |
| `n` | 下一个会话 |
| `p` | 上一个会话 |
| `Enter` | 查看会话详情 |
| `Esc` | 返回列表 |
| `Ctrl+C` | 复制提示词 |
| `e` | 导出会话 |

---

## 7. 使用示例

本节展示两种模式的实际终端输出样例，作为开发阶段的 UX 参考基准。

### 7.1 模式一：默认列表模式（Plain Text）

#### 基本命令：列出所有会话

```
$ cc-history

  #    DATE              TITLE (auto-extracted)              MSGS   DIR
  ─────────────────────────────────────────────────────────────────────────
  1    2026-03-29 10:30  Fix auth middleware bug              45    ~/project-a
  2    2026-03-28 15:22  Add unit tests for user API          23    ~/project-a
  3    2026-03-28 09:15  Refactor database layer              67    ~/project-b
  4    2026-03-27 18:04  Setup CI/CD pipeline with GitHub     12    ~/project-b
  5    2026-03-27 14:00  Write PRD for cc-history tool        88    ~/cc-history
  ...
  (showing 20 of 147 sessions, use --all to show all)
```

#### 查看某个会话的摘要

```
$ cc-history 3

Session #3  ·  2026-03-28 09:15  ·  67 messages  ·  ~/project-b
────────────────────────────────────────────────────────────────
[user]       Refactor the database layer to use repository pattern
[assistant]  I'll start by reading the existing database code...
[tool]       Read: src/db/database.go
[tool]       Read: src/db/queries.go
[assistant]  Here's the refactored structure...
[user]       Can you also add connection pooling?
[assistant]  Sure, I'll add pgxpool configuration...
[tool]       Edit: src/db/pool.go
[tool]       Bash: go test ./src/db/...
[assistant]  All tests pass. Here's a summary of the changes...
```

#### 搜索关键词

```
$ cc-history --search "auth"

Found 3 sessions matching "auth":
  #    DATE              TITLE                                MSGS   DIR
  ────────────────────────────────────────────────────────────────────────
  1    2026-03-29 10:30  Fix auth middleware bug              45    ~/project-a
  7    2026-03-25 11:10  Implement JWT auth flow              93    ~/project-a
  12   2026-03-20 16:45  Debug OAuth2 token refresh issue     31    ~/project-a
```

#### 按时间过滤

```
$ cc-history --since 2026-03-28 --until 2026-03-29

  #    DATE              TITLE                                MSGS   DIR
  ────────────────────────────────────────────────────────────────────────
  1    2026-03-29 10:30  Fix auth middleware bug              45    ~/project-a
  2    2026-03-28 15:22  Add unit tests for user API          23    ~/project-a
  3    2026-03-28 09:15  Refactor database layer              67    ~/project-b
```

#### 导出单个会话为 Markdown

```
$ cc-history export 1 --format markdown --output session-auth-fix.md

Exported session #1 to session-auth-fix.md (45 messages, 12.3 KB)
```

#### 重建提示词

```
$ cc-history prompt 1 --range 1-5

Extracted prompt template (messages 1–5 of session #1):

---
[Context]
Working directory: ~/project-a
Session date: 2026-03-29

[User]
Fix auth middleware bug

[Assistant Summary]
Identified issue in middleware/auth.go line 42: token expiry not checked.

[Reconstructed Prompt]
Given the following auth middleware code, fix the token expiry check bug:
<paste relevant code here>
---

Copied to clipboard.
```

---

### 7.2 模式二：交互式 TUI 模式（`--tui` 或 `-i`）

#### 启动 TUI

```
$ cc-history --tui
```

**会话列表主界面**

```
┌─ CC History ─────────────────────────────────── [?] Help  [q] Quit ─┐
│                                                                       │
│  Sessions (147)              [/ Search]                               │
│  ──────────────────────────────────────────────────────────────────  │
│  ▶  #1  2026-03-29 10:30  Fix auth middleware bug         45 msgs    │
│     #2  2026-03-28 15:22  Add unit tests for user API     23 msgs    │
│     #3  2026-03-28 09:15  Refactor database layer         67 msgs    │
│     #4  2026-03-27 18:04  Setup CI/CD pipeline            12 msgs    │
│     #5  2026-03-27 14:00  Write PRD for cc-history tool   88 msgs    │
│     #6  2026-03-26 20:30  Debug memory leak in parser     34 msgs    │
│     #7  2026-03-25 11:10  Implement JWT auth flow         93 msgs    │
│     ...                                                               │
│                                                                       │
│  ────────────────────────────────────────────────────────────────── │
│  [↑↓] Navigate  [Enter] Open  [/] Search  [e] Export  [q] Quit      │
└───────────────────────────────────────────────────────────────────────┘
```

**打开会话详情（按 Enter）**

```
┌─ CC History ─ Session #1: Fix auth middleware bug ── [Esc] Back ────┐
│                                                                       │
│  ◀ Back  │  2026-03-29 10:30  ·  45 messages  ·  ~/project-a        │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                       │
│  👤 User  [10:30:01]                                                 │
│  ╔══════════════════════════════════════════════════════════════╗    │
│  ║ The auth middleware is rejecting valid tokens. The error is  ║    │
│  ║ "token expired" but the tokens are brand new. Please fix it. ║    │
│  ╚══════════════════════════════════════════════════════════════╝    │
│                                                                       │
│  🤖 Claude  [10:30:05]                                               │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ I'll start by reading the auth middleware to find the issue. │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  🔧 Read  middleware/auth.go  [10:30:06]  (12ms)         [▶ expand]  │
│                                                                       │
│  🤖 Claude  [10:30:09]                                               │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ Found the bug on line 42: the code checks `exp < now` but    │    │
│  │ should check `exp <= now`. Here's the fix...                 │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ─────────────────────────────────────────────────────────────────  │
│  [↑↓] Scroll  [e] Export  [p] Copy Prompt  [t] Tool Details  [Esc]  │
└───────────────────────────────────────────────────────────────────────┘
```

**搜索模式（按 `/`）**

```
┌─ CC History ─────────────────────── Search ────────────────────────┐
│                                                                      │
│  Search: auth█                                                       │
│  ─────────────────────────────────────────────────────────────────  │
│  ▶ #1   2026-03-29 10:30  Fix auth middleware bug         45 msgs   │
│    #7   2026-03-25 11:10  Implement JWT auth flow         93 msgs   │
│    #12  2026-03-20 16:45  Debug OAuth2 token refresh      31 msgs   │
│                                                                      │
│  3 results                                                           │
│  ─────────────────────────────────────────────────────────────────  │
│  [↑↓] Navigate  [Enter] Open  [Esc] Cancel search                   │
└──────────────────────────────────────────────────────────────────────┘
```

**展开工具调用详情（按 `t`）**

```
│  🔧 Read  middleware/auth.go  [10:30:06]  (12ms)         [▼ collapse]│
│  ┌─ Tool Input ─────────────────────────────────────────────────┐    │
│  │ {                                                             │    │
│  │   "file_path": "/home/user/project-a/middleware/auth.go"     │    │
│  │ }                                                             │    │
│  └───────────────────────────────────────────────────────────── ┘    │
│  ┌─ Tool Result (truncated) ────────────────────────────────────┐    │
│  │ 1  package middleware                                         │    │
│  │ 2                                                             │    │
│  │ 3  func AuthMiddleware(next http.Handler) http.Handler {      │    │
│  │ 4    return http.HandlerFunc(func(w http.ResponseWriter, ...  │    │
│  │ ...  [press Space to see full output]                         │    │
│  └───────────────────────────────────────────────────────────── ┘    │
```

---

## 8. 实施计划

### 8.1 里程碑

| 里程碑 | 交付物 | 预计时间 |
|--------|--------|---------|
| **M1: 项目初始化** | 项目结构、依赖安装、基础配置 | Day 1 |
| **M2: 数据加载** | 会话数据加载和解析功能 | Day 2-3 |
| **M3: 基础 UI** | 会话列表和详情视图 | Day 4-6 |
| **M4: 搜索功能** | 搜索和过滤功能 | Day 7-8 |
| **M5: 高级功能** | 提示词重建、数据导出 | Day 9-10 |
| **M6: 测试和优化** | 单元测试、性能优化、文档完善 | Day 11-12 |

### 8.2 任务分解

#### Sprint 1: 基础设施 (Day 1-3)
- [ ] 创建 Go 项目结构（go mod init）
- [ ] 实现 Claude Code JSONL 文件扫描器
- [ ] 实现数据解析器（解析 JSONL → Go struct）
- [ ] 实现简洁列表输出（默认模式）

#### Sprint 2: 核心功能 (Day 4-8)
- [ ] 实现会话列表视图
- [ ] 实现会话详情视图
- [ ] 实现搜索功能
- [ ] 实现过滤功能

#### Sprint 3: 高级功能 (Day 9-10)
- [ ] 实现提示词重建
- [ ] 实现数据导出（Markdown/JSON）
- [ ] 实现配置管理

#### Sprint 4: 完善和测试 (Day 11-12)
- [ ] 编写单元测试
- [ ] 性能优化
- [ ] 完善 CLI 文档
- [ ] 用户测试和反馈

---

## 9. 验收标准

### 9.1 功能验收

- [ ] 能够加载并显示 Claude Code 会话列表
- [ ] 能够查看单个会话的完整历史
- [ ] 搜索功能正常工作且响应迅速
- [ ] 能够导出会话为 Markdown/JSON
- [ ] 能够从历史记录重建提示词

### 9.2 性能验收

- [ ] 1000 个会话加载时间 < 2s
- [ ] 搜索响应时间 < 500ms
- [ ] UI 操作响应时间 < 100ms

### 9.3 质量验收

- [ ] 代码测试覆盖率 > 80%
- [ ] 所有核心功能有单元测试
- [ ] CLI 帮助文档完整
- [ ] README 文档完整

---

## 10. 风险与依赖

### 10.1 风险

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Claude Code 数据格式变化 | 高 | 版本检测，向后兼容，parser 层隔离 |
| 性能不达标（大数据量） | 中 | 流式解析 JSONL，按需加载 |
| TUI 兼容性问题 | 低 | 默认简洁模式不依赖 TUI，TUI 为可选模式 |
| Go 编译环境 | 低 | 提供预编译二进制，go install 一键安装 |

### 10.2 依赖

| 依赖项 | 版本要求 | 用途 |
|--------|---------|------|
| Go | 1.21+ | 开发语言，编译为单一二进制 |
| Bubbletea | latest | TUI 框架（交互模式） |
| Cobra | latest | CLI 框架 |
| Lipgloss | latest | 终端样式（Bubbletea 生态） |

> **说明**: 所有依赖均通过 `go mod` 管理，最终产出为无外部运行时依赖的单一可执行文件。

---

## 11. 附录

### 11.1 术语表

| 术语 | 定义 |
|------|------|
| **CC** | Claude Code |
| **TUI** | Terminal User Interface，终端用户界面 |
| **CLI** | Command Line Interface，命令行界面 |
| **PRD** | Product Requirements Document，产品需求文档 |
| **BMAD** | 本项目采用的需求文档方法论 |

### 11.2 参考文档

- [Claude Code 官方文档](https://claude.ai/claude-code)
- [Bubbletea 文档](https://github.com/charmbracelet/bubbletea)
- [Cobra 文档](https://cobra.dev/)
- [Lipgloss 文档](https://github.com/charmbracelet/lipgloss)

### 11.3 变更记录

| 版本 | 日期 | 变更内容 | 作者 |
|------|------|---------|------|
| 1.0.0 | 2026-03-29 | 初始版本 | CC History Engineer |
| 1.1.0 | 2026-03-29 | 根据 LF 审核意见修订：技术栈改为 Go、去除数据库层、增加双模式输出 | CC History Engineer |
| 1.2.0 | 2026-03-29 | 新增完整使用示例（默认模式 & TUI 模式样例输出） | CC History Engineer |

---

**文档结束**
