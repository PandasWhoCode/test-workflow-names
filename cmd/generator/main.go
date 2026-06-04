// Package main implements a static site generator that produces an index.html
// page listing GitHub Actions workflows with their run counts and a time-aware
// color scheme (light 06:00–17:59 UTC, dark 18:00–05:59 UTC).
package main

import (
	_ "embed"
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

// pageTmplSrc holds the HTML template loaded from public/index.html.tmpl.
//
//go:embed public/index.html.tmpl
var pageTmplSrc string

// pageTmpl is the parsed HTML template for the static site.
var pageTmpl = template.Must(template.New("page").Parse(pageTmplSrc))
