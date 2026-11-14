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
const Marker = "<!-- lights-out:v3 -->"

// LegacyMarker opens a record in the previous format. A v2 chain is the start
// of the v3 chain: the first v3 record follows the latest v2 record and
// passes Transition, so a task in flight keeps its findings and counts.
const LegacyMarker = "<!-- lights-out:v2 -->"

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

// Set reports whether the artifact is recorded at all.
func (a Artifact) Set() bool { return a.SHA256 != "" || a.URL != "" || len(a.Parts) > 0 }

// span gives the IDs of the artifact's first and last comments, as its URLs
// name them. It is meaningful only for a valid artifact.
func (a Artifact) span() (first, last int64) {
	for i, u := range a.URLs() {
		ref, err := ParseCommentURL(u)
		if err != nil {
			return 0, 0
		}
		if i == 0 {
			first = ref.ID
		}
		last = max(last, ref.ID)
	}
	return first, last
}

// URLs lists the artifact's comments in order.
func (a Artifact) URLs() []string { return append([]string{a.URL}, a.Parts...) }

// Proof ties one acceptance criterion to evidence on the exact head. The only
// kind is "surface": a Verilex proof kind waits for the owner (D13).
// Requirement is the criterion as the proof proves it. When it differs from
// the frozen text, Decision pins the owner's decision that allows it.
type Proof struct {
	Criterion   string    `json:"criterion"`
	Requirement string    `json:"requirement,omitempty"`
	Head        string    `json:"head"`
	Kind        string    `json:"kind"`
	Build       string    `json:"build"`
	Evidence    Artifact  `json:"evidence"`
	Observation Artifact  `json:"observation"`
	Decision    *Artifact `json:"decision,omitempty"`
}

// Suspicion tracks one suspicion of the saved review (ZT-INDEP-6). It stays
// open until the seat By reproduces or disproves it, and Evidence is that
// seat's own review part. A reproduced suspicion names its Finding.
type Suspicion struct {
	ID       string    `json:"id"`
	State    string    `json:"state"`
	By       string    `json:"by,omitempty"`
	Evidence *Artifact `json:"evidence,omitempty"`
	Finding  string    `json:"finding,omitempty"`
}

// Review pins one zero-trust-review run, field by field in the order in which
// the policy requires each part to exist. A run is recorded as it goes: a
// record holds the parts posted so far, and never a part without the parts
// that the policy puts before it.
type Review struct {
	Sync       Artifact `json:"sync,omitzero"`
	Intent     Artifact `json:"intent,omitzero"`
	Suspicions Artifact `json:"suspicions,omitzero"`
	Surface    Artifact `json:"surface,omitzero"`
	Attack     Artifact `json:"attack,omitzero"`
	Report     Artifact `json:"report,omitzero"`
	Receipt    Artifact `json:"receipt,omitzero"`
}

// ReviewPart is one named part of a review run.
type ReviewPart struct {
	Name     string
	Artifact Artifact
}

// stages lists the parts in policy order. The parts of one stage may be
// posted in either order, and each stage follows the whole stage before it.
func (v Review) stages() [][]ReviewPart {
	return [][]ReviewPart{
		{{"sync", v.Sync}}, {{"intent", v.Intent}}, {{"suspicions", v.Suspicions}},
		{{"surface", v.Surface}, {"attack", v.Attack}}, {{"report", v.Report}}, {{"receipt", v.Receipt}},
	}
}

// Missing names the parts that the run does not have yet, in policy order.
func (v Review) Missing() []string {
	var missing []string
	for _, stage := range v.stages() {
		for _, p := range stage {
			if !p.Artifact.Set() {
				missing = append(missing, p.Name)
			}
		}
	}
	return missing
}

