---
workflowType: 'prd'
workflow: 'edit'
classification:
  domain: 'developer-tools'
  projectType: 'cli-tool'
  complexity: 'moderate'
inputDocuments:
  - 'docs/prd/CC-History-PRD.md'
stepsCompleted:
  - 'step-e-01-discovery'
  - 'step-e-01b-legacy-conversion'
  - 'step-e-02-review'
  - 'step-e-03-edit'
lastEdited: '2026-03-29'
editHistory:
  - date: '2026-03-29'
    changes: '从 Legacy 格式转换为 BMAD v6 标准格式；移除 implementation leakage（技术方案章节）；移除 Epic/Sprint 内容（迁移至 Epics 文档）；新增 Executive Summary、Product Scope；重构 FR/NFR 为 SMART 格式'
---

# CC History - 产品需求文档（PRD）

**作者：** LF
**日期：** 2026-03-29

---

## Executive Summary

CC History 是一个 Claude Code 会话历史查看器 CLI 工具，为使用 Claude Code 的开发者提供类 Unix `history` 命令的对话内容检索体验。

**核心差异化：** 以「当前 session 对话流」为默认视图，而非会话列表；支持类 `grep` 语法过滤消息，包含 `-A`/`-B`/`-C` 上下文行参数；通过 `--all` 一键切换到跨 session 全量时间线。

**目标用户：** 日常使用 Claude Code 的开发者，需要快速回溯对话内容、复用提示词、调试工具调用过程。

**目标结果：**
- 让开发者在 10 秒内找到任意历史对话片段
- 无需学习成本——命令行体验与 Unix 工具链一致
- 零安装依赖——单一可执行文件，`go install` 一行完成

---

## Success Criteria

| 指标 | 目标值 | 测量方法 |
|------|--------|---------|
| 当前 session 加载时间 | < 500ms | 本地基准测试，`time cc-history` |
| 全量历史加载时间（1000 条消息） | < 2s | 本地基准测试，`time cc-history --all` |
| Pattern 过滤响应时间 | < 200ms | 本地基准测试，`time cc-history <pattern>` |
| 导出单个会话为 Markdown | < 1s | 本地基准测试 |
| 二进制文件大小 | < 20MB | `ls -lh cc-history` |
| 首次运行无配置即可工作 | 100% | 在已有 Claude Code 安装的机器上测试 |
| 单元测试覆盖率 | ≥ 80% | `go test -cover ./...` |

---

## Product Scope

### MVP（阶段一）

核心价值：让开发者能快速查看和过滤 Claude Code 对话历史。

- 默认输出当前 session 对话流（自动检测当前 session）
- 类 grep pattern 过滤，支持 `-A`/`-B`/`-C` 上下文参数
- `--all` 全量历史模式，跨 session 按时间排序，含 session 分隔符
- 基础正则表达式支持（`-E`）
- 时间范围过滤（`--since`/`--until`）

### Growth（阶段二）

增强生产力工具能力：

- 会话导出（Markdown/JSON 格式）
- 提示词重建（从历史对话提取可复用模板）
- 工具调用详情展示（展开/折叠）

### Vision（阶段三）

完整交互体验：

- 交互式 TUI 模式（`--tui`/`-i`），Bubbletea 全屏界面
- TUI 内 session 切换与实时搜索

---

## User Journeys

### Journey 1：回溯当前工作对话

**场景：** 开发者正在使用 Claude Code 解决问题，想查看本次 session 的完整对话过程。

```
$ cc-history

2026-03-29 10:30:01  [user]       The auth middleware is rejecting valid tokens...
2026-03-29 10:30:05  [assistant]  I'll read the auth middleware to find the issue.
2026-03-29 10:30:06  [tool:Read]  middleware/auth.go  (12ms)
2026-03-29 10:30:09  [assistant]  Found the bug on line 42: exp < now should be exp <= now.
2026-03-29 10:30:12  [tool:Edit]  middleware/auth.go  (5ms)
2026-03-29 10:30:13  [tool:Bash]  go test ./...  (1.2s)
2026-03-29 10:30:14  [assistant]  All tests pass.
```

**关键需求：** 自动识别当前 session，无需指定任何参数。

---

### Journey 2：过滤查找特定内容

**场景：** 开发者记得讨论过某个问题，想找到对应的对话片段。

