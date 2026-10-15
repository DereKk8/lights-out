// Package conductor connects typed issue records to the existing forge.
package conductor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/DereKk8/lights-out/internal/boundary"
)

type Runner func(context.Context, []byte, string, ...string) ([]byte, error)

// Execute does not invoke a shell. Stderr is not copied: dependencies may emit
// sensitive diagnostics. The command name and exit error identify the failure.
func Execute(ctx context.Context, input []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(input)
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	return data, nil
}

type Response struct {
	Status          string           `json:"status"`
	Error           string           `json:"error,omitempty"`
	Issue           *boundary.Issue  `json:"issue,omitempty"`
	IntentSHA256    string           `json:"intent_sha256,omitempty"`
	Latest          int64            `json:"latest,omitempty"`
	Record          *boundary.Record `json:"record,omitempty"`
	URL             string           `json:"url,omitempty"`
	Head            string           `json:"head,omitempty"`
	MergeAuthorized bool             `json:"merge_authorized"`
	Next            string           `json:"next,omitempty"`
	ContractSHA256  string           `json:"contract_sha256,omitempty"`
	ProofsSHA256    string           `json:"proofs_sha256,omitempty"`
}

// Run is the same public entry point used by the executable and behavior tests.
func Run(ctx context.Context, in io.Reader, out io.Writer, run Runner) error {
	data, err := io.ReadAll(io.LimitReader(in, (4<<20)+1))
	if err != nil {
		return err
	}
	req, err := boundary.RequestFrom(data)
	if err != nil {
		return err
	}
	c := connection{run: run, ctx: ctx, repo: req.Repo, issue: req.Issue}
	result, err := c.command(req)
	if err != nil {
		return err
	}
	if result.Record != nil {
		result.ContractSHA256, err = Digest(result.Record.Acceptance)
		if err != nil {
			return err
		}
		result.ProofsSHA256, err = Digest(result.Record.Proofs)
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(out).Encode(result)
}
func WriteError(out io.Writer, err error) error {
	return json.NewEncoder(out).Encode(Response{Status: "error", Error: err.Error()})
}

type connection struct {
	run   Runner
	ctx   context.Context
	repo  string
	issue int
}

func (c connection) api(input []byte, args ...string) ([]byte, error) {
	return c.run(c.ctx, input, "gh", append([]string{"api"}, args...)...)
}
func (c connection) issuePath() string { return fmt.Sprintf("repos/%s/issues/%d", c.repo, c.issue) }
func (c connection) load() (boundary.Issue, int64, *boundary.Record, error) {
	data, err := c.api(nil, c.issuePath(), "--jq", "{number,title,body,url:.html_url}")
	if err != nil {
		return boundary.Issue{}, 0, nil, err
	}
	issue, err := boundary.IssueFrom(data)
	if err != nil {
		return issue, 0, nil, err
	}
	if issue.Number != c.issue {
		return issue, 0, nil, errors.New("forge returned another issue")
	}
	var latest int64
	var record *boundary.Record
	for page := 1; page <= 100; page++ {
		data, err = c.api(nil, fmt.Sprintf("%s/comments?per_page=100&page=%d", c.issuePath(), page), "--jq", "[.[]|{id,body,url:.html_url}]")
		if err != nil {
			return issue, 0, nil, err
		}
		comments, e := boundary.CommentsFrom(data)
		if e != nil {
			return issue, 0, nil, e
		}
		for _, comment := range comments {
			if !strings.HasPrefix(comment.Body, boundary.Marker) {
				continue
			}
			stored, e := boundary.StoredFrom(comment.Body)
			if e != nil {
				return issue, 0, nil, fmt.Errorf("comment %d: %w", comment.ID, e)
			}
			if stored.Previous != latest {
				return issue, 0, nil, errors.New("forked issue record: reconcile competing coordinators before continuing")
			}
			if record != nil {
				if e = transition(*record, stored.Record); e != nil {
					return issue, 0, nil, e
				}
			}
			r := stored.Record
			record = &r
			latest = comment.ID
		}
		if len(comments) < 100 {
			return issue, latest, record, nil
		}
	}
	return issue, 0, nil, errors.New("issue exceeds 10000 comments; no partial recovery")
}
func Intent(i boundary.Issue) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(i.Title+"\n"+i.Body)))
}
func transition(old, next boundary.Record) error {
	if old.IntentSHA256 != next.IntentSHA256 || !reflect.DeepEqual(old.Acceptance, next.Acceptance) {
		return errors.New("acceptance contract changed: owner product decision required")
	}
	if old.Coordinator != next.Coordinator || old.Author != next.Author {
		return errors.New("accountable coordinator and author identity cannot silently change")
	}
	if next.FailedRechecks < old.FailedRechecks {
		return errors.New("failed re-check count cannot decrease")
	}
	return nil
}
func (c connection) command(req boundary.Request) (Response, error) {
	issue, latest, old, err := c.load()
	if err != nil {
		return Response{}, err
	}
	digest := Intent(issue)
	if req.Command == "inspect" {
		return Response{Status: "recovered", Issue: &issue, IntentSHA256: digest, Latest: latest, Record: old}, nil
	}
	r := *req.Record
	if req.Previous != latest {
		return Response{}, errors.New("stale predecessor: inspect the issue again")
	}
	if r.IntentSHA256 != digest {
		return Response{}, errors.New("source intent changed or does not match the record")
	}
	if old != nil {
		if err = transition(*old, r); err != nil {
			return Response{}, err
		}
	}
	if req.Command == "assess" {
		if old == nil || !reflect.DeepEqual(*old, r) {
			return Response{}, errors.New("checkpoint the exact record before assessment")
		}
		return c.assess(r)
	}
	body, err := json.Marshal(boundary.Stored{Previous: latest, Record: r})
	if err != nil {
		return Response{}, err
	}
	payload, err := json.Marshal(struct {
		Body string `json:"body"`
	}{boundary.Marker + string(body)})
	if err != nil {
		return Response{}, err
	}
	data, err := c.api(payload, c.issuePath()+"/comments", "--method", "POST", "--input", "-", "--jq", "{id,body,url:.html_url}")
	if err != nil {
		return Response{}, err
	}
	comment, err := boundary.CommentFrom(data)
	if err != nil {
		return Response{}, err
	}
	// A concurrent writer can race the append. Detect the fork before reporting it
	// as saved. The next inspect also refuses it, rather than choosing a winner.
	_, after, _, err := c.load()
	if err != nil {
		return Response{}, err
	}
	if after != comment.ID {
		return Response{}, errors.New("record moved during checkpoint; inspect before continuing")
	}
	return Response{Status: "checkpointed", Latest: comment.ID, URL: comment.URL, Record: &r}, nil
}
func (c connection) artifact(a boundary.Artifact) (string, error) {
	// Evidence lives in a GitHub issue comment. Re-fetch it rather than trusting
	// a caller's local file, a URL alone, or a copied hash.
	prefix := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-", c.repo, c.issue)
	if !strings.HasPrefix(a.URL, prefix) {
		return "", errors.New("evidence must be a comment on the source issue")
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(a.URL, prefix), 10, 64)
	if err != nil || id < 1 {
		return "", errors.New("invalid evidence comment ID")
	}
	data, err := c.api(nil, fmt.Sprintf("repos/%s/issues/comments/%d", c.repo, id), "--jq", "{id,body,url:.html_url}")
	if err != nil {
		return "", err
	}
	comment, err := boundary.CommentFrom(data)
	if err != nil {
		return "", err
	}
	if comment.URL != a.URL || comment.ID != id || strings.TrimSpace(comment.Body) == "" {
		return "", errors.New("wrong or empty evidence comment")
	}
	if fmt.Sprintf("%x", sha256.Sum256([]byte(comment.Body))) != a.SHA256 {
		return "", errors.New("evidence content changed or digest is wrong")
	}
	return comment.Body, nil
}
func (c connection) pull(pr int) (boundary.Pull, error) {
	data, err := c.run(c.ctx, nil, "gh", "pr", "view", strconv.Itoa(pr), "--repo", c.repo, "--json", "headRefOid,baseRefName,state,url", "--jq", "{head:.headRefOid,base:.baseRefName,state,url}")
	if err != nil {
		return boundary.Pull{}, err
	}
	return boundary.PullFrom(data)
}
func (c connection) assess(r boundary.Record) (Response, error) {
	if r.FailedRechecks >= 2 {
		return Response{}, errors.New("two failed re-checks: owner re-scope decision required")
	}
	v := r.Review
	if v == nil {
		return Response{}, errors.New("independent review is missing")
	}
	if v.Identity != boundary.Policy || len(v.Gaps) != 0 {
		return Response{}, errors.New("review policy identity mismatch or sync gaps")
	}
	if v.Verdict != "PASS" || v.Head != r.Candidate.Head {
		return Response{}, errors.New("review is not PASS on the exact candidate")
	}
	if v.Reviewer == r.Author || v.Verifier == r.Author {
		return Response{}, errors.New("author cannot occupy a review or proof seat")
	}
	if v.AuthorFamily == v.ReviewerFamily && strings.TrimSpace(v.SameFamilyReason) == "" {
		return Response{}, errors.New("same-family review needs its policy reason")
	}
	for _, f := range r.Findings {
		if f.Scope == "in" && f.Disposition != "fixed" && f.Disposition != "disproved" {
			return Response{}, fmt.Errorf("unresolved finding %s", f.ID)
		}
		if f.Scope == "out" && strings.TrimSpace(f.Accepted) == "" {
			return Response{}, fmt.Errorf("handoff %s has not been accepted", f.ID)
		}
	}
	for _, a := range []boundary.Artifact{v.Intent, v.Surface, v.Attack, v.Sync} {
		if _, err := c.artifact(a); err != nil {
			return Response{}, err
		}
	}
	report, err := c.artifact(v.Report)
	if err != nil {
		return Response{}, err
	}
	receipt, err := boundary.ReceiptFrom([]byte(report))
	if err != nil {
		return Response{}, err
	}
	contractHash, err := Digest(r.Acceptance)
	if err != nil {
		return Response{}, err
	}
	proofHash, err := Digest(r.Proofs)
	if err != nil {
		return Response{}, err
	}
	if receipt.Policy != v.Identity || receipt.Head != v.Head || receipt.Verdict != v.Verdict || receipt.Author != r.Author || receipt.Reviewer != v.Reviewer || receipt.Verifier != v.Verifier || len(receipt.Gaps) != 0 || receipt.ContractSHA256 != contractHash || receipt.ProofsSHA256 != proofHash {
		return Response{}, errors.New("review receipt does not endorse this contract, evidence and candidate")
	}
	covered := map[string]bool{}
	for _, p := range r.Proofs {
		if p.Head != r.Candidate.Head {
			return Response{}, errors.New("stale proof revision")
		}
		if _, err := c.artifact(p.Evidence); err != nil {
			return Response{}, err
		}
		if _, err := c.artifact(p.Observation); err != nil {
			return Response{}, err
		}
		covered[p.Criterion] = true
	}
	for _, criterion := range r.Acceptance {
		if !covered[criterion.ID] {
			return Response{}, fmt.Errorf("unsupported acceptance: %s has no proof", criterion.ID)
		}
	}
	if r.Candidate.Kind == "no-change" {
		data, err := c.api(nil, "repos/"+c.repo+"/commits/"+r.Candidate.Head, "--jq", "{sha}")
		if err != nil {
			return Response{}, err
		}
		commit, err := boundary.CommitFrom(data)
		if err != nil {
			return Response{}, err
		}
		if commit.SHA != r.Candidate.Head {
			return Response{}, errors.New("no-change revision mismatch")
		}
		return Response{Status: "verified", Head: commit.SHA, Next: "No-change outcome verified; publish the outcome and its evidence."}, nil
	}
	before, err := c.pull(r.Candidate.PR)
	if err != nil {
		return Response{}, err
	}
	if before.Head != r.Candidate.Head || before.State != "OPEN" {
		return Response{}, errors.New("candidate is stale or PR is not open")
	}
	data, err := c.run(c.ctx, nil, "gh", "pr", "checks", strconv.Itoa(r.Candidate.PR), "--repo", c.repo, "--required", "--json", "name,bucket,link")
	if err != nil {
		return Response{}, err
	}
	checks, err := boundary.ChecksFrom(data)
	if err != nil {
		return Response{}, err
	}
	for _, check := range checks {
		if check.Bucket != "pass" {
			return Response{}, fmt.Errorf("required check %s is %s", check.Name, check.Bucket)
		}
	}
	after, err := c.pull(r.Candidate.PR)
	if err != nil {
		return Response{}, err
	}
	if after != before {
		return Response{}, errors.New("PR changed during assessment")
	}
	return Response{Status: "verified", Head: r.Candidate.Head, MergeAuthorized: false, Next: "Owner must arrange the separate external zero-trust-review before pilot integration. This assessment never authorizes a merge."}, nil
}

// Digest hashes Go's compact JSON encoding; field order follows the typed record.
func Digest(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
func Timeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Minute)
}
