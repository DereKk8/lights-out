package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutableRefusesMalformedRequest(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "lights-out")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader(`{"command":"merge","repo":"o/r","issue":1}`)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("exit: %v", err)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), `"status":"error"`) || !strings.Contains(stderr.String(), "command must be inspect, checkpoint or assess") {
		t.Fatalf("stdout=%s stderr=%s", &stdout, &stderr)
	}
}
