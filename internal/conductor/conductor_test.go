package conductor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/DereKk8/lights-out/internal/boundary"
	"github.com/DereKk8/lights-out/internal/conductor"
)

const head = "1111111111111111111111111111111111111111"
const other = "2222222222222222222222222222222222222222"

type forge struct {
	t          *testing.T
	issue      boundary.Issue
	comments   []boundary.Comment
	evidence   []boundary.Comment
	posted     *boundary.Stored
	checks     []boundary.Check
	pull       boundary.Pull
	secondHead string
	pulls      int
	fail       bool
}

func encoded(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func digest(t *testing.T, v any) string {
	t.Helper()
	s, err := conductor.Digest(v)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func (f *forge) run(_ context.Context, input []byte, name string, args ...string) ([]byte, error) {
	if f.fail {
		return nil, errors.New("forge unavailable")
	}
	if name != "gh" {
		f.t.Fatalf("unexpected executable %s", name)
	}
	joined := strings.Join(args, " ")
	switch {
	case args[0] == "api" && args[1] == "repos/o/r/issues/1":
		return encoded(f.t, f.issue), nil
	case strings.Contains(joined, "comments?per_page"):
		return encoded(f.t, f.comments), nil
	case strings.Contains(joined, "--method POST"):
		if f.posted == nil {
			f.t.Fatal("unexpected write")
		}
		body := boundary.Marker + string(encoded(f.t, *f.posted))
		expected := encoded(f.t, struct {
			Body string `json:"body"`
		}{body})
		if !bytes.Equal(input, expected) {
			f.t.Fatalf("wrong posted record: %s", input)
		}
		c := boundary.Comment{ID: 20, Body: body, URL: "https://github.com/o/r/issues/1#issuecomment-20"}
		f.comments = append(f.comments, c)
		return encoded(f.t, c), nil
	case strings.HasPrefix(args[1], "repos/o/r/issues/comments/"):
		for _, e := range f.evidence {
			if args[1] == fmt.Sprintf("repos/o/r/issues/comments/%d", e.ID) {
				return encoded(f.t, e), nil
			}
		}
		return nil, errors.New("evidence not found")
	case args[0] == "pr" && args[1] == "view":
		p := f.pull
		f.pulls++
		if f.pulls > 1 && f.secondHead != "" {
			p.Head = f.secondHead
		}
		return encoded(f.t, p), nil
	case args[0] == "pr" && args[1] == "checks":
		return encoded(f.t, f.checks), nil
	case strings.HasPrefix(args[1], "repos/o/r/commits/"):
		return encoded(f.t, boundary.Commit{SHA: head}), nil
	default:
		f.t.Fatalf("unexpected tool request: %v", args)
	}
	return nil, errors.New("unreachable")
}
func (f *forge) artifact(body string) boundary.Artifact {
	id := int64(100 + len(f.evidence))
	url := fmt.Sprintf("https://github.com/o/r/issues/1#issuecomment-%d", id)
	f.evidence = append(f.evidence, boundary.Comment{ID: id, Body: body, URL: url})
	return boundary.Artifact{URL: url, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(body)))}
}
func setup(t *testing.T) (*forge, boundary.Record) {
	f := &forge{t: t, issue: boundary.Issue{Number: 1, Title: "Preserve a value", Body: "The CLI must preserve the stored value.", URL: "https://github.com/o/r/issues/1"}, comments: []boundary.Comment{}, checks: []boundary.Check{{Name: "ci", Bucket: "pass", Link: "https://github.com/o/r/actions/runs/1"}}, pull: boundary.Pull{Head: head, Base: "main", State: "OPEN", URL: "https://github.com/o/r/pull/2"}}
	a := f.artifact("$ product get\nexpected: 7\nobserved: 7\nexit: 0\nindependent database read: 7")
	r := boundary.Record{Version: 1, IntentSHA256: conductor.Intent(f.issue), Coordinator: "team-1", Author: "author-session", CIOwner: "team-1", Phase: "review", Acceptance: []boundary.Criterion{{ID: "A1", Requirement: "preserve stored value"}}, Candidate: boundary.Candidate{Kind: "change", Head: head, PR: 2}, Proofs: []boundary.Proof{{Criterion: "A1", Head: head, Kind: "surface", Build: "binary-sha256", Evidence: a, Observation: a}}, Findings: []boundary.Finding{}, Next: "assess"}
	r.Review = &boundary.Review{Identity: boundary.Policy, Head: head, Verdict: "PASS", Reviewer: "review-session", Verifier: "proof-session", AuthorFamily: "GPT", ReviewerFamily: "GPT", SameFamilyReason: "Owner-approved pilot uses fresh sessions", Intent: a, Surface: a, Attack: a, Sync: a, Gaps: []string{}}
	receipt(f, &r)
	persist(f, r)
	return f, r
}
func receipt(f *forge, r *boundary.Record) {
	v := r.Review
	if v == nil {
		return
	}
	r.Review.Report = f.artifact(string(encoded(f.t, boundary.Receipt{Policy: v.Identity, Head: v.Head, Verdict: v.Verdict, ContractSHA256: digest(f.t, r.Acceptance), ProofsSHA256: digest(f.t, r.Proofs), Author: r.Author, Reviewer: v.Reviewer, Verifier: v.Verifier, Gaps: []string{}, Report: "independent report with product proof and attacks"})))
}
func persist(f *forge, r boundary.Record) {
	f.comments = []boundary.Comment{{ID: 10, Body: boundary.Marker + string(encoded(f.t, boundary.Stored{Record: r})), URL: "https://github.com/o/r/issues/1#issuecomment-10"}}
}
func drive(t *testing.T, f *forge, req boundary.Request) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := conductor.Run(context.Background(), bytes.NewReader(encoded(t, req)), &out, f.run)
	return out.String(), err
}
func assess(t *testing.T, f *forge, r boundary.Record) (string, error) {
	return drive(t, f, boundary.Request{Command: "assess", Repo: "o/r", Issue: 1, Previous: 10, Record: &r})
}
func TestAssessmentAndNoChangeThroughEntryPoint(t *testing.T) {
	for _, kind := range []string{"change", "no-change"} {
		t.Run(kind, func(t *testing.T) {
			f, r := setup(t)
			r.Candidate.Kind = kind
			if kind == "no-change" {
				r.Candidate.PR = 0
			}
			persist(f, r)
			out, err := assess(t, f, r)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, `"status":"verified"`) || !strings.Contains(out, `"head":"`+head+`"`) || !strings.Contains(out, `"merge_authorized":false`) {
				t.Fatalf("wrong observed assessment %s", out)
			}
		})
	}
}
func TestGateRejectsUnsupportedAcceptance(t *testing.T) {
	tests := []struct {
		name, want string
		change     func(*forge, *boundary.Record)
	}{
		{"missing review", "independent review is missing", func(f *forge, r *boundary.Record) { r.Review = nil }},
		{"stale review", "exact candidate", func(f *forge, r *boundary.Record) { r.Review.Head = other }},
		{"failed verdict", "not PASS", func(f *forge, r *boundary.Record) { r.Review.Verdict = "INCOMPLETE" }},
		{"author reviewer", "author cannot", func(f *forge, r *boundary.Record) { r.Review.Reviewer = r.Author }},
		{"author verifier", "author cannot", func(f *forge, r *boundary.Record) { r.Review.Verifier = r.Author }},
		{"policy drift", "policy identity", func(f *forge, r *boundary.Record) { r.Review.Identity = "other" }},
		{"sync gap", "sync gaps", func(f *forge, r *boundary.Record) { r.Review.Gaps = []string{"ZT-PROOF-1"} }},
		{"family reason", "same-family", func(f *forge, r *boundary.Record) { r.Review.SameFamilyReason = "" }},
		{"missing proof", "unsupported acceptance", func(f *forge, r *boundary.Record) { r.Proofs = []boundary.Proof{}; receipt(f, r) }},
		{"stale proof", "stale proof revision", func(f *forge, r *boundary.Record) { r.Proofs[0].Head = other; receipt(f, r) }},
		{"receipt does not endorse proof", "receipt does not endorse", func(f *forge, r *boundary.Record) { r.Proofs[0].Build = "changed-binary" }},
		{"edited evidence", "evidence content changed", func(f *forge, r *boundary.Record) { f.evidence[0].Body = "edited" }},
		{"unsupported evidence origin", "source issue", func(f *forge, r *boundary.Record) {
			r.Review.Attack.URL = "https://github.com/other/repo/issues/1#issuecomment-100"
		}},
		{"open finding", "unresolved finding", func(f *forge, r *boundary.Record) {
			r.Findings = []boundary.Finding{{ID: "S1", Scope: "in", Owner: "author", Disposition: "open", Evidence: "trigger and harm"}}
		}},
		{"unaccepted handoff", "has not been accepted", func(f *forge, r *boundary.Record) {
			r.Findings = []boundary.Finding{{ID: "S2", Scope: "out", Owner: "other team", Disposition: "assigned", Evidence: "trigger and harm"}}
		}},
		{"recheck bound", "owner re-scope", func(f *forge, r *boundary.Record) { r.FailedRechecks = 2 }},
		{"moved head", "candidate is stale", func(f *forge, r *boundary.Record) { f.pull.Head = other }},
		{"closed PR", "not open", func(f *forge, r *boundary.Record) { f.pull.State = "CLOSED" }},
		{"head moves during checks", "changed during assessment", func(f *forge, r *boundary.Record) { f.secondHead = other }},
		{"failed check", "required check ci is fail", func(f *forge, r *boundary.Record) { f.checks[0].Bucket = "fail" }},
		{"skipped check", "required check ci is skipping", func(f *forge, r *boundary.Record) { f.checks[0].Bucket = "skipping" }},
		{"no checks", "no required checks", func(f *forge, r *boundary.Record) { f.checks = []boundary.Check{} }},
		{"environment failure", "forge unavailable", func(f *forge, r *boundary.Record) { f.fail = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, r := setup(t)
			tc.change(f, &r)
			persist(f, r)
			out, err := assess(t, f, r)
			if err == nil || !strings.Contains(err.Error(), tc.want) || out != "" {
				t.Fatalf("got output=%q error=%v, want refusal %q", out, err, tc.want)
			}
		})
	}
}
func TestCheckpointRecoveryAndFrozenContract(t *testing.T) {
	f, r := setup(t)
	r.Next = "independent proof"
	f.posted = &boundary.Stored{Previous: 10, Record: r}
	out, err := drive(t, f, boundary.Request{Command: "checkpoint", Repo: "o/r", Issue: 1, Previous: 10, Record: &r})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"latest":20`) {
		t.Fatal(out)
	}
	// A fresh invocation has no memory of the posting invocation.
	out, err = drive(t, f, boundary.Request{Command: "inspect", Repo: "o/r", Issue: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"next":"independent proof"`) || !strings.Contains(out, `"latest":20`) {
		t.Fatal(out)
	}
	r.Acceptance[0].Requirement = "weaker requirement"
	_, err = drive(t, f, boundary.Request{Command: "checkpoint", Repo: "o/r", Issue: 1, Previous: 20, Record: &r})
	if err == nil || !strings.Contains(err.Error(), "acceptance contract changed") {
		t.Fatal(err)
	}
}
func TestRecoveryRejectsForksAndMalformedRecords(t *testing.T) {
	for _, body := range []string{boundary.Marker + `{}`, "fork"} {
		t.Run(body, func(t *testing.T) {
			f, r := setup(t)
			if body == "fork" {
				body = boundary.Marker + string(encoded(t, boundary.Stored{Previous: 0, Record: r}))
			}
			f.comments = append(f.comments, boundary.Comment{ID: 11, URL: "https://github.com/o/r/issues/1#issuecomment-11", Body: body})
			out, err := drive(t, f, boundary.Request{Command: "inspect", Repo: "o/r", Issue: 1})
			if err == nil || out != "" {
				t.Fatalf("recovered corrupt history: %s %v", out, err)
			}
		})
	}
}
func TestEntryRejectsMalformedInputBeforeTools(t *testing.T) {
	for _, input := range []string{`{}`, `null`, `{"command":"inspect","repo":"o/r","issue":1,"extra":true}`, `{"command":"inspect","repo":"o/r","issue":1,"issue":2}`, `{"Command":"inspect","repo":"o/r","issue":1}`, `{"command":"inspect","repo":"../r","issue":1}`, `{"command":"inspect","repo":"o/r","issue":1} {}`, strings.Repeat(" ", 4<<20) + `{}`} {
		var out bytes.Buffer
		err := conductor.Run(context.Background(), strings.NewReader(input), &out, func(context.Context, []byte, string, ...string) ([]byte, error) {
			t.Fatal("tool ran for invalid request")
			return nil, nil
		})
		if err == nil || out.Len() != 0 {
			t.Fatal("invalid request accepted")
		}
	}
}
