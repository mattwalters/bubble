package proxy

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/mattwalters/bubble/internal/state"
)

var dashboardTmpl = template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Bubble Dashboard</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 40px 20px; }
    .container { max-width: 800px; margin: 0 auto; }
    h1 { font-size: 28px; margin-bottom: 8px; font-weight: 700; }
    p.sub { color: #94a3b8; margin-top: 0; margin-bottom: 24px; }
    .card { background: #1e293b; border-radius: 8px; border: 1px solid #334155; padding: 16px 20px; margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center; }
    .card a { color: #38bdf8; text-decoration: none; font-weight: 600; font-size: 16px; }
    .card a:hover { text-decoration: underline; }
    .details { color: #94a3b8; font-size: 13px; margin-top: 4px; }
    .badge { background: #0284c7; color: white; padding: 2px 8px; border-radius: 9999px; font-size: 12px; }
    .empty { color: #64748b; font-style: italic; padding: 20px 0; }
  </style>
</head>
<body>
  <div class="container">
    <h1>🫧 Bubble Dashboard</h1>
    <p class="sub">Active development environments</p>
    {{if .Routes}}
      {{range .Routes}}
      <div class="card">
        <div>
          <div><a href="http://{{.ID}}.localhost:{{$.Port}}/">{{.ID}}.localhost:{{$.Port}}</a></div>
          <div class="details">Upstream: {{.Upstream}} &bull; Worktree: {{.Dir}}</div>
        </div>
        <div>
          <a class="badge" href="http://{{.ID}}.localhost:{{$.Port}}/_bubble/">Services</a>
        </div>
      </div>
      {{end}}
    {{else}}
      <div class="empty">No active bubbles running. Use <code>bubble up &lt;id&gt;</code> to start one.</div>
    {{end}}
  </div>
</body>
</html>`))

var landingTmpl = template.Must(template.New("landing").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Bubble: {{.ID}}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 40px 20px; }
    .container { max-width: 800px; margin: 0 auto; }
    h1 { font-size: 28px; margin-bottom: 8px; }
    p.sub { color: #94a3b8; margin-top: 0; margin-bottom: 24px; }
    .btn { display: inline-block; background: #0284c7; color: white; padding: 10px 18px; border-radius: 6px; text-decoration: none; font-weight: 600; margin-bottom: 24px; }
    .btn:hover { background: #0369a1; }
    .card { background: #1e293b; border-radius: 8px; border: 1px solid #334155; padding: 20px; margin-bottom: 16px; }
    h2 { font-size: 18px; margin-top: 0; margin-bottom: 12px; color: #e2e8f0; }
    table { width: 100%; border-collapse: collapse; }
    th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #334155; font-size: 14px; }
    th { color: #94a3b8; font-weight: 500; }
    a { color: #38bdf8; text-decoration: none; }
    a:hover { text-decoration: underline; }
  </style>
</head>
<body>
  <div class="container">
    <h1>🫧 Bubble: {{.ID}}</h1>
    <p class="sub">Worktree: {{.Dir}} &bull; Tier: {{.Tier}}</p>
    {{if .Upstream}}
    <a class="btn" href="http://{{.ID}}.localhost:{{.Port}}/">Open Web App &rarr;</a>
    {{end}}
    <div class="card">
      <h2>Discovered Service Ports</h2>
      {{if .Ports}}
      <table>
        <thead>
          <tr>
            <th>Service / Port</th>
            <th>Direct Host URL</th>
          </tr>
        </thead>
        <tbody>
          {{range $k, $v := .Ports}}
          <tr>
            <td><strong>{{$k}}</strong></td>
            <td><a href="http://127.0.0.1:{{$v}}" target="_blank">127.0.0.1:{{$v}}</a></td>
          </tr>
          {{end}}
        </tbody>
      </table>
      {{else}}
      <p style="color: #94a3b8; margin: 0;">No compose services configured or running.</p>
      {{end}}
    </div>
  </div>
</body>
</html>`))

var notFoundTmpl = template.Must(template.New("notfound").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Bubble Not Found</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 40px 20px; }
    .container { max-width: 600px; margin: 40px auto; text-align: center; }
    h1 { font-size: 32px; margin-bottom: 8px; color: #ef4444; }
    p { color: #94a3b8; margin-bottom: 24px; font-size: 16px; }
    .active-list { text-align: left; background: #1e293b; border-radius: 8px; border: 1px solid #334155; padding: 16px 20px; }
    .active-list a { color: #38bdf8; text-decoration: none; font-weight: 500; }
  </style>
</head>
<body>
  <div class="container">
    <h1>404: Unknown Bubble</h1>
    <p>Bubble <strong>{{.RequestedID}}</strong> is not active or does not exist.</p>
    {{if .Routes}}
      <div class="active-list">
        <h3 style="margin-top: 0; font-size: 14px; color: #cbd5e1;">Active bubbles:</h3>
        <ul style="margin: 0; padding-left: 20px;">
          {{range .Routes}}
          <li><a href="http://{{.ID}}.localhost:{{$.Port}}/">{{.ID}}.localhost:{{$.Port}}</a></li>
          {{end}}
        </ul>
      </div>
    {{end}}
  </div>
</body>
</html>`))

var deadUpstreamTmpl = template.Must(template.New("dead").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Bubble Upstream Offline</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 40px 20px; }
    .container { max-width: 600px; margin: 40px auto; }
    h1 { font-size: 28px; margin-bottom: 8px; color: #f59e0b; }
    p { color: #94a3b8; font-size: 15px; line-height: 1.5; }
    .box { background: #1e293b; border-left: 4px solid #f59e0b; padding: 16px; border-radius: 4px; margin: 20px 0; font-family: monospace; font-size: 13px; color: #e2e8f0; }
  </style>
</head>
<body>
  <div class="container">
    <h1>🫧 Upstream Offline</h1>
    <p>Bubble <strong>{{.ID}}</strong> is offline or the upstream process at <code>{{.Upstream}}</code> is not responding.</p>
    <p>To diagnose and recover, run:</p>
    <div class="box">
      bubble doctor {{.ID}} --fix<br>
      # or to restart host processes:<br>
      bubble restart {{.ID}}
    </div>
    <p>To inspect process logs:</p>
    <div class="box">
      bubble logs {{.ID}}
    </div>
  </div>
</body>
</html>`))

func renderDashboard(w http.ResponseWriter, routes []state.Route, port int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = dashboardTmpl.Execute(w, map[string]any{
		"Routes": routes,
		"Port":   port,
	})
}

func renderLanding(w http.ResponseWriter, id, dir, tier, upstream string, port int, ports map[string]int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = landingTmpl.Execute(w, map[string]any{
		"ID":       id,
		"Dir":      dir,
		"Tier":     tier,
		"Upstream": upstream,
		"Port":     port,
		"Ports":    ports,
	})
}

func renderNotFound(w http.ResponseWriter, requestedID string, routes []state.Route, port int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = notFoundTmpl.Execute(w, map[string]any{
		"RequestedID": requestedID,
		"Routes":      routes,
		"Port":        port,
	})
}

func renderDeadUpstream(w http.ResponseWriter, id, upstream string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_ = deadUpstreamTmpl.Execute(w, map[string]any{
		"ID":       id,
		"Upstream": upstream,
		"Err":      fmt.Sprintf("Failed to connect to upstream %s", upstream),
	})
}
