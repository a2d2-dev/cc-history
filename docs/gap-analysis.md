# CC History — 实现差距分析

**分析日期：** 2026-03-29
**基准 PRD：** `_bmad-output/planning-artifacts/prd.md`
**当前分支：** main（最新合并：PR #10, E3-S3.1 export）

---

## 当前实现文件

```
cmd/cc-history/main.go          # CLI 入口
internal/parser/parser.go       # JSONL 解析器
internal/loader/loader.go       # 文件扫描 + Session 检测
internal/export/export.go       # Markdown/JSON 导出
```

---

## 差距总览

| 级别 | 数量 | 说明 |
|------|------|------|
| 🔴 Critical（阻塞核心价值） | 5 | 默认视图未显示消息，核心功能缺失 |
| 🟡 Regression（已开发后退化） | 1 | `--output` flag 被删除 |
| 🟠 Missing-MVP（PRD MVP 范围未实现） | 4 | --all、过滤、上下文参数、时间过滤 |
| ⚪ Not-started（计划中） | 3 | Growth/Vision 阶段 |

---

## 详细差距

### 🔴 Critical

#### GAP-1：默认视图显示文件路径而非消息流

**PRD 要求（FR2）：** `cc-history` 默认输出当前 session 的消息流
**当前行为：** 输出 JSONL 文件路径列表

```bash
# 实际输出（❌ 错误）
/root/.claude/projects/xxx/abc.jsonl
/root/.claude/projects/xxx/def.jsonl

# 期望输出（✅ 正确）
2026-03-29 10:30:01  [user]       The auth middleware is rejecting valid tokens...
2026-03-29 10:30:05  [assistant]  I'll read the auth middleware to find the issue.
```

**根本原因：** `main()` 调用 `loader.ScanJSONL()` 后直接打印路径，未调用 `loader.FindCurrentSession()` + `parser.ParseFile()` 显示消息内容。

**影响：** 产品核心价值无法交付，用户运行 `cc-history` 看不到任何对话内容。

---

#### GAP-2：无消息展示层（output 包缺失）

**PRD 要求（FR10）：** 工具调用格式 `[tool:<名称>]  <关键参数>  (<执行时间>)`
**当前行为：** 无任何消息展示逻辑

**缺失包：** `internal/output/` — 消息格式化渲染（时间戳、角色颜色、内容截断 120 字符、`NO_COLOR` 支持）

---

#### GAP-3：无 pattern 过滤（FR3、FR4）

**PRD 要求：** `cc-history <pattern>` 过滤消息，`-E` 启用正则
**当前行为：** 无过滤功能

**缺失包：** `internal/filter/` — 子串/正则过滤引擎

---

#### GAP-4：无上下文参数 -A/-B/-C（FR5）

**PRD 要求：** 匹配消息前后显示 N 条上下文，行为与 GNU grep 一致
**当前行为：** 无此功能

---

#### GAP-5：无 --all 全量历史模式（FR6、FR7）

**PRD 要求：** `--all` 跨 session 输出，session 分隔行 + `--no-sep` 禁用
**当前行为：** `loader.LoadAllSessions()` 已实现，但未在 CLI 中暴露

---

### 🟡 Regression（功能退化）

#### GAP-6：`--output` 从 export 子命令删除（FR11 退化）

**PRD 要求（FR11）：** `--output <文件路径>` 将导出内容写入文件
**当前行为：** commit `063c9d9` 删除了 `--output` flag，只能输出到 stdout

```bash
# 应该支持（❌ 已删除）
cc-history export --format markdown --output session.md

# 当前只能
cc-history export --format markdown > session.md  # 需要 shell 重定向
```

---

### 🟠 Missing-MVP

#### GAP-7：无 --since/--until 时间过滤（FR8）

**PRD 要求：** `--since YYYY-MM-DD` / `--until YYYY-MM-DD` 限制时间范围

---

#### GAP-8：Cobra 未使用（Architecture 偏差）

**Architecture 文档决策：** 使用 Cobra CLI 框架（`github.com/spf13/cobra`）
**当前实现：** 使用标准库 `flag` 包

**影响：** 子命令结构、`--help` 输出格式、flag 继承等行为与 Architecture 设计不符；NFR10（完整 `--help`）无法达到 Cobra 水准

---

### ⚪ Not-started（预期中）

| FR | 功能 | 阶段 |
|----|------|------|
| FR14 | `cc-history prompt --range` 提示词提取 | Growth（E3-S3.2，PR #11 待合并）|
| FR15–FR17 | TUI 全屏交互界面（`--tui`/`-i`） | Vision（E4） |

---

## 实现状态矩阵

| FR | 需求 | 状态 | 备注 |
|----|------|------|------|
| FR1 | JSONL 扫描 + 解析 | ✅ | parser + loader |
| FR2 | 当前 session 检测 | ⚠️ 部分 | 逻辑存在，CLI 未接入 |
| FR3 | Pattern 过滤 | ❌ | — |
| FR4 | -E 正则 | ❌ | — |
| FR5 | -A/-B/-C 上下文 | ❌ | — |
| FR6 | --all 全量模式 | ❌ | LoadAllSessions 存在未接入 |
| FR7 | --all + pattern 组合 | ❌ | — |
| FR8 | --since/--until | ❌ | — |
| FR9 | --path 覆盖 | ✅ | — |
| FR10 | 工具调用显示格式 | ❌ | 无消息展示层 |
| FR11 | export 命令 | ⚠️ 部分 | --output 已删除 |
| FR12 | Markdown 导出 | ✅ | export.ToMarkdown |
| FR13 | JSON 导出 | ✅ | export.ToJSON |
| FR14 | prompt 提取 | 🔄 | PR #11 待合并 |
| FR15 | TUI 模式 | ⬜ | Vision 阶段 |
| FR16 | TUI 视觉 | ⬜ | Vision 阶段 |
| FR17 | TUI 搜索 | ⬜ | Vision 阶段 |

---

## 修复优先级建议

**P0（立即修复，解锁核心价值）：**
1. GAP-1 + GAP-2：`main()` 接入 `FindCurrentSession` + 消息展示层（`internal/output/`）
2. GAP-6：恢复 `export --output` flag

**P1（完成 MVP）：**
3. GAP-3 + GAP-4：`internal/filter/` 过滤引擎 + -A/-B/-C
4. GAP-5：`--all` 模式 CLI 接入
5. GAP-7：`--since/--until` 时间过滤

**P2（架构合规）：**
6. GAP-8：迁移到 Cobra（可分批进行）

**P3（计划中）：**
7. PR #11 合并（E3-S3.2 prompt）
8. E4 TUI 实现
