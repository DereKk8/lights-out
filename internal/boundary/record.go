package boundary

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Marker opens every record comment. Its version names the comment format.
const Marker = "<!-- lights-out:v2 -->"

const markerPrefix = "<!-- lights-out:"

const (
	PhaseActive   = "active"
	PhaseReview   = "review"
	PhaseBlocked  = "blocked"
	PhaseVerified = "verified"
)

type Criterion struct {
	ID          string `json:"id"`
	Requirement string `json:"requirement"`
}

// Candidate names the exact revision offered as the resolution: a change in
// a pull request, or an existing revision for a no-change outcome.
type Candidate struct {
	Kind string `json:"kind"`
	Head string `json:"head"`
	PR   int    `json:"pr"`
}

// Artifact pins the exact bytes of evidence on the source issue. Evidence
// longer than one comment continues in Parts, in order, and SHA256 covers
// the bodies of URL and every part joined.
type Artifact struct {
	SHA256 string   `json:"sha256"`
	URL    string   `json:"url"`
	Parts  []string `json:"parts,omitempty"`
}

// URLs lists the artifact's comments in order.
func (a Artifact) URLs() []string { return append([]string{a.URL}, a.Parts...) }

// Proof ties one acceptance criterion to evidence on the exact head. The only
// kind is "surface": a Verilex proof kind waits for the owner (D13).
type Proof struct {
	Criterion   string   `json:"criterion"`
	Head        string   `json:"head"`
	Kind        string   `json:"kind"`
	Build       string   `json:"build"`
	Evidence    Artifact `json:"evidence"`
	Observation Artifact `json:"observation"`
}

// Review pins one zero-trust-review run, field by field in the order in which
// the policy requires each part to exist.
type Review struct {
	Sync       Artifact `json:"sync"`
	Intent     Artifact `json:"intent"`
	Suspicions Artifact `json:"suspicions"`
	Surface    Artifact `json:"surface"`
	Attack     Artifact `json:"attack"`
	Report     Artifact `json:"report"`
	Receipt    Artifact `json:"receipt"`
}

// Finding is a confirmed defect. Evidence holds its trigger, harm and
// smallest disposition. Accepted links the receiving team's record. Filed
// links the tracker issue of a finding below the severity floor (ZT-SEV-1).
type Finding struct {
	ID          string   `json:"id"`
	Scope       string   `json:"scope"`
	Owner       string   `json:"owner"`
	Disposition string   `json:"disposition"`
	Evidence    Artifact `json:"evidence"`
	Accepted    string   `json:"accepted,omitempty"`
	Filed       string   `json:"filed,omitempty"`
}

type Check struct {
	Name   string `json:"name"`
	Bucket string `json:"bucket"`
	Link   string `json:"link"`
}

// Outcome is written only by assess: the verified head and the required
// checks that passed on it.
type Outcome struct {
	Head   string  `json:"head"`
	Checks []Check `json:"checks"`
}

// Record is the state of one issue. Records form an append-only chain of
// issue comments, each naming its predecessor.
type Record struct {
	IntentSHA256   string      `json:"intent_sha256"`
	Coordinator    string      `json:"coordinator"`
	Session        string      `json:"session"`
	Author         string      `json:"author"`
	CIOwner        string      `json:"ci_owner"`
	Phase          string      `json:"phase"`
	Acceptance     []Criterion `json:"acceptance"`
	Candidate      *Candidate  `json:"candidate,omitempty"`
	Proofs         []Proof     `json:"proofs"`
	Review         *Review     `json:"review,omitempty"`
	Findings       []Finding   `json:"findings"`
	FailedRechecks int         `json:"failed_rechecks"`
	RecheckBasis   *Artifact   `json:"recheck_basis,omitempty"`
	Next           string      `json:"next"`
	Outcome        *Outcome    `json:"outcome,omitempty"`
}

type Stored struct {
	Previous int64  `json:"previous"`
	Record   Record `json:"record"`
}

