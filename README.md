# test-workflow-names

A GitHub Pages site that displays this repository's GitHub Actions workflows — their file names, display names, and total run counts — with a color scheme that shifts automatically between light and dark based on UTC time of day.

## How it works

Three GitHub Actions workflows keep the site up to date:

| Workflow file | Purpose | Trigger |
|---|---|---|
| `update-table.yml` | Fetches workflow run counts via the GitHub API, regenerates `docs/index.html`, and triggers `publish.yml` | Hourly schedule + `workflow_dispatch` |
| `update-scheme.yml` | Regenerates the page at the exact moment the color scheme switches (06:00 UTC → light, 18:00 UTC → dark) and triggers `publish.yml` | Daily cron at 06:00 & 18:00 UTC + `workflow_dispatch` |
| `publish.yml` | Deploys the `docs/` folder to GitHub Pages | `workflow_dispatch` only (called by the above workflows via `benc-uk/workflow-dispatch`) |

The color scheme is also applied client-side via a small JavaScript snippet, so visitors always see the correct scheme regardless of when the page was last rebuilt.

## Security

- All `uses:` references are pinned to immutable full commit SHAs (with version tag comments for readability).
- Every job uses [`step-security/harden-runner`](https://github.com/step-security/harden-runner) (audit mode) as its first step.
- `workflow_run` is not used; `update-table.yml` and `update-scheme.yml` explicitly dispatch `publish.yml` via [`benc-uk/workflow-dispatch`](https://github.com/benc-uk/workflow-dispatch).

## Repository structure

```
cmd/generator/
  main.go                  # CLI: fetches GitHub API data and renders the site
  main_test.go             # unit tests
  public/
    index.html.tmpl        # HTML template (embedded at build time via //go:embed)
docs/
  index.html               # generated output — deployed to GitHub Pages
.github/workflows/
  update-table.yml
  update-scheme.yml
  publish.yml
```

## Setup

1. **Enable GitHub Pages** in *Settings → Pages → Source → GitHub Actions*.
2. Ensure `GITHUB_TOKEN` has **read access to Actions** (default for repos where Actions is enabled).
3. Run either `Update Workflow Table` or `Update Color Scheme` manually (via *Actions → workflow → Run workflow*) to generate the first `docs/index.html`, which triggers `Publish to GitHub Pages` automatically.

## Local development

```sh
# Generate docs/index.html using the live GitHub API
export GITHUB_TOKEN=<your-pat>
go run ./cmd/generator --repo PandasWhoCode/test-workflow-names

# Override color scheme
go run ./cmd/generator --repo PandasWhoCode/test-workflow-names --scheme dark

# Custom output path
go run ./cmd/generator --repo PandasWhoCode/test-workflow-names --output /tmp/preview.html
```

| Flag | Default | Description |
|---|---|---|
| `--repo` | `$GITHUB_REPOSITORY` | Repository in `owner/repo` format |
| `--scheme` | auto (UTC hour) | Color scheme: `light` or `dark` |
| `--output` | `docs/index.html` | Output file path |

