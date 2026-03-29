# CC History - Product Requirements Document

## 文档信息

| 字段 | 内容 |
|------|------|
| **项目名称** | CC History |
| **文档版本** | 1.4.0 |
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

- ✅ 默认展示**当前 session** 的对话记录（对话内容流，非 session 列表）
- ✅ 通过 `-A` 参数展示所有 session 的对话记录，**跨 session 按时间排序**，不区分 session 边界
- ✅ 类 grep 的关键词过滤，支持 `-A N`（after）/ `-B N`（before）上下文行参数
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
| **US-001** | 作为一个开发者，我想要直接运行 `cc-history` 就能看到**当前 session 的对话内容**，以便立即回顾正在进行的工作 | P0 |
| **US-002** | 作为一个开发者，我想要用 `cc-history -A` 查看**所有历史对话**（跨 session、按时间排序），以便全局回溯历史工作 | P0 |
| **US-003** | 作为一个开发者，我想要用类 grep 的语法过滤对话内容，以便快速找到包含特定关键词的消息 | P0 |
| **US-004** | 作为一个开发者，我想要用 `-A N` / `-B N` 参数显示匹配消息的前后 N 条上下文，以便理解匹配消息的来龙去脉 | P0 |
| **US-005** | 作为一个开发者，我想要查看工具调用的详细信息（输入、输出、执行时间），以便调试和优化 | P1 |
| **US-006** | 作为一个开发者，我想要从历史记录中提取并重建提示词，以便在新的会话中复用 | P1 |
| **US-007** | 作为一个开发者，我想要导出会话历史为 Markdown/JSON，以便文档化或分享 | P2 |
| **US-008** | 作为一个开发者，我想要通过 TUI 模式交互式浏览历史对话，以便在复杂场景下获得更好的导航体验 | P2 |

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

### 3.2 默认输出：当前 Session 对话流 (FR-002)

**需求描述**: 不带参数运行时，输出**当前 session** 的对话记录，格式为时间线消息流

**自动检测当前 session**:
- 读取 Claude Code 的当前 session 标识（进程环境变量 `CLAUDE_SESSION_ID` 或最近修改的 JSONL 文件）
- 若无法识别当前 session，回退到最近一个 session

**输出格式**（每条消息一行或多行）:
```
[时间戳]  [角色]  消息摘要（或全文）
```

**验收标准**:
- [ ] 默认输出当前 session 消息，按时间顺序
- [ ] 每条消息显示：时间戳、角色（user/assistant/tool）、内容摘要
- [ ] 工具调用显示：工具名 + 关键参数（单行简洁格式）
- [ ] 加载时间 < 500ms

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

### 3.4 全量历史模式 (FR-004-A)

**需求描述**: 通过 `-A` / `--all` 参数，输出**所有 session** 的对话记录，跨 session 按时间排序，不区分 session 边界

**输出特性**:
- 所有消息统一时间线，session 切换处可选打印分隔符（`--- session <id> ---`）
- 支持与过滤参数组合使用

**验收标准**:
- [ ] `-A` 输出跨 session 的时间排序消息流
- [ ] session 边界有轻量标识（可通过 `--no-sep` 关闭）
- [ ] 1000 条消息加载 < 2s

### 3.4 类 grep 过滤与上下文 (FR-004-B)

**需求描述**: 支持类 Unix grep 语法对消息流进行关键词过滤，并支持 `-A N`（after）/ `-B N`（before）显示匹配消息的上下文

**参数设计**:

| 参数 | 含义 |
|------|------|
| `cc-history <pattern>` | 过滤包含 pattern 的消息（当前 session） |
| `cc-history -A <pattern>` | 过滤所有 session 中包含 pattern 的消息 |
| `-A N` / `--after-context N` | 显示匹配消息后 N 条消息 |
| `-B N` / `--before-context N` | 显示匹配消息前 N 条消息 |
| `-C N` / `--context N` | 等价于 `-A N -B N` |
| `-E` / `--regexp` | pattern 使用正则表达式 |

