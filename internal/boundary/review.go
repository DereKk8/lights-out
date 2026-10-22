package boundary

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Receipt is the verifier's typed endorsement of one review run. It binds the
// verdict to the head, the frozen contract, the proofs and the report.
type Receipt struct {
	Policy         string `json:"policy"`
	Head           string `json:"head"`
	Verdict        string `json:"verdict"`
	ContractSHA256 string `json:"contract_sha256"`
	ProofsSHA256   string `json:"proofs_sha256"`
	ReportSHA256   string `json:"report_sha256"`
	Author         string `json:"author"`
	AuthorFamily   string `json:"author_family"`
	Reviewer       string `json:"reviewer"`
	ReviewerFamily string `json:"reviewer_family"`
	Verifier       string `json:"verifier"`
}

// ReceiptFrom reads a receipt comment: one JSON object in a json code fence.
func ReceiptFrom(body string) (Receipt, error) {
	var v Receipt
	text := strings.TrimSpace(body)
	if !strings.HasPrefix(text, "```json\n") || !strings.HasSuffix(text, "\n```") {
		return v, errors.New("receipt comment must be one JSON object in a ```json fence")
	}
	if err := decode([]byte(text[len("```json\n"):len(text)-len("\n```")]), &v, true); err != nil {
		return v, fmt.Errorf("receipt: %w", err)
	}
	if !shaPattern.MatchString(v.Head) || !hashPattern.MatchString(v.ContractSHA256) || !hashPattern.MatchString(v.ProofsSHA256) || !hashPattern.MatchString(v.ReportSHA256) {
		return v, errors.New("receipt needs the full head SHA and the contract, proofs and report digests")
	}
	if blank(v.Policy, v.Verdict, v.Author, v.AuthorFamily, v.Reviewer, v.ReviewerFamily, v.Verifier) {
		return v, errors.New("receipt needs policy, verdict, roles and model families")
	}
	return v, nil
}

// Policy is one exact version of zero-trust-review, identified as
// sync-zero-trust step 1 prescribes.
type Policy struct {
	Identity string
	IDs      []string
}

var (
	versionLine   = regexp.MustCompile(`(?m)^Skill version: (\S+)$`)
	requirementID = regexp.MustCompile(`(?m)^\s*- \*\*(ZT-[A-Z]+-[0-9]+)\.\*\*`)
)

type contents struct {
	SHA     string `json:"sha"`
	Content string `json:"content"`
}

// PolicyFrom reads the policy file from the GitHub contents API at commit. It
// checks the bytes against their git blob ID before it trusts them.
func PolicyFrom(commit string, data []byte) (Policy, error) {
	var c contents
	if err := decode(data, &c, true); err != nil {
		return Policy{}, fmt.Errorf("policy file: %w", err)
	}
	if !shaPattern.MatchString(commit) || !shaPattern.MatchString(c.SHA) {
		return Policy{}, errors.New("policy commit and blob must be full SHAs")
	}
	text, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(c.Content, "\n", ""))
	if err != nil {
		return Policy{}, fmt.Errorf("policy file content: %w", err)
	}
	if fmt.Sprintf("%x", sha1.Sum(fmt.Appendf(nil, "blob %d\x00%s", len(text), text))) != c.SHA {
		return Policy{}, errors.New("policy file bytes do not match their blob ID")
	}
	version := versionLine.FindSubmatch(text)
	if version == nil {
		return Policy{}, errors.New("policy file has no Skill version line")
	}
	var ids []string
	for _, m := range requirementID.FindAllSubmatch(text, -1) {
		ids = append(ids, string(m[1]))
	}
	if len(ids) == 0 {
		return Policy{}, errors.New("policy file defines no requirement IDs")
	}
	return Policy{Identity: fmt.Sprintf("Identity: zero-trust-review %s %s %s", version[1], commit, c.SHA), IDs: ids}, nil
}

// Sync is the reply form of sync-zero-trust: the identity a team loaded and
// the status it gave each requirement ID.
type Sync struct {
	Identity   string
	IDs        []string
	Blocking   []string
	Incomplete string
}

var (
	syncHeader = regexp.MustCompile(`^ZERO-TRUST SYNC sync-zero-trust \S+$`)
	mapLine    = regexp.MustCompile(`^(ZT-[A-Z]+-[0-9]+): (met|met another way|gap|accepted exception) \| (.+)$`)
)

// SyncFrom reads a sync reply comment, optionally inside one code fence.
func SyncFrom(body string) (Sync, error) {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n")), "\n")
	if len(lines) > 1 && strings.HasPrefix(lines[0], "```") && lines[len(lines)-1] == "```" {
		lines = lines[1 : len(lines)-1]
	}
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	if len(lines) < 7 || !syncHeader.MatchString(lines[0]) || !strings.HasPrefix(lines[1], "Identity: ") || !strings.HasPrefix(lines[2], "Consumer: ") || lines[3] != "Map:" {
		return Sync{}, errors.New("sync reply must open with the header, Identity, Consumer and Map lines")
	}
	s := Sync{Identity: lines[1]}
	rest := lines[4:]
	for len(rest) > 0 && strings.HasPrefix(rest[0], "ZT-") {
		m := mapLine.FindStringSubmatch(rest[0])
		if m == nil {
			return Sync{}, fmt.Errorf("sync map line %q has no valid status", rest[0])
		}
		s.IDs = append(s.IDs, m[1])
		switch m[2] {
		case "gap":
			i := strings.LastIndex(m[3], " | blocks: ")
			if i < 0 {
				return Sync{}, fmt.Errorf("sync gap %s must say what it blocks", m[1])
			}
			if strings.TrimSpace(m[3][i+len(" | blocks: "):]) != "none" {
				s.Blocking = append(s.Blocking, m[1])
			}
		case "accepted exception":
			if !strings.Contains(m[3], " | outside the scope: ") {
				return Sync{}, fmt.Errorf("sync exception %s must name its mechanism outside the scope", m[1])
			}
		}
		rest = rest[1:]
	}
	if len(rest) != 2 || !strings.HasPrefix(rest[0], "Incomplete parts: ") || !strings.HasPrefix(rest[1], "Proposals: ") {
		return Sync{}, errors.New("sync reply must end with the Incomplete parts and Proposals lines")
	}
	s.Incomplete = strings.TrimPrefix(rest[0], "Incomplete parts: ")
	return s, nil
}
