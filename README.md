# cc-history

Browse and search Claude Code session history from the command line.

## Install

```bash
go install github.com/a2d2-dev/cc-history/cmd/cc-history@latest
```

Or build from source:

```bash
git clone https://github.com/a2d2-dev/cc-history.git
cd cc-history
go build -o cc-history ./cmd/cc-history
```

## Usage

### List session files

```bash
cc-history
```

Scans `~/.claude/projects/` and prints all JSONL session file paths.

```bash
cc-history --path /custom/sessions/dir
```

Use a custom session directory.

### Export a session

Export the current session as Markdown:

```bash
cc-history export
cc-history export --format markdown
```

Export as JSON:

```bash
cc-history export --format json
```

Export a specific session by ID:

```bash
cc-history export --session <session-id> --format markdown
cc-history export --session <session-id> --format json
```

Write to a file using shell redirection:

```bash
cc-history export --format markdown > session.md
cc-history export --session abc123 --format json > session.json
```

**Markdown output** includes:
- Timestamps for each message
- User and assistant message text
- Tool call names, arguments (pretty-printed JSON), and results

**JSON output** mirrors the internal data model exactly — suitable for programmatic processing.

**Session detection** (when `--session` is omitted):
1. If `CLAUDE_SESSION_ID` is set, that session is used.
2. Otherwise, the most recently modified session file is used.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--path` | `~/.claude/projects` | Session directory |
| `--version` | — | Print version and exit |

#### export subcommand

| Flag | Default | Description |
|------|---------|-------------|
| `--session` | current session | Session ID to export |
| `--format` | `markdown` | Output format: `markdown` or `json` |

## Session ID

Each Claude Code session has a UUID. You can find it:

- In the JSONL filename under `~/.claude/projects/`
- Via the `CLAUDE_SESSION_ID` environment variable (set automatically by Claude Code)
- In the first line of any session JSONL file (`sessionId` field)
