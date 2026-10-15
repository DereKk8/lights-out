// lint-guard proves that the repository lint configuration rejects each seeded violation.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DereKk8/lights-out/internal/boundary"
	"github.com/DereKk8/lights-out/internal/conductor"
)

func guard() error {
	source, err := os.ReadFile("testdata/lintfixture/violations.go")
	if err != nil {
		return err
	}
	cmd := exec.Command("golangci-lint", "run", "--show-stats=false", "--output.json.path=stdout", "--output.text.path=/dev/null", "./testdata/lintfixture")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return fmt.Errorf("fixture lint must exit 1, got %v: %s", err, stderr.String())
	}
	report, err := boundary.LintFrom(out)
	if err != nil {
		return err
	}
	count := 0
	for i, line := range strings.Split(string(source), "\n") {
		_, linter, ok := strings.Cut(line, "// guard: ")
		if !ok {
			continue
		}
		count++
		found := false
		for _, issue := range report.Issues {
			if issue.Pos.Line == i+1 && issue.FromLinter == strings.TrimSpace(linter) && strings.HasSuffix(issue.Pos.Filename, "testdata/lintfixture/violations.go") {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("lint guard weakened: line %d must fail %s", i+1, linter)
		}
	}
	if count != 6 {
		return fmt.Errorf("expected six guard cases, found %d", count)
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Status   string `json:"status"`
		Rejected int    `json:"rejected"`
	}{"passed", count})
}
func main() {
	if err := guard(); err != nil {
		if writeErr := conductor.WriteError(os.Stderr, err); writeErr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