> **注意**：`-A` 有两个含义（all sessions 和 after-context），通过参数位置区分：
> - `cc-history -A` 无 pattern → 全量模式
> - `cc-history -A 3 <pattern>` → after-context 3 行
> - `cc-history --all` 始终为全量模式，避免歧义

**验收标准**:
- [ ] 无 pattern 时输出全部消息，有 pattern 时仅输出匹配行
- [ ] `-A N` 在匹配消息后额外输出 N 条消息
- [ ] `-B N` 在匹配消息前额外输出 N 条消息
- [ ] 上下文组之间用 `--` 分隔（与 grep 行为一致）
- [ ] 支持正则表达式（`-E`）
- [ ] 过滤响应时间 < 500ms（10000 条消息）

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

- 兼容 Claude Code 的会话数据格式（JSONL）
- 支持 Go 1.21+（编译环境要求，运行时无依赖）
- 支持主流终端类型（xterm、iTerm2、tmux 等）

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

### 6.1 三种输出模式

CC History 提供三种互补的输出模式：

#### 模式一：当前 Session 对话流（默认）

不带参数运行，输出当前 session 的消息流，类似 `git log` 的内容流视图。

```
$ cc-history
2026-03-29 10:30:01  [user]       Fix auth middleware bug — the error is "token expired"...
2026-03-29 10:30:05  [assistant]  I'll read the auth middleware to find the issue.
2026-03-29 10:30:06  [tool:Read]  middleware/auth.go  (12ms)
2026-03-29 10:30:09  [assistant]  Found the bug on line 42: exp < now should be exp <= now.
...
```

#### 模式二：全量历史模式（`-A` / `--all`）

跨所有 session 输出，按时间排序，session 切换处有轻量分隔。

```
$ cc-history --all
--- session abc123  2026-03-27 14:00 ---
...
--- session def456  2026-03-28 09:15 ---
...
```

#### 模式三：交互式 TUI 模式（`--tui` 或 `-i` 参数）

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

本节展示三种模式的实际终端输出样例，作为开发阶段的 UX 参考基准。

### 7.1 模式一：默认当前 Session 对话流

#### 查看当前 session 的对话记录

```
$ cc-history

2026-03-29 10:30:01  [user]       The auth middleware is rejecting valid tokens. The
                                  error is "token expired" but the tokens are brand new.
2026-03-29 10:30:05  [assistant]  I'll start by reading the auth middleware to find the issue.
2026-03-29 10:30:06  [tool:Read]  middleware/auth.go  (12ms)
2026-03-29 10:30:09  [assistant]  Found the bug on line 42: the code checks `exp < now`
                                  but should check `exp <= now`. Here's the fix...
2026-03-29 10:30:12  [tool:Edit]  middleware/auth.go  (5ms)
2026-03-29 10:30:13  [tool:Bash]  go test ./...  (1.2s)
2026-03-29 10:30:14  [assistant]  All tests pass. The token expiry fix is complete.
2026-03-29 10:30:20  [user]       Can you also add a unit test specifically for this edge case?
...
(45 messages)
```

#### 过滤当前 session（类 grep）

```
$ cc-history auth

2026-03-29 10:30:01  [user]       The auth middleware is rejecting valid tokens...
2026-03-29 10:30:05  [assistant]  I'll start by reading the auth middleware...
2026-03-29 10:30:06  [tool:Read]  middleware/auth.go  (12ms)
--
2026-03-29 10:31:15  [user]       Add a unit test for the auth edge case.
2026-03-29 10:31:18  [assistant]  I'll add a test for the auth middleware expiry check...
```

#### 带上下文的过滤（`-B 1 -A 2`）

```
$ cc-history -B 1 -A 2 "token expired"

2026-03-29 10:29:58  [assistant]  Let me check the middleware configuration first.   ← before
2026-03-29 10:30:01  [user]       The auth middleware is rejecting valid tokens.     ← match
                                  The error is "token expired" but tokens are new.
2026-03-29 10:30:05  [assistant]  I'll read the auth middleware to find the issue.   ← after
2026-03-29 10:30:06  [tool:Read]  middleware/auth.go  (12ms)                         ← after
```

