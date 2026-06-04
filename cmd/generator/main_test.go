package main

import (
"bytes"
"strings"
"testing"
)

func TestCurrentScheme(t *testing.T) {
s := currentScheme()
if s != "light" && s != "dark" {
t.Errorf("unexpected scheme %q, want light or dark", s)
}
}

func TestPageTemplate(t *testing.T) {
tests := []struct {
name   string
data   PageData
checks []string
}{
{
name: "light scheme with rows",
data: PageData{
Repo:      "owner/repo",
Scheme:    "light",
Rows:      []WorkflowRow{{Filename: "ci.yml", Name: "CI", RunCount: 42}},
UpdatedAt: "2026-01-01 06:00:00 UTC",
},
checks: []string{
`class="light"`,
`<td>ci.yml</td>`,
`<td>CI</td>`,
`<td>42</td>`,
`owner/repo`,
`2026-01-01 06:00:00 UTC`,
},
},
{
name: "dark scheme with no rows",
data: PageData{
Repo:      "org/project",
Scheme:    "dark",
Rows:      nil,
UpdatedAt: "2026-01-01 20:00:00 UTC",
},
checks: []string{
`class="dark"`,
`No workflows found.`,
},
},
}
for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
var buf bytes.Buffer
if err := pageTmpl.Execute(&buf, tc.data); err != nil {
t.Fatalf("template error: %v", err)
}
out := buf.String()
for _, want := range tc.checks {
if !strings.Contains(out, want) {
t.Errorf("expected output to contain %q", want)
}
}
})
}
}
