// Package conductor runs the Lights Out commands. It connects the typed issue
// record to GitHub, which owns the issue, the pull request and its checks.
package conductor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/DereKk8/lights-out/internal/boundary"
)

// Runner runs one tool without a shell and returns its stdout.
type Runner func(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error)

// Execute is the production Runner. A failure names the tool call and the
// tool's own error text, so the reason is visible without a rerun.
func Execute(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		call := name
		for _, a := range args {
			if strings.HasPrefix(a, "-") {
				break
			}
			call += " " + a
		}
		reason := strings.Join(strings.Fields(stderr.String()), " ")
		if len(reason) > 2000 {
			reason = reason[len(reason)-2000:]
		}
		return nil, fmt.Errorf("%s: %w: %s", call, err, reason)
	}
	return out, nil
}

// Followed is the zero-trust-review identity that this build follows. Change
// it only together with a sync-zero-trust run that maps every requirement ID
// for the new version.
const Followed = "Identity: zero-trust-review 6.0.0 948a446a568c2f5353c88a74c1a44fc73cb40244 e2e0757666dc1072dce4e7fb4580d2c3b0900106"

// Timeout bounds one command, including every forge call it makes.
func Timeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Minute)
}

type Response struct {
	Status         string           `json:"status"`
	Policy         string           `json:"policy,omitempty"`
	Issue          *boundary.Issue  `json:"issue,omitempty"`
	IntentSHA256   string           `json:"intent_sha256,omitempty"`
	Latest         int64            `json:"latest"`
	URL            string           `json:"url,omitempty"`
	Record         *boundary.Record `json:"record,omitempty"`
	ContractSHA256 string           `json:"contract_sha256,omitempty"`
	ProofsSHA256   string           `json:"proofs_sha256,omitempty"`
	Ignored        []Ignored        `json:"ignored,omitempty"`
}

// Run is the one entry point, shared by the executable and the tests. policy
// is the zero-trust-review identity that this build of Lights Out follows.
func Run(ctx context.Context, in io.Reader, out io.Writer, run Runner, policy string) error {
	data, err := io.ReadAll(io.LimitReader(in, boundary.MaxRecord+1))
	if err != nil {
		return err
	}
	req, err := boundary.RequestFrom(data)
	if err != nil {
		return err
	}
	f := &forge{run: run, ctx: ctx, repo: req.Repo, issue: req.Issue, writers: map[string]bool{}}
	var result any
	switch req.Command {
	case "inspect":
		result, err = inspect(f, policy)
	case "artifact":
		result, err = artifact(f, req.Comment)
	case "evidence":
		result, err = evidence(f, req.Body)
	case "checkpoint":
		result, err = checkpoint(f, req.Previous, *req.Record)
	case "assess":
		result, err = assess(f, req.Previous, policy)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}

// WriteError reports a failed command as structured output.
// Kind "environment" means a tool call failed, so nothing was proved either
// way. Kind "refused" means Lights Out checked and refused.
func WriteError(out io.Writer, err error) error {
	kind := "refused"
	if errors.As(err, new(unavailable)) {
		kind = "environment"
	}
	return json.NewEncoder(out).Encode(struct {
		Status string `json:"status"`
		Kind   string `json:"kind"`
		Error  string `json:"error"`
	}{"error", kind, err.Error()})
}

func intent(i boundary.Issue) string { return boundary.BodyDigest(i.Title + "\n" + i.Body) }

func recorded(status string, h history) (Response, error) {
	r := Response{Status: status, Latest: h.latest, URL: h.url, Record: h.record, Ignored: h.ignored}
	if h.record == nil {
		return r, nil
	}
	var err error
	if r.ContractSHA256, err = boundary.Digest(h.record.Acceptance); err != nil {
		return r, err
	}
	r.ProofsSHA256, err = boundary.Digest(h.record.Proofs)
	return r, err
}

// inspect recovers the issue's state for any session, with no other memory.
func inspect(f *forge, policy string) (Response, error) {
	h, err := f.load()
	if err != nil {
		return Response{}, err
	}
	r, err := recorded("recovered", h)
	r.Policy, r.Issue, r.IntentSHA256 = policy, &h.issue, intent(h.issue)
	return r, err
}

type pinned struct {
	Status   string            `json:"status"`
	Artifact boundary.Artifact `json:"artifact"`
}

// artifact pins one comment the way assess will check it.
func artifact(f *forge, id int64) (pinned, error) {
	c, err := f.source(id)
	if err != nil {
		return pinned{}, err
	}
	return pinned{"artifact", boundary.Artifact{SHA256: boundary.BodyDigest(c.Body), URL: c.URL}}, nil
}

// evidence posts body on the source issue, in as many comments as it needs,
// and pins them. The post refuses a body that may hold a secret. A body of
// several parts is also scanned whole, so a secret cut across parts is seen.
func evidence(f *forge, body string) (pinned, error) {
	parts, err := boundary.EvidenceParts(body)
	if err != nil {
		return pinned{}, err
	}
	if len(parts) > 1 {
		if err := f.scan(body); err != nil {
			return pinned{}, err
		}
	}
	a := boundary.Artifact{SHA256: boundary.BodyDigest(body)}
	for _, part := range parts {
		c, err := f.post(part)
		if err != nil {
			return pinned{}, err
		}
		if c, err = f.source(c.ID); err != nil {
			return pinned{}, err
		}
		if c.Body != part {
			return pinned{}, fmt.Errorf("comment %s stored other bytes than were scanned", c.URL)
		}
		if a.URL == "" {
			a.URL = c.URL
		} else {
			a.Parts = append(a.Parts, c.URL)
		}
	}
	return pinned{"posted", a}, nil
}

// current loads the chain and checks that the caller saw its latest record
// and that the source intent still matches.
func current(f *forge, previous int64) (history, error) {
	h, err := f.load()
	if err != nil {
		return h, err
	}
	if previous != h.latest {
		return h, fmt.Errorf("stale predecessor %d: the latest record is %d, so inspect again", previous, h.latest)
	}
	if h.record != nil && h.record.IntentSHA256 != intent(h.issue) {
		return h, errors.New("the source issue changed after the contract was recorded: that needs the owner's product decision")
	}
	return h, nil
}

func checkpoint(f *forge, previous int64, next boundary.Record) (Response, error) {
	h, err := current(f, previous)
	if err != nil {
		return Response{}, err
	}
	if next.IntentSHA256 != intent(h.issue) {
		return Response{}, errors.New("intent_sha256 does not match the source issue; copy it from inspect")
	}
	for _, a := range next.Artifacts() {
		for _, u := range a.URLs() {
			ref, err := boundary.ParseCommentURL(u)
			if err != nil {
				return Response{}, err
			}
			if ref.Repo != f.repo || ref.Issue != f.issue {
				return Response{}, fmt.Errorf("evidence %s is not a comment on the source issue", u)
			}
		}
	}
	h, err = f.append(h, next)
	if err != nil {
		return Response{}, err
	}
	return recorded("checkpointed", h)
}