func (r Record) Validate() error {
	if !hashPattern.MatchString(r.IntentSHA256) {
		return errors.New("intent_sha256 must be the digest that inspect returns")
	}
	if blank(r.Coordinator, r.Session, r.Author, r.CIOwner, r.Next) {
		return errors.New("coordinator, session, author, ci_owner and next are required")
	}
	if r.FailedRechecks < 0 {
		return errors.New("failed_rechecks cannot be negative")
	}
	if r.RecheckBasis != nil {
		if err := r.RecheckBasis.Validate(); err != nil {
			return fmt.Errorf("recheck_basis: %w", err)
		}
	}
	if r.Acceptance == nil || r.Proofs == nil || r.Findings == nil {
		return errors.New("acceptance, proofs and findings must be arrays")
	}
	if len(r.Acceptance) == 0 {
		return errors.New("record the acceptance requirements before implementation")
	}
	criteria := map[string]bool{}
	for _, c := range r.Acceptance {
		if blank(c.ID, c.Requirement) || criteria[c.ID] {
			return errors.New("acceptance IDs must be unique and each requirement stated")
		}
		criteria[c.ID] = true
	}
	if err := r.Candidate.validate(); err != nil {
		return err
	}
	if err := r.validatePhase(); err != nil {
		return err
	}
	for _, p := range r.Proofs {
		if !criteria[p.Criterion] {
			return fmt.Errorf("proof names unknown criterion %q", p.Criterion)
		}
		if p.Kind != "surface" {
			return errors.New(`proof kind must be "surface"; the owner deferred Verilex proofs (D13)`)
		}
		if !shaPattern.MatchString(p.Head) || blank(p.Build) {
			return errors.New("proof needs a full head SHA and the tested build")
		}
		if err := validArtifacts(p.Evidence, p.Observation); err != nil {
			return err
		}
		for _, u := range p.Observation.URLs() {
			if slices.Contains(p.Evidence.URLs(), u) {
				return errors.New("a proof's observation must be a separate comment from its evidence")
			}
		}
	}
	if r.Review != nil {
		v := r.Review
		if err := validArtifacts(v.Sync, v.Intent, v.Suspicions, v.Surface, v.Attack, v.Report, v.Receipt); err != nil {
			return err
		}
	}
	return validFindings(r.Findings)
}

func (c *Candidate) validate() error {
	if c == nil {
		return nil
	}
	if !shaPattern.MatchString(c.Head) {
		return errors.New("candidate head must be a full 40-character SHA")
	}
	switch {
	case c.Kind == "change" && c.PR > 0, c.Kind == "no-change" && c.PR == 0:
		return nil
	}
	return errors.New(`candidate kind must be "change" with its pr, or "no-change" with pr 0`)
}

func (r Record) validatePhase() error {
	switch r.Phase {
	case PhaseActive, PhaseBlocked:
	case PhaseReview:
		if r.Candidate == nil {
			return errors.New("phase review needs a candidate")
		}
	case PhaseVerified:
		if r.Candidate == nil || r.Review == nil || r.Outcome == nil || r.Outcome.Head != r.Candidate.Head || r.Outcome.Checks == nil {
			return errors.New("a verified record needs its candidate, review and outcome on the same head")
		}
		for _, c := range r.Outcome.Checks {
			if blank(c.Name) || c.Bucket != "pass" {
				return errors.New("an outcome lists only passed required checks")
			}
		}
		return nil
	default:
		return errors.New("phase must be active, review or blocked")
	}
	if r.Outcome != nil {
		return errors.New("only a verified record has an outcome")
	}
	return nil
}

func validFindings(findings []Finding) error {
	seen := map[string]bool{}
	for _, f := range findings {
		if blank(f.ID, f.Owner) || seen[f.ID] {
			return errors.New("finding IDs must be unique and each finding owned")
		}
		seen[f.ID] = true
		if err := f.Evidence.Validate(); err != nil {
			return fmt.Errorf("finding %s: %w", f.ID, err)
		}
		switch {
		case f.Scope == "in" && (f.Disposition == "open" || f.Disposition == "fixed" || f.Disposition == "disproved") && f.Accepted == "" && f.Filed == "":
		case f.Scope == "in" && f.Disposition == "filed" && f.Accepted == "":
			if _, err := ParseIssueURL(f.Filed); err != nil {
				return fmt.Errorf("finding %s: filed must link its tracker issue: %w", f.ID, err)
			}
		case f.Scope == "out" && f.Disposition == "open" && f.Accepted == "" && f.Filed == "":
		case f.Scope == "out" && f.Disposition == "handed-off" && f.Filed == "":
			if _, err := ParseCommentURL(f.Accepted); err != nil {
				return fmt.Errorf("finding %s: accepted must link the receiving team's record: %w", f.ID, err)
			}
		default:
			return fmt.Errorf("finding %s: scope in is open, fixed, disproved, or filed with its issue; scope out is open, or handed-off with accepted", f.ID)
		}
	}
	return nil
}

