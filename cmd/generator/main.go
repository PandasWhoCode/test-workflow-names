// Package main implements a static site generator that produces an index.html
// page listing GitHub Actions workflows with their run counts and a time-aware
// color scheme (light 06:00–17:59 UTC, dark 18:00–05:59 UTC).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const apiBase = "https://api.github.com"

// apiClient wraps authenticated calls to the GitHub REST API.
type apiClient struct {
	token string
	repo  string // "owner/repo"
	hc    *http.Client
}

func newAPIClient(token, repo string) *apiClient {
	return &apiClient{token: token, repo: repo, hc: &http.Client{Timeout: 30 * time.Second}}
}

func (c *apiClient) get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, apiBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API %s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type ghWorkflow struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"` // e.g. ".github/workflows/ci.yml"
}

type ghWorkflowsResp struct {
	Workflows []ghWorkflow `json:"workflows"`
}

type ghRunsResp struct {
	TotalCount int `json:"total_count"`
}

// WorkflowRow is one row in the rendered table.
type WorkflowRow struct {
	Filename string
	Name     string
	RunCount int
}

// PageData is the data passed to the HTML template.
type PageData struct {
	Repo      string
	Scheme    string
	Rows      []WorkflowRow
	UpdatedAt string
}

// currentScheme returns "light" between 06:00–17:59 UTC, "dark" otherwise.
func currentScheme() string {
	h := time.Now().UTC().Hour()
	if h >= 6 && h < 18 {
		return "light"
	}
	return "dark"
}

func fetchWorkflows(c *apiClient) ([]WorkflowRow, error) {
	var wfResp ghWorkflowsResp
	if err := c.get(fmt.Sprintf("/repos/%s/actions/workflows", c.repo), &wfResp); err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	rows := make([]WorkflowRow, 0, len(wfResp.Workflows))
	for _, wf := range wfResp.Workflows {
		var runsResp ghRunsResp
		if err := c.get(
			fmt.Sprintf("/repos/%s/actions/workflows/%d/runs?per_page=1", c.repo, wf.ID),
			&runsResp,
		); err != nil {
			fmt.Fprintf(os.Stderr, "warning: run count for %q: %v\n", wf.Name, err)
		}
		rows = append(rows, WorkflowRow{
			Filename: filepath.Base(wf.Path),
			Name:     wf.Name,
			RunCount: runsResp.TotalCount,
		})
	}
	return rows, nil
}

func generate(data PageData, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return pageTmpl.Execute(f, data)
}

func main() {
	repo := flag.String("repo", "", "owner/repo (defaults to $GITHUB_REPOSITORY)")
	scheme := flag.String("scheme", "", "color scheme: light|dark (default: auto from UTC time)")
	output := flag.String("output", "docs/index.html", "output HTML file path")
	flag.Parse()

	if *repo == "" {
		*repo = os.Getenv("GITHUB_REPOSITORY")
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "error: --repo or GITHUB_REPOSITORY is required")
		os.Exit(1)
	}

	if *scheme == "" {
		*scheme = currentScheme()
	}

	token := os.Getenv("GITHUB_TOKEN")
	client := newAPIClient(token, *repo)

	rows, err := fetchWorkflows(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	data := PageData{
		Repo:      *repo,
		Scheme:    *scheme,
		Rows:      rows,
		UpdatedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	}

	if err := generate(data, *output); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s (scheme: %s, %d workflows)\n", *output, *scheme, len(rows))
}

// pageTmpl is the HTML template for the static site.
var pageTmpl = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="en" class="{{.Scheme}}">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Workflows — {{.Repo}}</title>
  <style>
    /* ── Light scheme ── */
    html.light {
      --bg:         #f6f8fa;
      --surface:    #ffffff;
      --surface2:   #f0f2f5;
      --fg:         #1f2328;
      --fg-muted:   #636c76;
      --border:     #d0d7de;
      --th-bg:      #eaeef2;
      --row-even:   #f6f8fa;
      --accent:     #0969da;
    }
    /* ── Dark scheme ── */
    html.dark {
      --bg:         #0d1117;
      --surface:    #161b22;
      --surface2:   #1c2128;
      --fg:         #e6edf3;
      --fg-muted:   #848d97;
      --border:     #30363d;
      --th-bg:      #21262d;
      --row-even:   #1c2128;
      --accent:     #58a6ff;
    }
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: var(--bg);
      color: var(--fg);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
      font-size: 1rem;
      line-height: 1.5;
      padding: 2.5rem 1.5rem;
      max-width: 860px;
      margin: 0 auto;
      transition: background-color 0.25s ease, color 0.25s ease;
    }
    h1 {
      font-size: 1.6rem;
      font-weight: 600;
      margin-bottom: 0.25rem;
    }
    .repo {
      color: var(--fg-muted);
      font-size: 0.9rem;
      margin-bottom: 2rem;
    }
    .card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 6px;
      overflow: hidden;
    }
    table {
      width: 100%;
      border-collapse: collapse;
    }
    thead th {
      background: var(--th-bg);
      padding: 0.65rem 1rem;
      text-align: left;
      font-size: 0.825rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      border-bottom: 1px solid var(--border);
    }
    thead th:last-child { text-align: right; }
    tbody tr + tr td { border-top: 1px solid var(--border); }
    tbody tr:nth-child(even) td { background: var(--row-even); }
    td {
      padding: 0.7rem 1rem;
      font-size: 0.9rem;
      vertical-align: middle;
    }
    td:first-child {
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
      font-size: 0.82rem;
      color: var(--accent);
    }
    td:last-child {
      text-align: right;
      font-variant-numeric: tabular-nums;
      color: var(--fg-muted);
    }
    .empty { padding: 1.5rem; color: var(--fg-muted); text-align: center; }
    .updated {
      color: var(--fg-muted);
      font-size: 0.78rem;
      margin-top: 0.75rem;
      text-align: right;
    }
  </style>
  <script>
    // Re-apply color scheme based on the visitor's current UTC time of day.
    // Light: 06:00–17:59 UTC  |  Dark: 18:00–05:59 UTC
    (function () {
      var h = new Date().getUTCHours();
      document.documentElement.className = (h >= 6 && h < 18) ? 'light' : 'dark';
    }());
  </script>
</head>
<body>
  <h1>GitHub Actions Workflows</h1>
  <p class="repo">{{.Repo}}</p>
  <div class="card">
    <table>
      <thead>
        <tr>
          <th>File</th>
          <th>Workflow Name</th>
          <th>Runs</th>
        </tr>
      </thead>
      <tbody>
        {{- if .Rows}}
        {{- range .Rows}}
        <tr>
          <td>{{.Filename}}</td>
          <td>{{.Name}}</td>
          <td>{{.RunCount}}</td>
        </tr>
        {{- end}}
        {{- else}}
        <tr><td colspan="3" class="empty">No workflows found.</td></tr>
        {{- end}}
      </tbody>
    </table>
  </div>
  <p class="updated">Last updated: {{.UpdatedAt}}</p>
</body>
</html>
`))
