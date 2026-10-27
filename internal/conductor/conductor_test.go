package conductor_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/DereKk8/lights-out/internal/boundary"
	"github.com/DereKk8/lights-out/internal/conductor"
)

const (
	head    = "1111111111111111111111111111111111111111"
	other   = "2222222222222222222222222222222222222222"
	commit  = "3333333333333333333333333333333333333333"
	moved   = "4444444444444444444444444444444444444444"
	stamp   = "2026-10-18T10:00:00Z"
	edited  = "2026-10-18T11:00:00Z"
	issue1  = "https://github.com/o/r/issues/1"
	issue2  = "https://github.com/o/r/issues/2"
	written = "owner"
)

var policyText = "---\nname: zero-trust-review\n---\n\nSkill version: 9.9.9\n\n- **ZT-ONE-1.** First rule.\n- **ZT-TWO-1.** Second rule.\n  - Reviewer: a nested list item is not an ID.\n"

// github fakes the forge behind gh. It answers only the calls the conductor
// is expected to make and fails the test on any other call.
type github struct {
	t            *testing.T
	issue        boundary.Issue
	comments     []boundary.Comment
	elsewhere    []boundary.Comment
	permissions  map[string]string
	pull         boundary.Pull
	pullMovesTo  string
	views        int
	checks       []boundary.Check
	comparison   string
	policyCommit string
	nextID       int64
	posts        int
	onChecks     func()
	onPost       func()
	down         bool
	scans        []string
	leaks        string
	scannerDown  bool
	tracker      map[int]boundary.Issue
}

func newGitHub(t *testing.T) *github {
	return &github{
		t:            t,
		issue:        boundary.Issue{Number: 1, Title: "Keep the stored value", Body: "The CLI must keep the stored value.", URL: issue1},
		comments:     []boundary.Comment{},
		permissions:  map[string]string{written: "admin", "stranger": "read"},
		pull:         boundary.Pull{Head: head, State: "OPEN", URL: "https://github.com/o/r/pull/2"},
		checks:       []boundary.Check{{Name: "ci", Bucket: "pass", Link: "https://github.com/o/r/actions/runs/1"}},
		comparison:   "ahead",
		policyCommit: commit,
		nextID:       100,
	}
}