---

### 7.2 模式二：全量历史模式（`--all` / `-A`）

#### 查看所有 session 的对话记录

```
$ cc-history --all

--- session abc123  2026-03-27 14:00  ~/project-b ---
2026-03-27 14:00:05  [user]       Setup CI/CD pipeline with GitHub Actions
2026-03-27 14:00:09  [assistant]  I'll create a GitHub Actions workflow...
...

--- session def456  2026-03-28 09:15  ~/project-b ---
2026-03-28 09:15:02  [user]       Refactor the database layer to use repository pattern
...

--- session ghi789  2026-03-29 10:30  ~/project-a ---
2026-03-29 10:30:01  [user]       The auth middleware is rejecting valid tokens...
...
```

#### 全量历史过滤（`--all` + pattern）

```
$ cc-history --all "repository pattern"

--- session def456  2026-03-28 09:15  ~/project-b ---
2026-03-28 09:15:02  [user]       Refactor the database layer to use repository pattern
2026-03-28 09:15:08  [assistant]  I'll help you implement the repository pattern...
```

#### 全量历史带上下文

```
$ cc-history --all -C 2 "go test"

--- session def456 ---
2026-03-28 09:45:10  [assistant]  Here's the refactored pool.go...          ← context
2026-03-28 09:45:11  [tool:Edit]  src/db/pool.go  (8ms)                    ← context
2026-03-28 09:45:12  [tool:Bash]  go test ./src/db/...  (2.1s)             ← match
2026-03-28 09:45:14  [assistant]  All tests pass.                           ← context
2026-03-28 09:45:15  [user]       Can you add benchmarks too?               ← context
--
--- session ghi789 ---
2026-03-29 10:30:13  [tool:Bash]  go test ./...  (1.2s)                    ← match
2026-03-29 10:30:14  [assistant]  All tests pass. The fix is complete.      ← context
2026-03-29 10:30:20  [user]       Add a unit test for the edge case.        ← context
```

#### 导出会话为 Markdown

```
$ cc-history export --session ghi789 --format markdown --output session-auth-fix.md

Exported session ghi789 to session-auth-fix.md (45 messages, 12.3 KB)
```

#### 重建提示词

