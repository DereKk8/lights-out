// Package boundary owns JSON decoding and validation of external records.
package boundary

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const Marker = "<!-- lights-out:v1 -->\n"
const Policy = "Identity: zero-trust-review 3.2.0 0c71c96e973420f96912aa61fbbf313cfb5c85db b83638f62ab25178cbf140eebf9a72db00caf562"

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+$`)
var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Request is the entire command surface. Records are never read from prose.
type Request struct {
	Command  string  `json:"command"`
	Repo     string  `json:"repo"`
	Issue    int     `json:"issue"`
	Previous int64   `json:"previous,omitempty"`
	Record   *Record `json:"record,omitempty"`
}
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}
type Comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	URL  string `json:"url"`
}
type Criterion struct {
	ID          string `json:"id"`
	Requirement string `json:"requirement"`
}
type Candidate struct {
	Kind string `json:"kind"`
	Head string `json:"head"`
	PR   int    `json:"pr"`
}
type Finding struct {
	ID          string `json:"id"`
	Scope       string `json:"scope"`
	Owner       string `json:"owner"`
	Disposition string `json:"disposition"`
	Evidence    string `json:"evidence"`
	Accepted    string `json:"accepted"`
}

// Artifact pins evidence bytes on the source issue. Assessment fetches them again.
type Artifact struct {
	SHA256 string `json:"sha256"`
	URL    string `json:"url"`
}
type Proof struct {
	Criterion   string   `json:"criterion"`
	Head        string   `json:"head"`
	Kind        string   `json:"kind"`
	Build       string   `json:"build"`
	Evidence    Artifact `json:"evidence"`
	Observation Artifact `json:"observation"`
}
type Review struct {
	Identity         string   `json:"identity"`
	Head             string   `json:"head"`
	Verdict          string   `json:"verdict"`
	Reviewer         string   `json:"reviewer"`
	Verifier         string   `json:"verifier"`
	AuthorFamily     string   `json:"author_family"`
	ReviewerFamily   string   `json:"reviewer_family"`
	SameFamilyReason string   `json:"same_family_reason"`
	Intent           Artifact `json:"intent"`
	Report           Artifact `json:"report"`
	Surface          Artifact `json:"surface"`
	Attack           Artifact `json:"attack"`
	Sync             Artifact `json:"sync"`
	Gaps             []string `json:"gaps"`
}
type Record struct {
	Version        int         `json:"version"`
	IntentSHA256   string      `json:"intent_sha256"`
	Coordinator    string      `json:"coordinator"`
	Author         string      `json:"author"`
	CIOwner        string      `json:"ci_owner"`
	Phase          string      `json:"phase"`
	Acceptance     []Criterion `json:"acceptance"`
	Candidate      Candidate   `json:"candidate"`
	Proofs         []Proof     `json:"proofs"`
	Review         *Review     `json:"review,omitempty"`
	Findings       []Finding   `json:"findings"`
	FailedRechecks int         `json:"failed_rechecks"`
	Next           string      `json:"next"`
}
type Stored struct {
	Previous int64  `json:"previous"`
	Record   Record `json:"record"`
}
type Pull struct {
	Head  string `json:"head"`
	Base  string `json:"base"`
	State string `json:"state"`
	URL   string `json:"url"`
}
type Check struct {
	Name   string `json:"name"`
	Bucket string `json:"bucket"`
	Link   string `json:"link"`
}

func decode(data []byte, value any, strict bool) error {
	if len(data) > 4<<20 {
		return errors.New("JSON record exceeds 4 MiB")
	}
	if strict {
		if err := uniqueFields(json.NewDecoder(bytes.NewReader(data)), 0); err != nil {
			return err
		}
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if strict {
		d.DisallowUnknownFields()
	}
	if err := d.Decode(value); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra json.RawMessage
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("expected exactly one JSON record")
	}
	return nil
}
func RequestFrom(data []byte) (Request, error) {
	var r Request
	if err := decode(data, &r, true); err != nil {
		return r, err
	}
	if !repoPattern.MatchString(r.Repo) || strings.HasSuffix(r.Repo, "/.") || strings.HasSuffix(r.Repo, "/..") || r.Issue < 1 || r.Previous < 0 {
		return r, errors.New("repo, positive issue and nonnegative previous are required")
	}
	switch r.Command {
	case "inspect":
		if r.Record != nil || r.Previous != 0 {
			return r, errors.New("inspect accepts only repo and issue")
		}
	case "checkpoint", "assess":
		if r.Record == nil {
			return r, errors.New("record is required")
		}
		if err := r.Record.Validate(); err != nil {
			return r, err
		}
	default:
		return r, errors.New("command must be inspect, checkpoint or assess")
	}
	return r, nil
}
func (r Record) Validate() error {
	if r.Version != 1 || !hashPattern.MatchString(r.IntentSHA256) || blank(r.Coordinator, r.Author, r.CIOwner, r.Next) || r.FailedRechecks < 0 {
		return errors.New("invalid version, intent digest, ownership, next action or re-check count")
	}
	switch r.Phase {
	case "active", "review", "blocked":
	default:
		return errors.New("phase must be active, review or blocked; only assess can establish acceptance")
	}
	if len(r.Acceptance) == 0 {
		return errors.New("acceptance requirements are missing")
	}
	seen := map[string]bool{}
	for _, c := range r.Acceptance {
		if blank(c.ID, c.Requirement) || seen[c.ID] {
			return errors.New("acceptance IDs must be nonempty and unique")
		}
		seen[c.ID] = true
	}
	if r.Candidate.Kind != "change" && r.Candidate.Kind != "no-change" {
		return errors.New("candidate kind must be change or no-change")
	}
	if !shaPattern.MatchString(r.Candidate.Head) || r.Candidate.PR < 0 || (r.Candidate.Kind == "change" && r.Candidate.PR == 0) {
		return errors.New("candidate needs a full head SHA and changes need a PR")
	}
	for _, f := range r.Findings {
		if blank(f.ID, f.Scope, f.Owner, f.Disposition, f.Evidence) {
			return errors.New("finding requires identity, scope, owner, disposition and evidence")
		}
		if f.Scope != "in" && f.Scope != "out" {
			return errors.New("finding scope must be in or out")
		}
	}
	for _, p := range r.Proofs {
		if !seen[p.Criterion] || !shaPattern.MatchString(p.Head) || blank(p.Build) {
			return errors.New("proof has unknown criterion, invalid head or missing build")
		}
		if p.Kind != "surface" {
			return errors.New("proof kind must be surface; Verilex integration is deferred")
		}
		if err := p.Evidence.Validate(); err != nil {
			return err
		}
		if err := p.Observation.Validate(); err != nil {
			return err
		}
	}
	if r.Review != nil {
		v := r.Review
		if !shaPattern.MatchString(v.Head) || blank(v.Identity, v.Verdict, v.Reviewer, v.Verifier, v.AuthorFamily, v.ReviewerFamily) {
			return errors.New("review identity, verdict, head and roles are required")
		}
		for _, a := range []Artifact{v.Intent, v.Report, v.Surface, v.Attack, v.Sync} {
			if err := a.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}
func (a Artifact) Validate() error {
	if !hashPattern.MatchString(a.SHA256) || !strings.HasPrefix(a.URL, "https://github.com/") {
		return errors.New("evidence needs SHA256 and a durable GitHub URL")
	}
	return nil
}
func blank(s ...string) bool {
	for _, v := range s {
		if strings.TrimSpace(v) == "" {
			return true
		}
	}
	return false
}
func IssueFrom(data []byte) (Issue, error) {
	var v Issue
	if err := decode(data, &v, true); err != nil {
		return v, err
	}
	if v.Number < 1 || blank(v.Title, v.Body, v.URL) {
		return v, errors.New("invalid issue response")
	}
	return v, nil
}
func CommentsFrom(data []byte) ([]Comment, error) {
	var v []Comment
	if err := decode(data, &v, true); err != nil {
		return nil, err
	}
	if v == nil {
		return nil, errors.New("comments response must be an array")
	}
	for _, c := range v {
		if c.ID < 1 || blank(c.URL) {
			return nil, errors.New("invalid comment response")
		}
	}
	return v, nil
}
func CommentFrom(data []byte) (Comment, error) {
	var v Comment
	if err := decode(data, &v, true); err != nil {
		return v, err
	}
	if v.ID < 1 || blank(v.URL) {
		return v, errors.New("invalid posted comment")
	}
	return v, nil
}
func StoredFrom(body string) (Stored, error) {
	var v Stored
	if !strings.HasPrefix(body, Marker) {
		return v, errors.New("not a Lights Out record")
	}
	if err := decode([]byte(strings.TrimPrefix(body, Marker)), &v, true); err != nil {
		return v, err
	}
	if v.Previous < 0 {
		return v, errors.New("negative predecessor")
	}
	return v, v.Record.Validate()
}
func PullFrom(data []byte) (Pull, error) {
	var v Pull
	if err := decode(data, &v, true); err != nil {
		return v, err
	}
	if !shaPattern.MatchString(v.Head) || blank(v.Base, v.State, v.URL) {
		return v, errors.New("invalid pull response")
	}
	return v, nil
}
func ChecksFrom(data []byte) ([]Check, error) {
	var v []Check
	if err := decode(data, &v, true); err != nil {
		return nil, err
	}
	if len(v) == 0 {
		return nil, errors.New("no required checks returned")
	}
	for _, c := range v {
		if blank(c.Name, c.Bucket, c.Link) {
			return nil, errors.New("invalid check response")
		}
	}
	return v, nil
}