// validate checks that the run is a prefix of the policy's order, and that
// its comments and the proofs were posted in that order. Comment IDs grow
// with time, and each artifact URL names its comment ID, so the order needs
// no forge call.
func (v Review) validate(proofs []Proof) error {
	gap, before := "", int64(0)
	for i, stage := range v.stages() {
		first, last, missing := int64(0), int64(0), ""
		for _, p := range stage {
			if !p.Artifact.Set() {
				missing = p.Name
				continue
			}
			if gap != "" {
				return fmt.Errorf("review part %s needs %s first: record the review in the policy's order", p.Name, gap)
			}
			if err := p.Artifact.Validate(); err != nil {
				return fmt.Errorf("review part %s: %w", p.Name, err)
			}
			f, l := p.Artifact.span()
			if first == 0 || f < first {
				first = f
			}
			last = max(last, l)
		}
		if gap == "" {
			gap = missing
		}
		if first == 0 {
			continue
		}
		if first <= before && i <= 2 {
			return errors.New("the sync, the saved intent reading and the saved review must be posted in that order (ZT-INDEP-1)")
		}
		if first <= before {
			return errors.New("proof must follow the saved review, and the report and receipt must follow proof (ZT-INDEP-7)")
		}
		before = last
	}
	_, saved := v.Suspicions.span()
	receipt, _ := v.Receipt.span()
	for _, p := range proofs {
		for _, a := range []Artifact{p.Evidence, p.Observation} {
			first, last := a.span()
			if v.Suspicions.Set() && first <= saved || v.Receipt.Set() && last >= receipt {
				return fmt.Errorf("proof %s of %s was not posted between the saved review and the receipt", a.URL, p.Criterion)
			}
		}
	}
	return nil
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
	Reopened       *Artifact   `json:"reopened,omitempty"`
	Waiting        *Artifact   `json:"waiting,omitempty"`
	Answer         *Artifact   `json:"answer,omitempty"`
	ContractBasis  *Artifact   `json:"contract_basis,omitempty"`
	Suspicions     []Suspicion `json:"suspicions,omitempty"`
	Next           string      `json:"next"`
	Outcome        *Outcome    `json:"outcome,omitempty"`
}

// Pin is one named artifact that a record pins on its own, outside a proof,
// a finding or the review.
type Pin struct {
	Name     string
	Artifact *Artifact
}

// Pins lists the record's own pins: the question a blocked task waits on,
// and the decisions and reasons that allowed a guarded change.
func (r Record) Pins() []Pin {
	return []Pin{{"recheck_basis", r.RecheckBasis}, {"reopened", r.Reopened}, {"waiting", r.Waiting}, {"answer", r.Answer}, {"contract_basis", r.ContractBasis}}
}

type Stored struct {
	Previous int64  `json:"previous"`
	Record   Record `json:"record"`
}

func (r Record) Validate() error { return r.validate(false) }