```
$ cc-history prompt --range 1-5

Extracted prompt template (messages 1–5 of current session):

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

### 7.3 模式三：交互式 TUI 模式（`--tui` 或 `-i`）

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

## 11. Epic 列表

### 11.1 Epic 概览

| Epic | 标题 | 目标 |
|------|------|------|
| **E1** | 项目基础设施 & 数据加载 | 建立 Go 项目结构，实现 JSONL 解析，输出当前 session 对话流 |
| **E2** | 类 grep 过滤 & 全量历史模式 | 实现 pattern 过滤、`-A`/`-B`/`-C` 上下文参数、`--all` 全量模式 |
| **E3** | 导出 & 提示词重建 | 会话导出（Markdown/JSON）、提示词提取 |
| **E4** | 交互式 TUI 模式 | 实现 Bubbletea 全屏 TUI 界面 |

---

### 11.2 Epic 1：项目基础设施 & 数据加载

**目标**：搭建 Go 项目骨架，实现 Claude Code JSONL 文件的扫描与解析，并能将会话列表以纯文本形式输出到终端。这是所有后续功能的基础，本 Epic 完成后可独立运行并验证数据读取的正确性。

#### Story 1.1：项目骨架初始化

作为开发者，
我想要一个可编译、可运行的 Go 项目骨架，
以便后续功能都基于统一的项目结构开发。

**验收标准**：
1. `go mod init` 完成，模块名为 `github.com/a2d2-dev/cc-history`
2. 目录结构包含 `cmd/`、`internal/loader`、`internal/parser`
3. `go build ./...` 无报错，产出 `cc-history` 可执行文件
4. `cc-history --version` 输出版本号（初始 `0.1.0`）
5. CI（GitHub Actions）能成功执行 `go build` 和 `go test`

#### Story 1.2：JSONL 文件扫描器

作为开发者，
我想要程序能自动发现 `~/.claude/projects/` 下所有会话 JSONL 文件，
以便后续解析有完整的文件列表。

**验收标准**：
1. 扫描 `~/.claude/projects/**/*.jsonl`，返回所有文件路径列表
2. 支持通过 `--path` 参数覆盖默认目录
3. 文件不存在时给出友好错误提示（非 panic）
4. 1000 个文件扫描完成时间 < 100ms

#### Story 1.3：会话数据解析器

作为开发者，
我想要程序能将 JSONL 文件解析为 Go 结构体（Session / Message / ToolCall），
以便后续展示和搜索功能使用统一的数据模型。

**验收标准**：
1. 正确解析 `user`、`assistant`、`system` 消息
2. 正确解析工具调用（name、arguments、result、duration）
3. 损坏或空文件不会导致程序崩溃，跳过并记录 warning
4. 单元测试覆盖率 > 80%，含边界用例

#### Story 1.4：默认当前 Session 对话流输出

作为开发者，
我想要运行 `cc-history` 时立即看到当前 session 的对话内容流，
以便快速回顾正在进行的工作。

**验收标准**：
1. 自动检测当前 session（优先读取 `CLAUDE_SESSION_ID` 环境变量，否则取最近修改的 JSONL 文件）
2. 输出格式：`[时间戳]  [role]  消息摘要`，工具调用显示工具名 + 关键参数
3. 按时间顺序输出全部消息
4. 加载时间 < 500ms（当前 session）
5. 若无法识别当前 session，回退到最近一个 session 并打印提示

---

### 11.3 Epic 2：类 grep 过滤 & 全量历史模式

**目标**：在基础对话流之上，实现类 Unix grep 的 pattern 过滤与上下文行参数（`-A`/`-B`/`-C`），以及 `--all` 全量历史模式，使工具在命令行下具备强大的历史检索能力。

#### Story 2.1：Pattern 过滤（当前 Session）

作为开发者，
我想要通过 `cc-history <pattern>` 过滤当前 session 的消息，
以便快速找到包含特定关键词的对话。

**验收标准**：
1. `cc-history <pattern>` 仅输出包含 pattern 的消息行
2. 多个匹配组之间用 `--` 分隔（与 grep 行为一致）
3. 支持 `-E` 启用正则表达式
4. 匹配响应时间 < 200ms（当前 session）

#### Story 2.2：上下文参数（`-A`/`-B`/`-C`）

作为开发者，
我想要用 `-A N` / `-B N` / `-C N` 参数显示匹配消息的上下文，
以便理解匹配内容的来龙去脉。

**验收标准**：
1. `-A N` 在每个匹配消息后额外显示 N 条消息
2. `-B N` 在每个匹配消息前额外显示 N 条消息
3. `-C N` 等价于同时设置 `-A N -B N`
4. 上下文组之间用 `--` 分隔
5. 无 pattern 时 `-A`/`-B`/`-C` 参数被忽略（输出全部）

#### Story 2.3：全量历史模式（`--all`）

作为开发者，
我想要通过 `cc-history --all` 查看所有 session 的对话，跨 session 按时间排序，
以便全局回溯历史工作。

**验收标准**：
1. `--all` 输出所有 session 消息，按时间顺序
2. session 切换处打印分隔行：`--- session <id>  <时间>  <目录> ---`
3. 支持 `--no-sep` 禁用分隔行
4. 可与 pattern 和 `-A`/`-B`/`-C` 组合使用
5. 1000 条消息加载 < 2s

---

### 11.4 Epic 3：导出 & 提示词重建

**目标**：实现从历史对话中导出 Markdown/JSON 文件，以及提取和重建提示词的功能，满足文档化和工作流复用需求。

#### Story 3.1：会话导出

作为开发者，
我想要将指定 session 导出为 Markdown 或 JSON 文件，
以便文档化或分享工作过程。

**验收标准**：
1. `cc-history export --session <id> --format markdown --output <文件>` 正常工作
2. `cc-history export --session <id> --format json` 输出完整原始数据
3. Markdown 导出包含：时间、消息气泡、工具调用细节
4. JSON 导出结构与内部数据模型一致
5. 不指定 `--session` 时默认导出当前 session

#### Story 3.2：提示词重建

作为开发者，
我想要通过 `cc-history prompt --range 1-5` 从当前 session 中提取提示词模板，
以便在新会话中复用成功的对话模式。

**验收标准**：
1. `--range start-end` 指定消息序号范围
2. 输出包含：工作目录、日期、用户输入、助手摘要、重建提示词
3. 支持 `--copy` 自动复制到剪贴板
4. 支持 `--output <文件>` 保存为文件

---

### 11.5 Epic 4：交互式 TUI 模式

**目标**：基于 Bubbletea 框架，实现全屏交互式终端界面，提供比 CLI 模式更丰富的浏览和搜索体验，支持消息流导航和工具调用展开。

#### Story 4.1：TUI 框架 & 对话流视图

作为开发者，
我想要通过 `cc-history --tui` 进入全屏 TUI，以可滚动的消息流浏览当前 session，
以便在终端内交互式阅读对话历史。

**验收标准**：
1. `cc-history --tui` 或 `-i` 启动全屏 TUI
2. 默认显示当前 session 消息流，可上下滚动
3. `user` / `assistant` / `tool` 消息视觉区分（颜色/前缀）
4. 工具调用默认折叠，`t` 展开/折叠
5. `q` 退出，`?` 显示帮助

#### Story 4.2：TUI session 切换 & 搜索

作为开发者，
我想要在 TUI 中切换 session 并实时搜索消息，
以便在交互界面中快速定位历史内容。

**验收标准**：
1. `s` 打开 session 选择列表，Enter 切换，Esc 取消
2. `/` 激活搜索模式，输入时实时过滤当前视图
3. `n`/`N` 跳转到下一个/上一个匹配项
4. 搜索词清空后恢复完整消息流

---

## 12. 下一步（Next Steps）

### 12.1 给 UX Expert 的提示词

```
请基于 CC History PRD v1.3.0（位于 docs/prd/CC-History-PRD.md）设计 TUI 交互细节。

