package boundary

import (
	"bytes"
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxComment is GitHub's limit on one comment, in characters.
const MaxComment = 65536

// Leak is one gitleaks finding. gitleaks runs with --redact, and only the
// rule and the line leave this package.
type Leak struct {
	RuleID    string `json:"RuleID"`
	StartLine int    `json:"StartLine"`
}

// LeaksFrom decodes a gitleaks JSON report. An empty report is the array [].
func LeaksFrom(data []byte) ([]Leak, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("[")) {
		return nil, errors.New("gitleaks returned no report")
	}
	var leaks []Leak
	if err := decode(data, &leaks, false); err != nil {
		return nil, err
	}
	for _, l := range leaks {
		if blank(l.RuleID) || l.StartLine < 1 {
			return nil, errors.New("invalid gitleaks finding")
		}
	}
	return leaks, nil
}

// ValidEvidence checks a body before evidence posts it.
func ValidEvidence(body string) error {
	switch {
	case blank(body):
		return errors.New("evidence body is empty")
	case utf8.RuneCountInString(body) > MaxComment:
		return errors.New("evidence body is longer than GitHub's 65536-character comment limit")
	case strings.HasPrefix(strings.TrimSpace(body), markerPrefix):
		return errors.New("evidence cannot start with the Lights Out record marker")
	}
	return nil
}