func encoded(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// comment posts a comment on issue 1 as author and returns it.
func (g *github) comment(body, author string) boundary.Comment {
	g.nextID++
	c := boundary.Comment{ID: g.nextID, Body: body, URL: fmt.Sprintf("%s#issuecomment-%d", issue1, g.nextID), Author: author, Created: stamp, Updated: stamp}
	g.comments = append(g.comments, c)
	return c
}

func (g *github) artifact(body string) boundary.Artifact {
	c := g.comment(body, written)
	return boundary.Artifact{SHA256: boundary.BodyDigest(c.Body), URL: c.URL}
}

func (g *github) store(previous int64, r boundary.Record) boundary.Comment {
	body, err := boundary.Render(boundary.Stored{Previous: previous, Record: r})
	if err != nil {
		g.t.Fatal(err)
	}
	return g.comment(body, written)
}

func (g *github) find(id int64) (boundary.Comment, bool) {
	for _, c := range slices.Concat(g.comments, g.elsewhere) {
		if c.ID == id {
			return c, true
		}
	}
	return boundary.Comment{}, false
}

func (g *github) policy() []byte {
	return encoded(g.t, map[string]string{"sha": blob(g.t, policyText), "content": base64.StdEncoding.EncodeToString([]byte(policyText))})
}

// blob asks git itself for the blob ID, as an independent oracle.
func blob(t *testing.T, text string) string {
	cmd := exec.Command("git", "hash-object", "--stdin")
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func followed(t *testing.T) string {
	return "Identity: zero-trust-review 9.9.9 " + commit + " " + blob(t, policyText)
}

func (g *github) run(_ context.Context, input []byte, name string, args ...string) ([]byte, error) {
	if g.down {
		return nil, errors.New("gh api: exit status 1: forge unavailable")
	}
	if name == "gitleaks" {
		return g.scan(input, args)
	}
	if name != "gh" || len(args) < 2 {
		g.t.Fatalf("unexpected tool call %s %v", name, args)
	}
	call := strings.Join(args, " ")
	switch {
	case call == "api repos/o/r/issues/1 --jq {number,title,body,url:.html_url}":
		return encoded(g.t, g.issue), nil
	case strings.HasPrefix(call, "api repos/o/r/issues/") && strings.HasSuffix(call, " --jq {number,title,body,url:.html_url}"):
		n, err := strconv.Atoi(strings.Fields(strings.TrimPrefix(call, "api repos/o/r/issues/"))[0])
		if err != nil {
			g.t.Fatal(err)
		}
		if issue, ok := g.tracker[n]; ok {
			return encoded(g.t, issue), nil
		}
		return nil, errors.New("gh api: exit status 1: HTTP 404: Not Found")
	case strings.HasPrefix(call, "api repos/o/r/issues/1/comments?per_page=100&page="):
		page, err := strconv.Atoi(strings.Fields(strings.TrimPrefix(call, "api repos/o/r/issues/1/comments?per_page=100&page="))[0])
		if err != nil {
			g.t.Fatal(err)
		}
		lo, hi := min((page-1)*100, len(g.comments)), min(page*100, len(g.comments))
		return encoded(g.t, g.comments[lo:hi]), nil
	case strings.HasPrefix(call, "api repos/o/r/issues/1/comments --method POST --input -"):
		g.posts++
		body, err := strconv.Unquote(strings.TrimSuffix(strings.TrimPrefix(string(input), `{"body":`), "}"))
		if err != nil {
			g.t.Fatalf("posted payload %s: %v", input, err)
		}
		c := g.comment(body, written)
		if g.onPost != nil {
			g.onPost()
		}
		return encoded(g.t, c), nil
	case strings.HasPrefix(call, "api repos/o/r/issues/comments/"):
		id, err := strconv.ParseInt(strings.Fields(strings.TrimPrefix(call, "api repos/o/r/issues/comments/"))[0], 10, 64)
		if err != nil {
			g.t.Fatal(err)
		}
		if c, ok := g.find(id); ok {
			return encoded(g.t, c), nil
		}
		return nil, errors.New("gh api: exit status 1: HTTP 404: Not Found")
	case strings.HasPrefix(call, "api repos/o/r/collaborators/"):
		login := strings.TrimSuffix(strings.Fields(strings.TrimPrefix(call, "api repos/o/r/collaborators/"))[0], "/permission")
		permission, ok := g.permissions[login]
		if !ok {
			g.t.Fatalf("permission of unknown login %q", login)
		}
		return encoded(g.t, map[string]string{"permission": permission}), nil
	case strings.HasPrefix(call, "pr view 2 --repo o/r "):
		g.views++
		p := g.pull
		if g.views > 1 && g.pullMovesTo != "" {
			p.Head = g.pullMovesTo
		}
		return encoded(g.t, p), nil
	case call == "pr checks 2 --repo o/r --required --json name,bucket,link":
		if g.onChecks != nil {
			g.onChecks()
		}
		return encoded(g.t, g.checks), nil
	case call == "api repos/o/r --jq {default_branch}":
		return []byte(`{"default_branch":"main"}`), nil
	case call == "api repos/o/r/compare/"+head+"...main --jq {status}":
		return encoded(g.t, map[string]string{"status": g.comparison}), nil
	case call == "api repos/DereKk8/project-skills/commits?sha=master&path=shared/zero-trust-review/SKILL.md&per_page=1 --jq [.[]|{sha}]":
		return encoded(g.t, []map[string]string{{"sha": g.policyCommit}}), nil
	case call == "api repos/DereKk8/project-skills/contents/shared/zero-trust-review/SKILL.md?ref="+g.policyCommit+" --jq {sha,content}":
		return g.policy(), nil
	}
	g.t.Fatalf("unexpected tool call: gh %s", call)
	return nil, nil
}

// scan fakes gitleaks. It checks that the conductor pins the default rules and
// ignores allow comments, then answers with g.leaks or an empty report.
func (g *github) scan(input []byte, args []string) ([]byte, error) {
	g.scans = append(g.scans, string(input))
	if g.scannerDown {
		return nil, errors.New("gitleaks stdin: executable file not found in $PATH")
	}
	if len(args) != 16 {
		g.t.Fatalf("unexpected gitleaks call %v", args)
	}
	config, ignore := args[9], args[11]
	want := []string{"stdin", "--no-banner", "--log-level", "error", "--redact", "--exit-code", "0", "--ignore-gitleaks-allow", "--config", config, "--gitleaks-ignore-path", ignore, "--report-format", "json", "--report-path", "-"}
	if !slices.Equal(args, want) || filepath.Dir(config) != ignore {
		g.t.Fatalf("unexpected gitleaks call %v", args)
	}
	rules, err := os.ReadFile(config)
	if err != nil || string(rules) != "[extend]\nuseDefault = true\n" {
		g.t.Fatalf("gitleaks config %q: %v", rules, err)
	}
	if g.leaks != "" {
		return []byte(g.leaks), nil
	}
	return []byte("[]\n"), nil
}

func drive(t *testing.T, g *github, req string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := conductor.Run(context.Background(), strings.NewReader(req), &out, g.run, followed(t))
	return out.String(), err
}

func base(g *github) boundary.Record {
	return boundary.Record{
		IntentSHA256: boundary.BodyDigest(g.issue.Title + "\n" + g.issue.Body),
		Coordinator:  "team-1", Session: "coordinator-session-a", Author: "author-seat", CIOwner: "team-1",
		Phase:      boundary.PhaseActive,
		Acceptance: []boundary.Criterion{{ID: "A1", Requirement: "keep the stored value"}},
		Proofs:     []boundary.Proof{}, Findings: []boundary.Finding{},
		Next: "reproduce the defect",
	}
}

func syncReply(identity string, lines ...string) string {
	return "```\nZERO-TRUST SYNC sync-zero-trust 2.0.0\n" + identity + "\nConsumer: team-1 | runtime\nMap:\n" + strings.Join(lines, "\n") + "\nIncomplete parts: none\nProposals: none\n```"
}

func receiptBody(t *testing.T, v boundary.Receipt) string {
	return "```json\n" + string(encoded(t, v)) + "\n```"
}

// reviewed is a complete, valid issue history for assess: every review part
// posted in the order the policy requires, then the review record.
type reviewed struct {
	g       *github
	record  boundary.Record
	receipt boundary.Receipt
	latest  int64
}

func setup(t *testing.T, change func(*reviewed)) *reviewed {
	g := newGitHub(t)
	s := &reviewed{g: g, record: base(g)}
	s.record.Phase, s.record.Next = boundary.PhaseReview, "assess"
	s.record.Candidate = &boundary.Candidate{Kind: "change", Head: head, PR: 2}
	v := &boundary.Review{}
	v.Sync = g.artifact(syncReply(followed(t), "ZT-ONE-1: met | assess checks it", "ZT-TWO-1: gap | not built | blocks: none"))
	v.Intent = g.artifact("Intent reading: the stored value must survive a restart.")
	v.Suspicions = g.artifact("S1: the value may be lost on restart.")
	v.Surface = g.artifact("S1 disproved: $ product get\nexpected 7, observed 7")
	v.Attack = g.artifact("attack: restart mid-write | safe: held")
	surface := v.Surface
	s.record.Suspicions = []boundary.Suspicion{{ID: "S1", State: "disproved", By: "verify-seat", Evidence: &surface}}
	s.record.Proofs = []boundary.Proof{{Criterion: "A1", Requirement: "keep the stored value", Head: head, Kind: "surface", Build: "sha256:binary", Evidence: g.artifact("$ product set 7; product get\n7"), Observation: g.artifact("$ cat store\n7")}}
	v.Report = g.artifact("Skill identity: ...\nVerdict: PASS")
	s.record.Findings = []boundary.Finding{{ID: "F1", Scope: "in", Owner: "team-1", Disposition: "fixed", Evidence: g.artifact("trigger: restart; harm: lost value; disposition: flush")}}
	s.record.Review = v
	s.receipt = boundary.Receipt{Policy: followed(t), Head: head, Verdict: "PASS", Author: "author-seat", AuthorFamily: "Grok", Reviewer: "review-seat", ReviewerFamily: "GPT", Verifier: "verify-seat"}
	if change != nil {
		change(s)
	}
	s.seal(t)
	return s
}

// seal posts the receipt for the record as it stands, then the record.
func (s *reviewed) seal(t *testing.T) {
	var err error
	if s.receipt.ContractSHA256 == "" {
		s.receipt.ContractSHA256, err = boundary.Digest(s.record.Acceptance)
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.receipt.ProofsSHA256 == "" {
		s.receipt.ProofsSHA256, err = boundary.Digest(s.record.Proofs)
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.receipt.ReportSHA256 == "" {
		s.receipt.ReportSHA256 = s.record.Review.Report.SHA256
	}
	if s.record.Review.Receipt.URL == "" {
		s.record.Review.Receipt = s.g.artifact(receiptBody(t, s.receipt))
	}
	s.latest = s.g.store(0, s.record).ID
}

func (s *reviewed) assess(t *testing.T) (string, error) {
	return drive(t, s.g, fmt.Sprintf(`{"command":"assess","repo":"o/r","issue":1,"previous":%d}`, s.latest))
}

// handoff gives the record an out-of-scope finding that the team-2 record on
// issue 2 accepted.
func handoff(s *reviewed, coordinator string) {
	g := s.g
	accepting := base(g)
	accepting.Coordinator = coordinator
	body, err := boundary.Render(boundary.Stored{Record: accepting})
	if err != nil {
		g.t.Fatal(err)
	}
	g.elsewhere = append(g.elsewhere, boundary.Comment{ID: 50, Body: body, URL: issue2 + "#issuecomment-50", Author: written, Created: stamp, Updated: stamp})
	s.record.Findings = append(s.record.Findings, boundary.Finding{ID: "F2", Scope: "out", Owner: "team-2", Disposition: "handed-off", Evidence: g.artifact("trigger and harm of an unrelated defect"), Accepted: issue2 + "#issuecomment-50"})
}

func TestAssessRecordsTheVerifiedOutcome(t *testing.T) {
	s := setup(t, func(s *reviewed) { handoff(s, "team-2") })
	out, err := s.assess(t)
	if err != nil {
		t.Fatal(err)
	}
	if s.g.posts != 1 {
		t.Fatalf("assess wrote %d records, want 1", s.g.posts)
	}
	stored, err := boundary.StoredFrom(s.g.comments[len(s.g.comments)-1].Body)
	if err != nil {
		t.Fatal(err)
	}
	r := stored.Record
	if stored.Previous != s.latest || r.Phase != boundary.PhaseVerified || r.Outcome.Head != head || len(r.Outcome.Checks) != 1 || r.Outcome.Checks[0].Name != "ci" {
		t.Fatalf("wrong outcome record: %+v", stored)
	}
	if !strings.Contains(out, `"status":"verified"`) || !strings.Contains(out, `"phase":"verified"`) {
		t.Fatalf("wrong assess output %s", out)
	}
	// A fresh invocation, with no memory of this one, recovers the outcome.
	out, err = drive(t, s.g, `{"command":"inspect","repo":"o/r","issue":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"phase":"verified"`) || !strings.Contains(out, fmt.Sprintf(`"latest":%d`, s.g.nextID)) {
		t.Fatalf("inspect did not recover the outcome: %s", out)
	}
}

// TestAssessRefusesUnjudgedAcceptance covers the three ways a run could pass
// without a judgment that the record proves: a narrowed criterion, a closed
// but unproven suspicion, and a judgment credited to the wrong seat.
func TestAssessRefusesUnjudgedAcceptance(t *testing.T) {
	accepted := map[string]func(*reviewed){
		"a changed criterion with the owner's decision": func(s *reviewed) {
			decision := s.g.artifact("Owner decision: A1 may be proved for restarts only.")
			s.record.Proofs[0].Requirement, s.record.Proofs[0].Decision = "keep the stored value across restarts", &decision
		},
		"a suspicion reproduced as a finding": func(s *reviewed) {
			s.record.Suspicions[0].State, s.record.Suspicions[0].Finding = "reproduced", "F1"
		},
		"a suspicion disproved in the reviewer's own part": func(s *reviewed) {
			suspicions := s.record.Review.Suspicions
			s.record.Suspicions[0].By, s.record.Suspicions[0].Evidence = "review-seat", &suspicions
		},
	}
	for name, change := range accepted {
		t.Run(name, func(t *testing.T) {
			if _, err := setup(t, change).assess(t); err != nil {
				t.Fatalf("refused: %v", err)
			}
		})
	}

	refused := map[string]struct {
		want   string
		change func(*reviewed)
	}{
		"no requirement stated": {"must state the requirement it proves", func(s *reviewed) { s.record.Proofs[0].Requirement = "" }},
		"a narrowed criterion": {"a changed criterion needs the owner's decision", func(s *reviewed) {
			s.record.Proofs[0].Requirement = "keep the stored value while nothing else writes"
		}},
		"an edited owner decision": {"the decision for A1", func(s *reviewed) {
			decision := s.g.artifact("Owner decision.")
			s.record.Proofs[0].Requirement, s.record.Proofs[0].Decision = "narrower", &decision
			s.g.comments[len(s.g.comments)-1].Body = "Edited."
		}},
		"an open suspicion": {"suspicion S1 is open", func(s *reviewed) {
			s.record.Suspicions = []boundary.Suspicion{{ID: "S1", State: "open"}}
		}},
		"an untracked suspicion": {"names suspicion S1, which the record does not track", func(s *reviewed) {
			s.record.Suspicions = nil
		}},
		"a reproduced suspicion without its finding": {"does not list as a finding", func(s *reviewed) {
			s.record.Suspicions[0].State, s.record.Suspicions[0].Finding = "reproduced", "F7"
		}},
		"a judgment credited to another seat": {"neither the reviewer nor the verifier", func(s *reviewed) {
			s.record.Suspicions[0].By = "team-1"
		}},
		"a verifier judgment that cites the reviewer's part": {"not a review part of verify-seat", func(s *reviewed) {
			suspicions := s.record.Review.Suspicions
			s.record.Suspicions[0].Evidence = &suspicions
		}},
		"a reviewer judgment that the reviewer never made": {"not a review part of review-seat", func(s *reviewed) {
			s.record.Suspicions[0].By = "review-seat"
		}},
		"a cited part that does not name the suspicion": {"does not name it", func(s *reviewed) {
			attack := s.record.Review.Attack
			s.record.Suspicions[0].Evidence = &attack
		}},
	}
	for name, tc := range refused {
		t.Run(name, func(t *testing.T) {
			s := setup(t, tc.change)
			if _, err := s.assess(t); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

// below adds a below-floor finding (ZT-SEV-1) filed as tracker issue 3.
func below(s *reviewed) {
	s.g.tracker = map[int]boundary.Issue{3: {Number: 3, Title: "Clarify the drift wording", Body: "Below-floor finding F9.", URL: "https://github.com/o/r/issues/3"}}
	s.record.Findings = append(s.record.Findings, boundary.Finding{ID: "F9", Scope: "in", Owner: "team-1", Disposition: "filed", Evidence: s.g.artifact("trigger: rare; harm: wording; disposition: file"), Filed: "https://github.com/o/r/issues/3"})
}

func TestAssessAcceptsFiledBelowFloorFindings(t *testing.T) {
	s := setup(t, below)
	if _, err := s.assess(t); err != nil {
		t.Fatalf("a PASS with a filed below-floor finding was refused: %v", err)
	}
	for name, tc := range map[string]struct {
		want   string
		change func(*reviewed)
	}{
		"issue missing": {"404", func(s *reviewed) { s.g.tracker = nil }},
		"pull request linked": {"not a pull request", func(s *reviewed) {
			s.g.tracker[3] = boundary.Issue{Number: 3, Title: "A PR", Body: "x", URL: "https://github.com/o/r/pull/3"}
		}},
		"source issue linked": {"its own issue in the source repository", func(s *reviewed) { s.record.Findings[len(s.record.Findings)-1].Filed = issue1 }},
		"other repository": {"its own issue in the source repository", func(s *reviewed) {
			s.record.Findings[len(s.record.Findings)-1].Filed = "https://github.com/x/y/issues/3"
		}},
		"edited recheck basis": {"recheck_basis", func(s *reviewed) {
			basis := s.g.artifact("Owner decision.")
			s.record.RecheckBasis = &basis
			s.g.comments[len(s.g.comments)-1].Body = "Edited decision."
		}},
	} {
		t.Run(name, func(t *testing.T) {
			s := setup(t, func(s *reviewed) {
				below(s)
				tc.change(s)
			})
			if _, err := s.assess(t); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestAssessVerifiesNoChangeOnTheDefaultBranch(t *testing.T) {
	for status, want := range map[string]string{"ahead": "", "identical": "", "diverged": "not on the default branch"} {
		t.Run(status, func(t *testing.T) {
			s := setup(t, func(s *reviewed) { s.record.Candidate = &boundary.Candidate{Kind: "no-change", Head: head} })
			s.g.comparison = status
			out, err := s.assess(t)
			if want == "" && (err != nil || !strings.Contains(out, `"checks":[]`)) {
				t.Fatalf("output=%s error=%v", out, err)
			}
			if want != "" && (err == nil || !strings.Contains(err.Error(), want) || s.g.posts != 0) {
				t.Fatalf("output=%s error=%v", out, err)
			}
		})
	}
}

func TestAssessRefusesUnsupportedAcceptance(t *testing.T) {
	tests := []struct {
		name, want string
		before     func(*reviewed)
		after      func(*reviewed)
	}{
		{name: "no review", want: "no independent review", before: func(s *reviewed) {
			s.record.Review.Receipt = s.g.artifact("unused")
		}, after: func(s *reviewed) {
			s.record.Review = nil
			s.latest = s.g.store(s.latest, s.record).ID
		}},
		{name: "phase active", want: "needs phase review", after: func(s *reviewed) {
			s.record.Phase = boundary.PhaseActive
			s.latest = s.g.store(s.latest, s.record).ID
		}},
		{name: "phase blocked on the owner", want: "needs phase review", after: func(s *reviewed) {
			s.record.Phase = boundary.PhaseBlocked
			s.latest = s.g.store(s.latest, s.record).ID
		}},
		{name: "re-check bound", want: "re-scope", before: func(s *reviewed) { s.record.FailedRechecks = 2 }},
		{name: "missing proof", want: "acceptance A1 has no proof", before: func(s *reviewed) { s.record.Proofs = []boundary.Proof{} }},
		{name: "stale proof", want: "stale evidence", before: func(s *reviewed) { s.record.Proofs[0].Head = other }},
		{name: "stale verdict", want: "stale verdict", before: func(s *reviewed) { s.receipt.Head = other }},
		{name: "not PASS", want: "INCOMPLETE, not PASS", before: func(s *reviewed) { s.receipt.Verdict = "INCOMPLETE" }},
		{name: "other contract", want: "acceptance contract", before: func(s *reviewed) { s.receipt.ContractSHA256 = strings.Repeat("a", 64) }},
		{name: "proofs changed after the receipt", want: "these proofs", before: func(s *reviewed) {
			s.receipt.ProofsSHA256 = strings.Repeat("b", 64)
		}},
		{name: "other report", want: "this report", before: func(s *reviewed) { s.receipt.ReportSHA256 = strings.Repeat("c", 64) }},
		{name: "receipt for another author", want: "names author", before: func(s *reviewed) { s.receipt.Author = "someone" }},
		{name: "author reviews", want: "cannot hold", before: func(s *reviewed) { s.receipt.Reviewer = "author-seat" }},
		{name: "author verifies", want: "cannot hold", before: func(s *reviewed) { s.receipt.Verifier = "author-seat" }},
		{name: "same model family", want: "ZT-MODEL-1", before: func(s *reviewed) { s.receipt.ReviewerFamily = " grok" }},
		{name: "receipt follows another policy", want: "the receipt follows", before: func(s *reviewed) { s.receipt.Policy = "Identity: zero-trust-review 1.0.0" }},
		{name: "edited evidence", want: "changed since it was pinned", after: func(s *reviewed) { s.g.comments[1].Body = "edited intent" }},
		{name: "evidence from a non-writer", want: "who cannot write", before: func(s *reviewed) {
			s.g.comments[2].Author = "stranger"
		}},
		{name: "evidence on another issue", want: "not a comment on the source issue", before: func(s *reviewed) {
			s.record.Review.Attack.URL = issue2 + "#issuecomment-104"
		}},
		{name: "intent after the review", want: "ZT-INDEP-1", before: func(s *reviewed) {
			s.record.Review.Intent, s.record.Review.Suspicions = s.record.Review.Suspicions, s.record.Review.Intent
		}},
		{name: "proof before the saved review", want: "ZT-INDEP-7", before: func(s *reviewed) {
			s.record.Review.Surface = s.record.Review.Sync
		}},
		{name: "proof outside the window", want: "between the saved review and the receipt", before: func(s *reviewed) {
			s.record.Proofs[0].Observation = s.record.Review.Intent
		}},
		{name: "policy moved upstream", want: "zero-trust-review is now", after: func(s *reviewed) { s.g.policyCommit = moved }},
		{name: "sync to another identity", want: "synced to", before: func(s *reviewed) {
			s.record.Review.Sync = s.g.artifact(syncReply("Identity: zero-trust-review 1.0.0 x y", "ZT-ONE-1: met | x", "ZT-TWO-1: met | y"))
			s.reorder()
		}},
		{name: "sync skips an ID", want: "each requirement ID", before: func(s *reviewed) {
			s.record.Review.Sync = s.g.artifact(syncReply(followed(s.g.t), "ZT-ONE-1: met | x"))
			s.reorder()
		}},
		{name: "sync blocking gap", want: "INCOMPLETE", before: func(s *reviewed) {
			s.record.Review.Sync = s.g.artifact(syncReply(followed(s.g.t), "ZT-ONE-1: met | x", "ZT-TWO-1: gap | no attack runner | blocks: attack phase"))
			s.reorder()
		}},
		{name: "open in-scope finding", want: "finding F1 is open", before: func(s *reviewed) { s.record.Findings[0].Disposition = "open" }},
		{name: "unaccepted handoff", want: "finding F2 is open", before: func(s *reviewed) {
			handoff(s, "team-2")
			s.record.Findings[1].Disposition, s.record.Findings[1].Accepted = "open", ""
		}},
		{name: "handoff kept by the same team", want: "separate from this team", before: func(s *reviewed) {
			handoff(s, "team-1")
			s.record.Findings[1].Owner = "team-1"
		}},
		{name: "handoff accepted by prose", want: "not a Lights Out record", before: func(s *reviewed) {
			handoff(s, "team-2")
			s.g.elsewhere[0].Body = "I will take this."
		}},
		{name: "handoff accepted on this issue", want: "another issue", before: func(s *reviewed) {
			handoff(s, "team-2")
			s.record.Findings[1].Accepted = s.record.Review.Intent.URL
		}},
		{name: "moved head", want: "the candidate is stale", after: func(s *reviewed) { s.g.pull.Head = other }},
		{name: "closed PR", want: "not open", after: func(s *reviewed) { s.g.pull.State = "MERGED" }},
		{name: "head moves during checks", want: "changed during assessment", after: func(s *reviewed) { s.g.pullMovesTo = other }},
		{name: "failed check", want: "required check ci is fail", after: func(s *reviewed) { s.g.checks[0].Bucket = "fail" }},
		{name: "pending check", want: "required check ci is pending", after: func(s *reviewed) { s.g.checks[0].Bucket = "pending" }},
		{name: "skipped check", want: "required check ci is skipping", after: func(s *reviewed) { s.g.checks[0].Bucket = "skipping" }},
		{name: "no required checks", want: "requires no checks", after: func(s *reviewed) { s.g.checks = []boundary.Check{} }},
		{name: "issue edited during assessment", want: "issue changed during assessment", after: func(s *reviewed) {
			s.g.onChecks = func() { s.g.issue.Body = "A weaker intent." }
		}},
		{name: "source intent changed", want: "source issue changed", after: func(s *reviewed) { s.g.issue.Body = "A weaker intent." }},
		{name: "stale predecessor", want: "stale predecessor", after: func(s *reviewed) { s.latest = 7 }},
		{name: "environment failure", want: "forge unavailable", after: func(s *reviewed) { s.g.down = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := setup(t, tc.before)
			if tc.after != nil {
				tc.after(s)
			}
			out, err := s.assess(t)
			if err == nil || !strings.Contains(err.Error(), tc.want) || out != "" || s.g.posts != 0 {
				t.Fatalf("output=%q error=%v posts=%d, want refusal %q and no write", out, err, s.g.posts, tc.want)
			}
		})
	}
}

// reorder posts the review parts after the sync again, so a replaced sync
// still comes first.
func (s *reviewed) reorder() {
	v := s.record.Review
	v.Intent = s.g.artifact("Intent reading again.")
	v.Suspicions = s.g.artifact("S1 again.")
	v.Surface = s.g.artifact("S1 disproved again")
	surface := v.Surface
	s.record.Suspicions[0].Evidence = &surface
	v.Attack = s.g.artifact("attack again")
	p := &s.record.Proofs[0]
	p.Evidence, p.Observation = s.g.artifact("evidence again"), s.g.artifact("observation again")
	v.Report = s.g.artifact("report again")
}

func checkpoint(t *testing.T, g *github, previous int64, r boundary.Record) (string, error) {
	return drive(t, g, fmt.Sprintf(`{"command":"checkpoint","repo":"o/r","issue":1,"previous":%d,"record":%s}`, previous, encoded(t, r)))
}

func TestCheckpointPersistsStateForAFreshSession(t *testing.T) {
	g := newGitHub(t)
	g.comment("Ordinary discussion.", "stranger")
	r := base(g)
	out, err := checkpoint(t, g, 0, r)
	if err != nil {
		t.Fatal(err)
	}
	first := g.nextID
	if g.posts != 1 || !strings.Contains(out, fmt.Sprintf(`"latest":%d`, first)) || !strings.Contains(out, `"contract_sha256":"`) {
		t.Fatalf("wrong checkpoint output %s", out)
	}
	if !strings.HasPrefix(g.comments[1].Body, boundary.Marker+"\n**Lights Out record** · phase `active` · no candidate yet") {
		t.Fatalf("record comment is not readable: %s", g.comments[1].Body)
	}
	r.Session, r.Phase, r.Next = "coordinator-session-b", boundary.PhaseReview, "independent review"
	r.Candidate = &boundary.Candidate{Kind: "change", Head: head, PR: 2}
	if _, err = checkpoint(t, g, first, r); err != nil {
		t.Fatal(err)
	}
	// A fresh invocation has no memory of the posting invocations.
	out, err = drive(t, g, `{"command":"inspect","repo":"o/r","issue":1}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"status":"recovered"`, `"policy":"` + followed(t) + `"`, `"next":"independent review"`, `"session":"coordinator-session-b"`, fmt.Sprintf(`"latest":%d`, g.nextID), `"intent_sha256":"` + r.IntentSHA256} {
		if !strings.Contains(out, want) {
			t.Fatalf("inspect output %s lacks %s", out, want)
		}
	}
}

func TestCheckpointRefusesUnauthorizedChanges(t *testing.T) {
	tests := []struct {
		name, want string
		change     func(*github, *boundary.Record)
	}{
		{"weaker acceptance", "acceptance contract changed", func(_ *github, r *boundary.Record) { r.Acceptance[0].Requirement = "weaker" }},
		{"new coordinator", "cannot change", func(_ *github, r *boundary.Record) { r.Coordinator = "team-9" }},
		{"new author", "cannot change", func(_ *github, r *boundary.Record) { r.Author = "someone" }},
		{"lower re-check count", "decreases only with a new recheck_basis", func(_ *github, r *boundary.Record) { r.FailedRechecks = 0 }},
		{"wrong intent", "does not match the source issue", func(_ *github, r *boundary.Record) { r.IntentSHA256 = strings.Repeat("d", 64) }},
		{"edited issue", "source issue changed", func(g *github, _ *boundary.Record) { g.issue.Title = "Another task" }},
		{"evidence elsewhere", "not a comment on the source issue", func(_ *github, r *boundary.Record) {
			r.Findings = []boundary.Finding{{ID: "F1", Scope: "in", Owner: "team-1", Disposition: "open", Evidence: boundary.Artifact{SHA256: strings.Repeat("e", 64), URL: issue2 + "#issuecomment-9"}}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newGitHub(t)
			r := base(g)
			r.FailedRechecks = 1
			latest := g.store(0, r).ID
			r.Acceptance = slices.Clone(r.Acceptance)
			tc.change(g, &r)
			out, err := checkpoint(t, g, latest, r)
			if err == nil || !strings.Contains(err.Error(), tc.want) || out != "" || g.posts != 0 {
				t.Fatalf("output=%q error=%v posts=%d, want refusal %q", out, err, g.posts, tc.want)
			}
		})
	}
}

func TestOwnerDecisionRecountsFailedRechecks(t *testing.T) {
	g := newGitHub(t)
	r := base(g)
	r.FailedRechecks = 1
	latest := g.store(0, r).ID
	basis := g.artifact("Owner decision: that re-review was a fresh review, not a re-check.")
	r.FailedRechecks, r.RecheckBasis = 0, &basis
	out, err := checkpoint(t, g, latest, r)
	if err != nil {
		t.Fatalf("a recount with a new owner basis was refused: %v", err)
	}
	if !strings.Contains(out, `"failed_rechecks":0`) || !strings.Contains(out, `"recheck_basis":{"sha256":"`+basis.SHA256) {
		t.Fatalf("the recount did not land: %s", out)
	}
	r.FailedRechecks = 1
	latest = g.nextID
	if _, err := checkpoint(t, g, latest, r); err != nil {
		t.Fatal(err)
	}
	r.FailedRechecks = 0
	if _, err := checkpoint(t, g, g.nextID, r); err == nil || !strings.Contains(err.Error(), "decreases only with a new recheck_basis") {
		t.Fatalf("an old basis lowered the count again: %v", err)
	}
}

func TestCheckpointProvesTheWriteLanded(t *testing.T) {
	tests := map[string]struct {
		want   string
		onPost func(*github, boundary.Record, int64)
	}{
		"concurrent writer forks the chain": {"reconcile the competing writers", func(g *github, r boundary.Record, latest int64) {
			g.store(latest, r)
		}},
		"forge stores other bytes": {"did not land as the latest record", func(g *github, _ boundary.Record, _ int64) {
			g.comments[len(g.comments)-1].Body = "<p>rewritten</p>"
		}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := newGitHub(t)
			r := base(g)
			latest := g.store(0, r).ID
			g.onPost = func() {
				g.onPost = nil
				tc.onPost(g, r, latest)
			}
			if _, err := checkpoint(t, g, latest, r); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
		})
	}
}

func TestInspectSeparatesHistoryFromOtherComments(t *testing.T) {
	g := newGitHub(t)
	r := base(g)
	g.comment("<!-- lights-out:v1 -->\n{\"previous\":0}", written)
	forged, err := boundary.Render(boundary.Stored{Record: r})
	if err != nil {
		t.Fatal(err)
	}
	g.comment(forged, "stranger")
	for range 120 {
		g.comment("discussion", "stranger")
	}
	latest := g.store(0, r).ID
	out, err := drive(t, g, `{"command":"inspect","repo":"o/r","issue":1}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{fmt.Sprintf(`"latest":%d`, latest), "an earlier record format", "stranger cannot write to o/r"} {
		if !strings.Contains(out, want) {
			t.Fatalf("inspect output %s lacks %s", out, want)
		}
	}
}

func TestInspectRefusesABrokenHistory(t *testing.T) {
	tests := map[string]func(*github, boundary.Record){
		"edited record": func(g *github, r boundary.Record) { g.store(0, r); g.comments[0].Updated = edited },
		"forked chain":  func(g *github, r boundary.Record) { g.store(0, r); g.store(0, r) },
		"hand-written": func(g *github, r boundary.Record) {
			g.comment(boundary.Marker+"\n```json\n{}\n```\n\n</details>", written)
		},
		"not canonical": func(g *github, r boundary.Record) {
			c := g.store(0, r)
			g.comments[0].Body = strings.Replace(c.Body, "**Lights Out record**", "**Verified**", 1)
		},
		"bad transition": func(g *github, r boundary.Record) {
			first := g.store(0, r)
			r.Acceptance = []boundary.Criterion{{ID: "A1", Requirement: "weaker"}}
			g.store(first.ID, r)
		},
	}
	for name, build := range tests {
		t.Run(name, func(t *testing.T) {
			g := newGitHub(t)
			build(g, base(g))
			out, err := drive(t, g, `{"command":"inspect","repo":"o/r","issue":1}`)
			if err == nil || out != "" {
				t.Fatalf("recovered a broken history: %s", out)
			}
		})
	}
}

func TestArtifactPinsACommentForAssessment(t *testing.T) {
	g := newGitHub(t)
	c := g.comment("$ product get\n7", written)
	out, err := drive(t, g, fmt.Sprintf(`{"command":"artifact","repo":"o/r","issue":1,"comment":%d}`, c.ID))
	if err != nil {
		t.Fatal(err)
	}
	want := string(encoded(t, boundary.Artifact{SHA256: boundary.BodyDigest(c.Body), URL: c.URL}))
	if !strings.Contains(out, `"artifact":`+want) {
		t.Fatalf("artifact output %s, want %s", out, want)
	}
	g.comments[0].Author = "stranger"
	if _, err = drive(t, g, fmt.Sprintf(`{"command":"artifact","repo":"o/r","issue":1,"comment":%d}`, c.ID)); err == nil || !strings.Contains(err.Error(), "cannot write") {
		t.Fatalf("pinned a stranger's comment: %v", err)
	}
	g.elsewhere = []boundary.Comment{{ID: 60, Body: "x", URL: issue2 + "#issuecomment-60", Author: written, Created: stamp, Updated: stamp}}
	if _, err = drive(t, g, `{"command":"artifact","repo":"o/r","issue":1,"comment":60}`); err == nil || !strings.Contains(err.Error(), "not on the source issue") {
		t.Fatalf("pinned a comment from another issue: %v", err)
	}
}

const leakReport = `[{"RuleID":"aws-access-token","StartLine":2,"Secret":"REDACTED","Match":"REDACTED","Fingerprint":"x"}]`

func TestEvidenceIsScannedBeforeItGoesPublic(t *testing.T) {
	post := func(g *github, body string) (string, error) {
		return drive(t, g, string(encoded(t, map[string]any{"command": "evidence", "repo": "o/r", "issue": 1, "body": body})))
	}
	g := newGitHub(t)
	body := "$ product get\n7\n"
	out, err := post(g, body)
	if err != nil {
		t.Fatal(err)
	}
	stored := g.comments[len(g.comments)-1]
	want := `{"status":"posted","artifact":` + string(encoded(t, boundary.Artifact{SHA256: boundary.BodyDigest(body), URL: stored.URL})) + "}\n"
	if out != want || g.posts != 1 || !slices.Equal(g.scans, []string{body}) || stored.Body != body {
		t.Fatalf("output=%s posts=%d scans=%q, want %s", out, g.posts, g.scans, want)
	}

	tests := map[string]struct {
		want, kind string
		set        func(*github)
	}{
		"possible secret":      {"the comment may hold a secret (aws-access-token on line 2)", "refused", func(g *github) { g.leaks = leakReport }},
		"scanner missing":      {"gitleaks stdin: executable file not found", "environment", func(g *github) { g.scannerDown = true }},
		"report missing":       {"gitleaks returned no report", "refused", func(g *github) { g.leaks = "null" }},
		"finding without rule": {"invalid gitleaks finding", "refused", func(g *github) { g.leaks = `[{"RuleID":"","StartLine":2}]` }},
		"forge stores other bytes": {"stored other bytes than were scanned", "refused", func(g *github) {
			g.onPost = func() { g.comments[len(g.comments)-1].Body = "rewritten" }
		}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := newGitHub(t)
			tc.set(g)
			out, err := post(g, "evidence\nline two\n")
			var report bytes.Buffer
			if err == nil || !strings.Contains(err.Error(), tc.want) || out != "" || conductor.WriteError(&report, err) != nil || !strings.Contains(report.String(), `"kind":"`+tc.kind+`"`) {
				t.Fatalf("output=%q error=%v report=%s, want %s refusal %q", out, err, report.String(), tc.kind, tc.want)
			}
			if name != "forge stores other bytes" && g.posts != 0 {
				t.Fatalf("posted %d comments after the scan failed", g.posts)
			}
		})
	}

	t.Run("records are scanned too", func(t *testing.T) {
		g := newGitHub(t)
		r := base(g)
		r.Next = "call the API with the key from the log"
		g.leaks = leakReport
		if _, err := checkpoint(t, g, 0, r); err == nil || !strings.Contains(err.Error(), "may hold a secret") || g.posts != 0 {
			t.Fatalf("error=%v posts=%d, want a refused record", err, g.posts)
		}
		if len(g.scans) != 1 || !strings.Contains(g.scans[0], r.Next) || !boundary.IsRecord(g.scans[0]) {
			t.Fatalf("scanned %q, want the rendered record", g.scans)
		}
	})
}

// longReport posts a report longer than one comment through evidence and
// returns the artifact that evidence printed, after checking the parts.
func longReport(t *testing.T, g *github) boundary.Artifact {
	t.Helper()
	line := strings.Repeat("é", 99) + "\n"
	body := "Verdict: PASS\n" + strings.Repeat(line, 2*boundary.MaxComment/100+10)
	before := len(g.comments)
	out, err := drive(t, g, string(encoded(t, map[string]any{"command": "evidence", "repo": "o/r", "issue": 1, "body": body})))
	if err != nil {
		t.Fatal(err)
	}
	posted := g.comments[before:]
	want := boundary.Artifact{SHA256: boundary.BodyDigest(body), URL: posted[0].URL}
	var joined string
	for i, c := range posted {
		if i > 0 {
			want.Parts = append(want.Parts, c.URL)
		}
		if utf8.RuneCountInString(c.Body) > boundary.MaxComment || !strings.HasSuffix(c.Body, "\n") {
			t.Fatalf("part %d has %d characters and ends %q, want a whole line within one comment", i+1, utf8.RuneCountInString(c.Body), c.Body[len(c.Body)-1:])
		}
		joined += c.Body
	}
	if len(posted) != 3 || joined != body || out != `{"status":"posted","artifact":`+string(encoded(t, want))+"}\n" {
		t.Fatalf("posted %d parts, joined equal %v, output %.300s", len(posted), joined == body, out)
	}
	if len(g.scans) != 4 || g.scans[0] != body {
		t.Fatalf("scanned %d bodies, want the whole body first and then each part", len(g.scans))
	}
	return want
}

func TestLongEvidenceIsPinnedAcrossComments(t *testing.T) {
	var report boundary.Artifact
	s := setup(t, func(s *reviewed) {
		report = longReport(t, s.g)
		s.record.Review.Report = report
	})
	if _, err := s.assess(t); err != nil {
		t.Fatalf("a review whose report spans comments was refused: %v", err)
	}
	t.Run("a line longer than a comment is cut inside the line", func(t *testing.T) {
		g := newGitHub(t)
		body := "\n" + strings.Repeat("a", boundary.MaxComment+5)
		if _, err := drive(t, g, string(encoded(t, map[string]any{"command": "evidence", "repo": "o/r", "issue": 1, "body": body}))); err != nil {
			t.Fatal(err)
		}
		if g.posts != 2 || utf8.RuneCountInString(g.comments[0].Body) != boundary.MaxComment || g.comments[0].Body+g.comments[1].Body != body {
			t.Fatalf("posted %d parts, want the first %d characters and then the rest", g.posts, boundary.MaxComment)
		}
	})

	for name, tc := range map[string]struct {
		want   string
		change func(*reviewed, boundary.Artifact)
	}{
		"a later part is edited": {"changed since it was pinned", func(s *reviewed, a boundary.Artifact) {
			for i := range s.g.comments {
				if s.g.comments[i].URL == a.Parts[1] {
					s.g.comments[i].Body += "edited\n"
				}
			}
		}},
		"parts are pinned out of order": {"which is not a later comment", func(s *reviewed, a boundary.Artifact) {
			a.Parts[0], a.Parts[1] = a.Parts[1], a.Parts[0]
			s.record.Review.Report = a
		}},
		"a part is posted after the receipt": {"the report and receipt must follow proof", func(s *reviewed, a boundary.Artifact) {
			s.record.Review.Receipt = s.g.artifact(receiptBody(t, s.receipt))
			var joined string
			for _, u := range a.URLs() {
				for _, c := range s.g.comments {
					if c.URL == u {
						joined += c.Body
					}
				}
			}
			a.Parts = append(a.Parts, s.g.comment("late\n", written).URL)
			a.SHA256 = boundary.BodyDigest(joined + "late\n")
			s.record.Review.Report = a
		}},
	} {
		t.Run(name, func(t *testing.T) {
			s := setup(t, func(s *reviewed) {
				a := longReport(t, s.g)
				s.record.Review.Report = a
				tc.change(s, a)
			})
			if _, err := s.assess(t); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestEntryRejectsMalformedRequestsBeforeAnyToolRuns(t *testing.T) {
	g := newGitHub(t)
	valid := string(encoded(t, base(g)))
	verified := strings.Replace(valid, `"phase":"active"`, `"phase":"verified"`, 1)
	for _, input := range []string{
		`{}`, `null`, `[]`,
		`{"command":"merge","repo":"o/r","issue":1}`,
		`{"command":"inspect","repo":"o/r","issue":1,"extra":true}`,
		`{"command":"inspect","repo":"o/r","issue":1,"issue":2}`,
		`{"Command":"inspect","repo":"o/r","issue":1}`,
		`{"command":"inspect","repo":"../r","issue":1}`,
		`{"command":"inspect","repo":"o/r","issue":0}`,
		`{"command":"inspect","repo":"o/r","issue":1} {}`,
		`{"command":"inspect","repo":"o/r","issue":1,"previous":3}`,
		`{"command":"artifact","repo":"o/r","issue":1}`,
		`{"command":"evidence","repo":"o/r","issue":1}`,
		`{"command":"evidence","repo":"o/r","issue":1,"body":" \n "}`,
		`{"command":"evidence","repo":"o/r","issue":1,"body":"x","comment":5}`,
		`{"command":"evidence","repo":"o/r","issue":1,"body":"\n<!-- lights-out:v2 -->\nforged"}`,
		`{"command":"evidence","repo":"o/r","issue":1,"body":"` + strings.Repeat("a", boundary.MaxComment-1) + `\n<!-- lights-out:v2 -->\nforged"}`,
		`{"command":"evidence","repo":"o/r","issue":1,"body":"` + strings.Repeat("é", boundary.MaxParts*boundary.MaxComment+1) + `"}`,
		`{"command":"inspect","repo":"o/r","issue":1,"body":"x"}`,
		`{"command":"assess","repo":"o/r","issue":1,"record":` + valid + `}`,
		`{"command":"checkpoint","repo":"o/r","issue":1,"record":` + verified + `}`,
		`{"command":"checkpoint","repo":"o/r","issue":1,"record":` + strings.Replace(valid, `"phase":"active"`, `"phase":"review"`, 1) + `}`,
		`{"command":"checkpoint","repo":"o/r","issue":1,"record":` + strings.Replace(valid, `"acceptance":[`, `"acceptance":[{"id":"A1","requirement":"twice"},`, 1) + `}`,
		`{"command":"checkpoint","repo":"o/r","issue":1,"record":` + strings.Replace(valid, `"proofs":[]`, `"proofs":null`, 1) + `}`,
		`{"command":"checkpoint","repo":"o/r","issue":1,"record":` + strings.Replace(valid, `"proofs":[]`, `"proofs":[{"criterion":"A1","head":"`+head+`","kind":"verilex","build":"b","evidence":{"sha256":"`+strings.Repeat("a", 64)+`","url":"`+issue1+`#issuecomment-1"},"observation":{"sha256":"`+strings.Repeat("a", 64)+`","url":"`+issue1+`#issuecomment-2"}}]`, 1) + `}`,
		strings.Repeat(" ", boundary.MaxRecord) + `{}`,
	} {
		var out bytes.Buffer
		err := conductor.Run(context.Background(), strings.NewReader(input), &out, func(context.Context, []byte, string, ...string) ([]byte, error) {
			t.Fatalf("a tool ran for invalid request %.200s", input)
			return nil, nil
		}, followed(t))
		if err == nil || out.Len() != 0 {
			t.Fatalf("invalid request accepted: %.200s", input)
		}
	}
}