```
$ cc-history auth

2026-03-29 10:30:01  [user]       The auth middleware is rejecting valid tokens...
2026-03-29 10:30:05  [assistant]  I'll read the auth middleware...
--
2026-03-29 10:31:15  [user]       Add a unit test for the auth edge case.
```

**关键需求：** 仅输出匹配行，用 `--` 分隔不连续的匹配组。

---

### Journey 3：查看匹配消息的上下文

**场景：** 开发者找到匹配消息后，需要了解前后的对话背景。

```
$ cc-history -B 1 -A 2 "token expired"

2026-03-29 10:29:58  [assistant]  Let me check the middleware first.          ← before
2026-03-29 10:30:01  [user]       The error is "token expired"...             ← match
2026-03-29 10:30:05  [assistant]  I'll read the middleware.                   ← after
2026-03-29 10:30:06  [tool:Read]  middleware/auth.go  (12ms)                  ← after
```

**关键需求：** `-A N`（after）`-B N`（before）`-C N`（前后各 N 条），行为与 GNU grep 一致。

---

### Journey 4：全量历史检索

**场景：** 开发者需要在所有历史会话中查找某个主题。

```
$ cc-history --all "repository pattern"

--- session def456  2026-03-28 09:15  ~/project-b ---
2026-03-28 09:15:02  [user]       Refactor the database layer to use repository pattern
2026-03-28 09:15:08  [assistant]  I'll help you implement the repository pattern...
```

**关键需求：** `--all` 跨 session 输出，session 切换处打印分隔行。

---

### Journey 5：导出会话记录（Growth）

**场景：** 开发者想把一次会话整理成文档分享给团队。

```
$ cc-history export --format markdown --output session-auth-fix.md
Exported current session to session-auth-fix.md (45 messages, 12.3 KB)
```

---

### Journey 6：提示词重建（Growth）

**场景：** 开发者想从历史对话中提取成功的提示词模板复用。

```
$ cc-history prompt --range 1-5 --copy
Extracted prompt template (messages 1-5), copied to clipboard.
```

---

### Journey 7：TUI 交互浏览（Vision）

**场景：** 开发者需要深度浏览历史，偏好全屏交互界面。

```
$ cc-history --tui
[全屏 Bubbletea 界面，可滚动消息流，支持 session 切换和实时搜索]
```

---

## Functional Requirements

### MVP 功能需求

**FR1：** 用户可不带任何参数运行 `cc-history`，输出当前活跃 session 的完整消息流，按时间顺序排列，每条消息显示时间戳、角色（user/assistant/tool）和内容摘要。

**FR2：** 当 `CLAUDE_SESSION_ID` 环境变量存在时，`cc-history` 使用该变量值定位当前 session；否则，使用最近修改的 JSONL 文件所属 session；若均无法确定，输出最近一条 session 并打印提示信息。

**FR3：** 用户可通过 `cc-history <pattern>` 过滤当前 session 消息，仅输出内容包含 `pattern` 的消息行；多个不连续匹配组之间用 `--` 行分隔。

**FR4：** 用户可通过 `-E` 参数启用正则表达式模式，pattern 使用 Go 标准正则语法；正则解析错误时输出明确错误信息并退出，退出码非零。

**FR5：** 用户可通过 `-A N`（after）、`-B N`（before）、`-C N`（context）参数在每个匹配消息周围输出额外 N 条消息；三个参数可组合使用；无 pattern 时这三个参数被忽略。

**FR6：** 用户可通过 `cc-history --all`（或 `-A` 无数字参数）输出所有 session 的消息，跨 session 按时间升序排列；每个 session 开始处打印分隔行 `--- session <id>  <datetime>  <workdir> ---`；`--no-sep` 参数禁用分隔行。

**FR7：** `--all` 模式支持与 `<pattern>`、`-A`/`-B`/`-C` 组合使用，行为与单 session 模式一致。

**FR8：** 用户可通过 `--since YYYY-MM-DD` 和 `--until YYYY-MM-DD` 参数限制输出时间范围；日期格式错误时输出明确错误信息；两个参数可独立使用或组合使用。

**FR9：** 用户可通过 `--path <dir>` 参数覆盖默认的 Claude Code 数据目录（默认：`~/.claude/projects/`）；路径不存在时输出友好错误提示。

**FR10：** 工具调用消息显示格式为 `[tool:<工具名>]  <关键参数>  (<执行时间>)`，单行简洁输出。

### Growth 功能需求

**FR11：** 用户可通过 `cc-history export --format <markdown|json>` 导出当前 session；`--session <id>` 参数指定其他 session；`--output <文件路径>` 指定输出文件，默认输出到 stdout。