func validArtifacts(artifacts ...Artifact) error {
	for _, a := range artifacts {
		if err := a.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (a Artifact) Validate() error {
	if !hashPattern.MatchString(a.SHA256) {
		return errors.New("artifact needs the SHA-256 of its comment body; get it with the artifact command")
	}
	if len(a.Parts) >= MaxParts {
		return fmt.Errorf("artifact spans more than %d comments", MaxParts)
	}
	for _, u := range a.URLs() {
		if _, err := ParseCommentURL(u); err != nil {
			return err
		}
	}
	return nil
}

// Artifacts lists every artifact the record pins.
func (r Record) Artifacts() []Artifact {
	var all []Artifact
	for _, p := range r.Proofs {
		all = append(all, p.Evidence, p.Observation)
	}
	if v := r.Review; v != nil {
		all = append(all, v.Sync, v.Intent, v.Suspicions, v.Surface, v.Attack, v.Report, v.Receipt)
	}
	for _, f := range r.Findings {
		all = append(all, f.Evidence)
	}
	if r.RecheckBasis != nil {
		all = append(all, *r.RecheckBasis)
	}
	return all
}

// Transition checks that next may follow old in one issue's chain.
func Transition(old, next Record) error {
	if old.IntentSHA256 != next.IntentSHA256 || !reflect.DeepEqual(old.Acceptance, next.Acceptance) {
		return errors.New("the intent or acceptance contract changed: that needs the owner's product decision, not a checkpoint")
	}
	if old.Coordinator != next.Coordinator || old.Author != next.Author {
		return errors.New("the accountable coordinator and author identities cannot change")
	}
	if next.FailedRechecks < old.FailedRechecks && (next.RecheckBasis == nil || reflect.DeepEqual(next.RecheckBasis, old.RecheckBasis)) {
		return errors.New("the failed re-check count decreases only with a new recheck_basis: the pinned owner decision that recounts it")
	}
	if next.Phase == PhaseVerified {
		same := next
		same.Phase, same.Next, same.Outcome = old.Phase, old.Next, old.Outcome
		if old.Phase != PhaseReview || !reflect.DeepEqual(old, same) {
			return errors.New("a verified record must repeat the assessed review record")
		}
	}
	return nil
}

var issueURL = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+)/issues/([1-9][0-9]{0,9})$`)

// IssueRef locates one issue.
type IssueRef struct {
	Repo  string
	Issue int
}

func ParseIssueURL(url string) (IssueRef, error) {
	m := issueURL.FindStringSubmatch(url)
	if m == nil {
		return IssueRef{}, fmt.Errorf("%q is not a GitHub issue URL", url)
	}
	issue, err := strconv.Atoi(m[2])
	return IssueRef{m[1], issue}, err
}

var commentURL = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+)/issues/([1-9][0-9]{0,9})#issuecomment-([1-9][0-9]{0,18})$`)

// CommentRef locates one issue comment.
type CommentRef struct {
	Repo  string
	Issue int
	ID    int64
}

func ParseCommentURL(url string) (CommentRef, error) {
	m := commentURL.FindStringSubmatch(url)
	if m == nil {
		return CommentRef{}, fmt.Errorf("%q is not a GitHub issue comment URL", url)
	}
	issue, err := strconv.Atoi(m[2])
	if err != nil {
		return CommentRef{}, err
	}
	id, err := strconv.ParseInt(m[3], 10, 64)
	if err != nil {
		return CommentRef{}, err
	}
	return CommentRef{Repo: m[1], Issue: issue, ID: id}, nil
}

// IsRecord reports a comment in the current record format.
func IsRecord(body string) bool { return strings.HasPrefix(body, Marker+"\n") }

// IsRetiredRecord reports a record comment in an earlier format.
func IsRetiredRecord(body string) bool {
	return strings.HasPrefix(body, markerPrefix) && !IsRecord(body)
}

const (
	fenceOpen  = "```json\n"
	fenceClose = "\n```\n\n</details>"
)

// Render is the one canonical comment form of a stored record: a summary line
// for people, and the typed record folded below it.
func Render(s Stored) (string, error) {
	if err := s.Record.Validate(); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return Marker + "\n" + summary(s.Record) + "\n\n<details><summary>Typed record</summary>\n\n" + fenceOpen + string(data) + fenceClose, nil
}

// The summary uses only validated fields, so free text cannot inject Markdown.
func summary(r Record) string {
	candidate := "no candidate yet"
	if c := r.Candidate; c != nil && c.Kind == "change" {
		candidate = fmt.Sprintf("candidate `%s` in #%d", c.Head[:12], c.PR)
	} else if c != nil {
		candidate = fmt.Sprintf("no-change at `%s`", c.Head[:12])
	}
	open := 0
	for _, f := range r.Findings {
		if f.Disposition == "open" {
			open++
		}
	}
	return fmt.Sprintf("**Lights Out record** · phase `%s` · %s · criteria %d · proofs %d · open findings %d · failed re-checks %d",
		r.Phase, candidate, len(r.Acceptance), len(r.Proofs), open, r.FailedRechecks)
}

// StoredFrom accepts only a comment that is exactly the canonical rendering
// of a valid record, so an edited or hand-written comment fails.
func StoredFrom(body string) (Stored, error) {
	var s Stored
	start, end := strings.Index(body, fenceOpen), strings.LastIndex(body, fenceClose)
	if !IsRecord(body) || start < 0 || end < start {
		return s, errors.New("record comment is malformed")
	}
	if err := decode([]byte(body[start+len(fenceOpen):end]), &s, true); err != nil {
		return s, err
	}
	if s.Previous < 0 {
		return s, errors.New("record names a negative predecessor")
	}
	if err := s.Record.Validate(); err != nil {
		return s, err
	}
	canonical, err := Render(s)
	if err != nil {
		return s, err
	}
	if canonical != body {
		return s, errors.New("record comment is not in canonical form")
	}
	return s, nil
}