// validate checks a record. A legacy record was valid under the v2 rules, so
// the rules that v3 added do not apply to it: a blocked record without
// waiting, and a review whose order v2 checked only in assess.
func (r Record) validate(legacy bool) error {
	if !hashPattern.MatchString(r.IntentSHA256) {
		return errors.New("intent_sha256 must be the digest that inspect returns")
	}
	if blank(r.Coordinator, r.Session, r.Author, r.CIOwner, r.Next) {
		return errors.New("coordinator, session, author, ci_owner and next are required")
	}
	if r.FailedRechecks < 0 {
		return errors.New("failed_rechecks cannot be negative")
	}
	for _, p := range r.Pins() {
		if p.Artifact != nil {
			if err := p.Artifact.Validate(); err != nil {
				return fmt.Errorf("%s: %w", p.Name, err)
			}
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
	if err := r.validatePhase(legacy); err != nil {
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
		if p.Decision != nil {
			if err := p.Decision.Validate(); err != nil {
				return fmt.Errorf("proof of %s: decision: %w", p.Criterion, err)
			}
		}
		for _, u := range p.Observation.URLs() {
			if slices.Contains(p.Evidence.URLs(), u) {
				return errors.New("a proof's observation must be a separate comment from its evidence")
			}
		}
	}
	if r.Review != nil && legacy {
		v := r.Review
		if err := validArtifacts(v.Sync, v.Intent, v.Suspicions, v.Surface, v.Attack, v.Report, v.Receipt); err != nil {
			return err
		}
	} else if r.Review != nil {
		if err := r.Review.validate(r.Proofs); err != nil {
			return err
		}
	}
	if err := validFindings(r.Findings); err != nil {
		return err
	}
	return validSuspicions(r.Suspicions)
}

var (
	suspicionID      = regexp.MustCompile(`\bS[1-9][0-9]{0,3}\b`)
	wholeSuspicionID = regexp.MustCompile(`^S[1-9][0-9]{0,3}$`)
)

// SuspicionIDs lists the suspicion IDs that a saved review names, in order.
func SuspicionIDs(review string) []string {
	var ids []string
	for _, id := range suspicionID.FindAllString(review, -1) {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func validSuspicions(suspicions []Suspicion) error {
	seen := map[string]bool{}
	for _, s := range suspicions {
		if !wholeSuspicionID.MatchString(s.ID) || seen[s.ID] {
			return fmt.Errorf("suspicion IDs must be unique, such as S1: %q", s.ID)
		}
		seen[s.ID] = true
		switch {
		case s.State == "open" && s.By == "" && s.Evidence == nil && s.Finding == "":
		case s.State == "reproduced" && !blank(s.By, s.Finding) && s.Evidence != nil:
		case s.State == "disproved" && !blank(s.By) && s.Evidence != nil && s.Finding == "":
		default:
			return fmt.Errorf("suspicion %s: open has nothing else; reproduced needs by, evidence and finding; disproved needs by and evidence", s.ID)
		}
		if s.Evidence != nil {
			if err := s.Evidence.Validate(); err != nil {
				return fmt.Errorf("suspicion %s: %w", s.ID, err)
			}
		}
	}
	return nil
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

func (r Record) validatePhase(legacy bool) error {
	if r.FailedRechecks >= 2 && r.Phase != PhaseBlocked && !legacy {
		return errors.New("two re-checks failed: the next step is the owner's re-scope decision (ZT-LOOP-6), so the phase must be blocked")
	}
	if (r.Phase == PhaseBlocked) != (r.Waiting != nil) && !legacy {
		return errors.New("phase blocked needs waiting, the pinned question that the task waits on, and no other phase has one")
	}
	switch r.Phase {
	case PhaseActive, PhaseBlocked:
	case PhaseReview:
		if r.Candidate == nil {
			return errors.New("phase review needs a candidate")
		}
	case PhaseVerified:
		if r.Candidate == nil || r.Review == nil || len(r.Review.Missing()) > 0 || r.Outcome == nil || r.Outcome.Head != r.Candidate.Head || r.Outcome.Checks == nil {
			return errors.New("a verified record needs its candidate, complete review and outcome on the same head")
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
		for _, stage := range v.stages() {
			for _, p := range stage {
				if p.Artifact.Set() {
					all = append(all, p.Artifact)
				}
			}
		}
	}
	for _, f := range r.Findings {
		all = append(all, f.Evidence)
	}
	for _, p := range r.Pins() {
		if p.Artifact != nil {
			all = append(all, *p.Artifact)
		}
	}
	for _, p := range r.Proofs {
		if p.Decision != nil {
			all = append(all, *p.Decision)
		}
	}
	for _, s := range r.Suspicions {
		if s.Evidence != nil {
			all = append(all, *s.Evidence)
		}
	}
	return all
}

// moves lists the phases that may follow each phase. Only assess writes
// verified, and a verified record is followed only by a reopening.
var moves = map[string][]string{
	PhaseActive:   {PhaseActive, PhaseReview, PhaseBlocked},
	PhaseReview:   {PhaseActive, PhaseReview, PhaseBlocked, PhaseVerified},
	PhaseBlocked:  {PhaseActive, PhaseReview, PhaseBlocked},
	PhaseVerified: {PhaseActive},
}

// Transition checks that next may follow old in one issue's chain.
func Transition(old, next Record) error {
	if !slices.Contains(moves[old.Phase], next.Phase) {
		return fmt.Errorf("a %s record cannot be followed by a %s record", old.Phase, next.Phase)
	}
	if old.IntentSHA256 == next.IntentSHA256 && !reflect.DeepEqual(old.Acceptance, next.Acceptance) {
		return errors.New("the acceptance contract changes only after the owner edits the source issue: then the record takes the new intent, the amended contract and a new contract_basis")
	}
	guarded := []struct {
		due                bool
		old, next          *Artifact
		refusal, misplaced string
	}{
		{old.Phase == PhaseVerified, old.Reopened, next.Reopened,
			"a verified task reopens only with a new reopened: the pinned reason that the verified outcome no longer holds",
			"reopened changes only in the checkpoint that reopens a verified task"},
		{old.IntentSHA256 != next.IntentSHA256, old.ContractBasis, next.ContractBasis,
			"the intent or acceptance contract changed: that needs the owner's product decision, pinned as a new contract_basis",
			"contract_basis changes only in the checkpoint that amends the intent or the acceptance contract"},
		{old.Phase == PhaseBlocked && next.Phase != PhaseBlocked, old.Answer, next.Answer,
			"a task leaves phase blocked only with a new answer: the pinned reply to the question it waited on",
			"answer changes only in the checkpoint that leaves phase blocked"},
	}
	for _, g := range guarded {
		same := reflect.DeepEqual(g.old, g.next)
		if g.due && (same || g.next == nil) {
			return errors.New(g.refusal)
		}
		if !g.due && !same {
			return errors.New(g.misplaced)
		}
	}
	if old.Coordinator != next.Coordinator || old.Author != next.Author {
		return errors.New("the accountable coordinator and author identities cannot change")
	}
	if next.FailedRechecks < old.FailedRechecks && (next.RecheckBasis == nil || reflect.DeepEqual(next.RecheckBasis, old.RecheckBasis)) {
		return errors.New("the failed re-check count decreases only with a new recheck_basis: the pinned owner decision that recounts it")
	}
	if err := keptFindings(old.Findings, next.Findings); err != nil {
		return err
	}
	if err := freshReview(old, next); err != nil {
		return err
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

// freshReview checks that a new candidate head starts a new review run. Only
// the sync carries over: it maps the policy, not the candidate. A re-check
// needs a fresh independent review, whose reviewer saves its own reading of
// the intent (ZT-INDEP-1, ZT-LOOP-2).
func freshReview(old, next Record) error {
	if old.Candidate == nil || next.Candidate == nil || old.Candidate.Head == next.Candidate.Head || old.Review == nil || next.Review == nil {
		return nil
	}
	was := old.Review.stages()
	for i, stage := range next.Review.stages() {
		for j, p := range stage {
			if i >= 1 && p.Artifact.Set() && reflect.DeepEqual(p.Artifact, was[i][j].Artifact) {
				return fmt.Errorf("the candidate head changed, but review part %s is from the run on %s: a new head starts a new review run", p.Name, old.Candidate.Head)
			}
		}
	}
	return nil
}

// keptFindings checks that next still lists every finding of old. A finding
// changes scope only while it stays open, and a closed finding changes only
// by reopening, so no defect leaves the record or changes its verdict in one
// step.
func keptFindings(old, next []Finding) error {
	now := map[string]Finding{}
	for _, f := range next {
		now[f.ID] = f
	}
	for _, was := range old {
		f, listed := now[was.ID]
		switch {
		case !listed:
			return fmt.Errorf("finding %s left the record: a finding stays listed, so close it as fixed, disproved, filed or handed-off", was.ID)
		case f.Scope != was.Scope && (f.Disposition != "open" || was.Disposition != "open"):
			return fmt.Errorf("finding %s changed scope from %s to %s: only an open finding changes scope", was.ID, was.Scope, f.Scope)
		case was.Disposition != "open" && f.Disposition != was.Disposition && f.Disposition != "open":
			return fmt.Errorf("finding %s moved from %s to %s: reopen it first", was.ID, was.Disposition, f.Disposition)
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

// IsLegacyRecord reports a comment in the previous record format, v2.
func IsLegacyRecord(body string) bool { return strings.HasPrefix(body, LegacyMarker+"\n") }

// IsRetiredRecord reports a record comment in a format older than v2.
func IsRetiredRecord(body string) bool {
	return strings.HasPrefix(body, markerPrefix) && !IsRecord(body) && !IsLegacyRecord(body)
}

const (
	fenceOpen  = "```json\n"
	fenceClose = "\n```\n\n</details>"
)

// Render is the one canonical comment form of a stored record: a summary line
// for people, and the typed record folded below it.
func Render(s Stored) (string, error) { return render(s, false) }

func render(s Stored, legacy bool) (string, error) {
	if err := s.Record.validate(legacy); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	marker := Marker
	if legacy {
		marker = LegacyMarker
	}
	return marker + "\n" + summary(s.Record) + "\n\n<details><summary>Typed record</summary>\n\n" + fenceOpen + string(data) + fenceClose, nil
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
func StoredFrom(body string) (Stored, error) { return storedFrom(body, false) }

// LegacyStoredFrom reads a v2 record the same way, under the v2 rules.
func LegacyStoredFrom(body string) (Stored, error) { return storedFrom(body, true) }

func storedFrom(body string, legacy bool) (Stored, error) {
	var s Stored
	start, end := strings.Index(body, fenceOpen), strings.LastIndex(body, fenceClose)
	if (legacy && !IsLegacyRecord(body)) || (!legacy && !IsRecord(body)) || start < 0 || end < start {
		return s, errors.New("record comment is malformed")
	}
	if err := decode([]byte(body[start+len(fenceOpen):end]), &s, true); err != nil {
		return s, err
	}
	if s.Previous < 0 {
		return s, errors.New("record names a negative predecessor")
	}
	if err := s.Record.validate(legacy); err != nil {
		return s, err
	}
	canonical, err := render(s, legacy)
	if err != nil {
		return s, err
	}
	if canonical != body {
		return s, errors.New("record comment is not in canonical form")
	}
	return s, nil
}
