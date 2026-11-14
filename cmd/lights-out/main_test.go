package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGH stands in for gh on PATH. It logs each call and answers an issue
// with no comments; any other call fails the way gh does.
const fakeGH = `#!/bin/sh
echo "$*" >> "$GH_LOG"
case "$2" in
repos/o/r/issues/1) printf '%s' '{"number":1,"title":"Task","body":"Intent.","url":"https://github.com/o/r/issues/1"}' ;;
repos/o/r/issues/1/comments*) printf '[]' ;;
*) echo "HTTP 404: Not Found (https://api.github.com/$2)" >&2; exit 1 ;;
esac
`

// allowAll is a gitleaks config that allows everything. lightsOut leaves it
// where gitleaks would find it, so the conductor must pin its own rules.
const allowAll = "[extend]\nuseDefault = true\n[allowlist]\nregexes = ['.*']\n"

// lightsOut builds the real executable and runs it with request on stdin.
func lightsOut(t *testing.T, request string) (stdout, stderr, calls string, code int) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "lights-out")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(fakeGH), 0o755); err != nil {
		t.Fatal(err)
	}
	permissive := filepath.Join(dir, ".gitleaks.toml")
	if err := os.WriteFile(permissive, []byte(allowAll), 0o644); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "gh.log")
	cmd := exec.Command(bin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_LOG="+log, "GITLEAKS_CONFIG="+permissive)
	cmd.Stdin = strings.NewReader(request)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatal(err)
	}
	logged, err := os.ReadFile(log)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return out.String(), errOut.String(), string(logged), cmd.ProcessState.ExitCode()
}

func TestExecutableAnswersInJSON(t *testing.T) {
	stdout, stderr, calls, code := lightsOut(t, `{"command":"inspect","repo":"o/r","issue":1}`)
	if code != 0 || stderr != "" || !strings.HasPrefix(stdout, `{"status":"recovered",`) || !strings.Contains(stdout, `"latest":0`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(calls, "api repos/o/r/issues/1/comments?per_page=100&page=1") {
		t.Fatalf("gh calls: %s", calls)
	}
}

func TestExecutableNamesTheFailingToolCall(t *testing.T) {
	stdout, stderr, _, code := lightsOut(t, `{"command":"inspect","repo":"o/r","issue":9}`)
	want := `{"status":"error","kind":"environment","error":"gh api repos/o/r/issues/9: exit status 1: HTTP 404: Not Found (https://api.github.com/repos/o/r/issues/9)"}`
	if code != 1 || stdout != "" || strings.TrimSpace(stderr) != want {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestExecutableRefusesAMalformedRequestBeforeAnyToolRuns(t *testing.T) {
	stdout, stderr, calls, code := lightsOut(t, `{"command":"merge","repo":"o/r","issue":1}`)
	want := `{"status":"error","kind":"refused","error":"command must be inspect, artifact, evidence, checkpoint or assess"}`
	if code != 1 || stdout != "" || calls != "" || strings.TrimSpace(stderr) != want {
		t.Fatalf("code=%d stdout=%s stderr=%s calls=%s", code, stdout, stderr, calls)
	}
}

// The real gitleaks runs here, so a change to its flags or report fails.
func TestExecutableRefusesASecretBeforeItGoesPublic(t *testing.T) {
	key := "AKIA" + "Z3MSJV4WWNKWW5FG" // joined at run time, so this file holds no key-shaped string
	stdout, stderr, calls, code := lightsOut(t, `{"command":"evidence","repo":"o/r","issue":1,"body":"$ env\naws_access_key_id = `+key+` # gitleaks:allow\n"}`)
	want := `{"status":"error","kind":"refused","error":"the comment may hold a secret (aws-access-token on line 2): remove it, rotate it if it is real, and post again"}`
	if code != 1 || stdout != "" || calls != "" || strings.TrimSpace(stderr) != want {
		t.Fatalf("code=%d stdout=%s stderr=%s calls=%s", code, stdout, stderr, calls)
	}
}
