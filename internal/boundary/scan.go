package boundary

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxComment is GitHub's limit on one comment, in characters.
const MaxComment = 65536

// MaxParts bounds how many comments one evidence body may span.
const MaxParts = 8

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
	_, err := EvidenceParts(body)
	return err
}

// EvidenceParts cuts a body into the comments that evidence posts, in order.
// A body longer than one comment is cut at line ends, so the parts joined
// give the body again, and each part must be valid evidence on its own.
func EvidenceParts(body string) ([]string, error) {
	if blank(body) {
		return nil, errors.New("evidence body is empty")
	}
	if utf8.RuneCountInString(body) > MaxParts*MaxComment {
		return nil, fmt.Errorf("evidence body is longer than %d comments of %d characters", MaxParts, MaxComment)
	}
	var parts []string
	for rest := body; rest != ""; {
		cut := len(rest)
		if utf8.RuneCountInString(rest) > MaxComment {
			cut = runeOffset(rest, MaxComment)
			if end := strings.LastIndexByte(rest[:cut], '\n'); end >= 0 && !blank(rest[:end+1]) {
				cut = end + 1
			}
		}
		part := rest[:cut]
		if blank(part) || strings.HasPrefix(strings.TrimSpace(part), markerPrefix) {
			return nil, fmt.Errorf("evidence part %d would be blank or start with the Lights Out record marker", len(parts)+1)
		}
		parts, rest = append(parts, part), rest[cut:]
	}
	if len(parts) > MaxParts {
		return nil, fmt.Errorf("evidence body needs more than %d comments", MaxParts)
	}
	return parts, nil
}

// runeOffset is the byte offset of the first n runes of s.
func runeOffset(s string, n int) int {
	for i := range s {
		if n == 0 {
			return i
		}
		n--
	}
	return len(s)
}