**FR12：** Markdown 导出内容包含：会话元信息（时间、工作目录）、每条消息的角色标记和完整内容、工具调用的输入参数和返回结果。

**FR13：** JSON 导出内容为完整会话数据结构，与内部 Session/Message/ToolCall 数据模型一致。

**FR14：** 用户可通过 `cc-history prompt --range <start>-<end>` 从当前 session 提取消息范围生成提示词模板；`--copy` 参数自动复制到剪贴板；`--output <文件>` 保存为文件。

### Vision 功能需求

**FR15：** 用户可通过 `cc-history --tui`（或 `-i`）进入全屏交互式 TUI；默认显示当前 session 消息流，支持上下滚动；`q` 退出，`?` 显示帮助。

**FR16：** TUI 模式中，user/assistant/tool 消息通过颜色和前缀视觉区分；工具调用默认折叠，`t` 键展开/折叠详情。

**FR17：** TUI 模式中，`s` 键打开 session 选择列表，Enter 切换 session，Esc 取消；`/` 激活搜索，输入时实时过滤消息，`n`/`N` 跳转匹配项。

---

## Non-Functional Requirements

**NFR1：** 当前 session 加载并输出第一条消息的时间 ≤ 500ms，在 macOS/Linux 本地文件系统上以 `time cc-history` 测量。

**NFR2：** `--all` 模式加载 1000 条消息的总时间 ≤ 2s，以 `time cc-history --all` 在本地测试数据集上测量。

**NFR3：** pattern 过滤（含正则）在 10000 条消息中的响应时间 ≤ 200ms，以 `time cc-history <pattern>` 在本地测试数据集上测量。

**NFR4：** 程序内存峰值 ≤ 200MB（加载 1000 条消息时），以 `/usr/bin/time -v` 或 `go tool pprof` 测量。

**NFR5：** 编译产出单一静态链接二进制文件，在目标平台（Linux amd64、macOS amd64、macOS arm64）上无需任何运行时依赖即可执行，以全新机器（无 Go 环境）验证。

**NFR6：** 二进制文件大小 ≤ 20MB（strip 后），以 `ls -lh cc-history` 测量。

**NFR7：** 单元测试覆盖率 ≥ 80%（`go test -cover ./...`），涵盖 JSONL 解析、pattern 过滤、上下文参数逻辑的边界用例。

**NFR8：** JSONL 格式解析错误（损坏文件、不完整记录）不导致程序 panic，跳过损坏记录并向 stderr 输出 warning，退出码为 0（文件损坏为 warning，非 error）。

**NFR9：** 支持 Linux（amd64）、macOS（amd64/arm64）操作系统；支持 xterm、iTerm2、tmux 等主流终端类型，在 `TERM=xterm-256color` 环境下验证颜色输出。

---

## Risk & Dependencies

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Claude Code JSONL 格式变更 | 高 — 解析失败 | Parser 层隔离，添加格式版本检测，版本变更时输出警告 |
| 无法通过环境变量检测当前 session | 中 — 需回退策略 | 使用最近修改文件作为回退，并在 stderr 提示检测方式 |
| 大量历史文件导致加载超时 | 中 — 性能降级 | 流式解析 JSONL，按需加载，`--all` 模式分页输出 |

| 依赖项 | 要求 | 用途 |
|--------|------|------|
| Go | 1.21+ | 编译环境（运行时无依赖） |
| Bubbletea | latest | TUI 框架（Vision 阶段，可选） |
| Cobra | latest | CLI 参数解析 |
| Lipgloss | latest | 终端颜色样式（Bubbletea 生态） |

---

## Next Steps

### UX Expert Prompt

基于此 PRD，设计 CC History 的 TUI 交互规范（Vision 阶段）：颜色方案（Lipgloss tokens）、键盘导航完整映射、工具调用折叠/展开视觉规范、搜索高亮样式。输入：本 PRD `_bmad-output/planning-artifacts/prd.md`。

### Architect Prompt

基于此 PRD，创建 CC History 架构文档：Go 项目目录结构（`cmd/`/`internal/` 子包划分）、JSONL 解析数据模型、流式解析策略（避免全量加载）、CLI 参数路由（Cobra）、TUI 组件树（Bubbletea Model/View/Update）、搜索模块选型。输入：本 PRD `_bmad-output/planning-artifacts/prd.md`。
