// Package export formats parsed sessions as Markdown, JSON, or HTML.
package export

import (
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

// htmlPageTmpl is the outer page shell (title, CSS, JS).
const htmlPageTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Session: {{.ID}}</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;font-size:15px;line-height:1.6;background:#f6f8fa;color:#24292e;padding:24px}
h1{font-size:20px;font-weight:600;margin-bottom:20px;color:#1b1f23;border-bottom:1px solid #d1d5da;padding-bottom:12px}
h1 span{font-family:monospace;font-size:16px;color:#586069}
.messages{display:flex;flex-direction:column;gap:16px}
.msg{border-radius:8px;padding:16px 20px;max-width:100%;position:relative}
.msg-user{background:#fff;border:1px solid #d1d5da}
.msg-assistant{background:#f0f7ff;border:1px solid #c8e1ff}
.msg-system{background:#fffbdd;border:1px solid #f0e68c;font-size:13px}
.msg-header{display:flex;align-items:center;gap:10px;margin-bottom:10px;font-size:12px;color:#586069}
.role-badge{display:inline-block;padding:2px 8px;border-radius:12px;font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.5px}
.badge-user{background:#dbeafe;color:#1e40af}
.badge-assistant{background:#d1fae5;color:#065f46}
.badge-system{background:#fef3c7;color:#92400e}
.msg-text{white-space:pre-wrap;word-break:break-word}
.tool-section{margin-top:12px}
details{border:1px solid #d1d5da;border-radius:6px;margin-top:8px;background:#fff}
details[open]{border-color:#0366d6}
summary{padding:8px 12px;cursor:pointer;font-size:13px;font-weight:600;color:#0366d6;list-style:none;display:flex;align-items:center;gap:6px;user-select:none}
summary::-webkit-details-marker{display:none}
summary::before{content:"▶";font-size:10px;transition:transform .15s}
details[open] summary::before{transform:rotate(90deg)}
.tool-name{font-family:monospace;background:#e8f4fd;padding:1px 6px;border-radius:4px;color:#0550ae}
.tool-id{font-family:monospace;font-size:11px;color:#6a737d}
.tool-body{padding:12px;border-top:1px solid #d1d5da}
.tool-label{font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.5px;color:#6a737d;margin-bottom:4px}
.tool-duration{font-size:11px;color:#6a737d;margin-top:6px}
.code-block{background:#f6f8fa;border-radius:4px;padding:10px 12px;overflow-x:auto;font-family:"SFMono-Regular",Consolas,"Liberation Mono",Menlo,monospace;font-size:13px;line-height:1.5;white-space:pre}
.error-block{background:#fff5f5;border:1px solid #feb2b2;border-radius:4px;padding:10px 12px;font-family:monospace;font-size:13px;color:#c53030;white-space:pre;overflow-x:auto}
/* JSON syntax highlighting */
.jk{color:#24292e;font-weight:600}
.js{color:#032f62}
.jn{color:#005cc5}
.jb{color:#e36209}
.jl{color:#6f42c1}
</style>
</head>
<body>
<h1>Session <span>{{.ID}}</span></h1>
<div class="messages">
{{range .Messages}}{{if .Role}}{{template "msg" .}}{{end}}{{end}}
</div>
<script>
function highlightJSON(s){
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
    .replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,function(m){
      if(/^"/.test(m)){return /:$/.test(m)?'<span class="jk">'+m+'</span>':'<span class="js">'+m+'</span>'}
      if(/true|false/.test(m)){return '<span class="jb">'+m+'</span>'}
      if(/null/.test(m)){return '<span class="jl">'+m+'</span>'}
      return '<span class="jn">'+m+'</span>'
    });
}
document.querySelectorAll('.json-hl').forEach(function(el){
  el.innerHTML=highlightJSON(el.textContent);
});
</script>
</body>
</html>
`

// htmlMsgTmpl renders a single message.
const htmlMsgTmpl = `{{define "msg"}}<div class="msg msg-{{.Role}}">
<div class="msg-header"><span class="role-badge badge-{{.Role}}">{{.Role}}</span><span>{{.TimestampStr}}</span>{{if .GitBranch}}<span>⎇ {{.GitBranch}}</span>{{end}}</div>
{{if .Text}}<div class="msg-text">{{.TextEscaped}}</div>{{end}}
{{range .ToolCalls}}{{template "toolcall" .}}{{end}}
</div>{{end}}
`

// htmlToolTmpl renders a single tool call as a collapsible block.
const htmlToolTmpl = `{{define "toolcall"}}<details class="tool-section">
<summary><span class="tool-name">{{.Name}}</span> <span class="tool-id">{{.ID}}</span></summary>
<div class="tool-body">
<div class="tool-label">Arguments</div>
<pre class="code-block json-hl">{{.ArgumentsEscaped}}</pre>
{{if .Result}}
<div class="tool-label" style="margin-top:10px">{{if .IsError}}Error{{else}}Result{{end}}</div>
{{if .IsError}}<pre class="error-block">{{.ResultEscaped}}</pre>{{else}}<pre class="code-block">{{.ResultEscaped}}</pre>{{end}}
{{if .DurationMs}}<div class="tool-duration">Duration: {{printf "%.0f" .DurationMs}} ms</div>{{end}}
{{end}}
</div>
</details>{{end}}
`

// htmlTemplates is parsed once.
var htmlTemplates = template.Must(
	template.New("page").Parse(htmlPageTmpl + htmlMsgTmpl + htmlToolTmpl),
)

// htmlPageData is the data passed to the page template.
type htmlPageData struct {
	ID       string
	Messages []*htmlMsgData
}

type htmlMsgData struct {
	Role         string
	TimestampStr string
	GitBranch    string
	Text         string
	TextEscaped  template.HTML
	ToolCalls    []*htmlToolData
}

type htmlToolData struct {
	ID               string
	Name             string
	ArgumentsEscaped template.HTML
	Result           string
	ResultEscaped    template.HTML
	IsError          bool
	DurationMs       float64
}

// ToHTML writes session as a self-contained HTML document to w.
// Tool call arguments and results are wrapped in collapsible <details> elements.
// JSON arguments receive inline syntax highlighting via an embedded script.
func ToHTML(w io.Writer, session *parser.Session) error {
	page := buildHTMLPageData(session)
	return htmlTemplates.Execute(w, page)
}

func buildHTMLPageData(session *parser.Session) *htmlPageData {
	page := &htmlPageData{ID: session.ID}

	for _, msg := range session.Messages {
		if msg.Role == "" {
			continue
		}

		md := &htmlMsgData{
			Role:         msg.Role,
			TimestampStr: formatHTMLTimestamp(msg.Timestamp),
			GitBranch:    msg.GitBranch,
			Text:         msg.Text,
			TextEscaped:  template.HTML(template.HTMLEscapeString(strings.TrimSpace(msg.Text))), //nolint:gosec
		}

		for _, tc := range msg.ToolCalls {
			td := &htmlToolData{
				ID:               tc.ID,
				Name:             tc.Name,
				ArgumentsEscaped: template.HTML(template.HTMLEscapeString(prettyJSON(tc.Arguments))), //nolint:gosec
				IsError:          tc.IsError,
				DurationMs:       tc.DurationMs,
			}
			if tc.Result != "" {
				result := tc.Result
				const maxResult = 4000
				runes := []rune(result)
				if len(runes) > maxResult {
					result = string(runes[:maxResult]) + "\n… (truncated)"
				}
				td.Result = result
				td.ResultEscaped = template.HTML(template.HTMLEscapeString(result)) //nolint:gosec
			}
			md.ToolCalls = append(md.ToolCalls, td)
		}

		page.Messages = append(page.Messages, md)
	}
	return page
}

func formatHTMLTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