重点关注：
1. Epic 3 中 TUI 模式的颜色方案和布局规范（Lipgloss 样式 token）
2. 会话列表与详情视图的键盘导航映射完整清单
3. 工具调用折叠/展开的视觉表达方式
4. 搜索高亮的 UI 规范

请输出：设计决策文档 + Lipgloss 样式参考代码片段
```

### 12.2 给 Architect 的提示词

```
请基于 CC History PRD v1.3.0（位于 docs/prd/CC-History-PRD.md）创建技术架构文档。

重点设计：
1. Go 项目目录结构（cmd/、internal/ 子包划分）
2. JSONL 解析层：Claude Code 会话文件结构分析 & Go 数据模型
3. 流式解析策略（避免一次性加载大量文件到内存）
4. Bubbletea 组件树设计（Model / View / Update 分层）
5. 搜索模块：内存内 BM25 或简单字符串匹配的选型

输出：架构决策文档（ADR 格式）+ 关键模块接口定义
```

---

## 13. 附录

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
| 1.3.0 | 2026-03-29 | 新增 Epic List（E1-E4）含 Story & 验收标准；新增 Next Steps；修正第 4.4 节 Python 引用 | CC History Engineer |
| 1.4.0 | 2026-03-29 | 重新定义核心交互模型：默认输出当前 session 对话流；新增 `--all` 全量历史模式；新增类 grep 过滤与 `-A`/`-B`/`-C` 上下文参数 | CC History Engineer |

---

**文档结束**
