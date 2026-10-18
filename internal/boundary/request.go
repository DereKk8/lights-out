package boundary

import (
	"errors"
	"strings"
)

// Request is the whole command surface. One JSON object arrives on stdin.
type Request struct {
	Command  string  `json:"command"`
	Repo     string  `json:"repo"`
	Issue    int     `json:"issue"`
	Previous int64   `json:"previous,omitempty"`
	Record   *Record `json:"record,omitempty"`
	Comment  int64   `json:"comment,omitempty"`
	Body     string  `json:"body,omitempty"`
}

func RequestFrom(data []byte) (Request, error) {
	var r Request
	if err := decode(data, &r, true); err != nil {
		return r, err
	}
	if !repoPattern.MatchString(r.Repo) || strings.HasSuffix(r.Repo, "/.") || strings.HasSuffix(r.Repo, "/..") {
		return r, errors.New("repo must be owner/name")
	}
	if r.Issue < 1 || r.Previous < 0 || r.Comment < 0 {
		return r, errors.New("issue must be positive; previous and comment cannot be negative")
	}
	if r.Body != "" && r.Command != "evidence" {
		return r, errors.New("only evidence accepts a body")
	}
	switch r.Command {
	case "inspect":
		if r.Record != nil || r.Previous != 0 || r.Comment != 0 {
			return r, errors.New("inspect accepts only repo and issue")
		}
	case "artifact":
		if r.Record != nil || r.Previous != 0 || r.Comment == 0 {
			return r, errors.New("artifact accepts repo, issue and comment")
		}
	case "checkpoint":
		if r.Record == nil || r.Comment != 0 {
			return r, errors.New("checkpoint accepts repo, issue, previous and record")
		}
		if r.Record.Phase == PhaseVerified || r.Record.Outcome != nil {
			return r, errors.New("a checkpoint cannot declare verification; only assess records an outcome")
		}
		if err := r.Record.Validate(); err != nil {
			return r, err
		}
	case "evidence":
		if r.Record != nil || r.Previous != 0 || r.Comment != 0 {
			return r, errors.New("evidence accepts repo, issue and body")
		}
		if err := ValidEvidence(r.Body); err != nil {
			return r, err
		}
	case "assess":
		if r.Record != nil || r.Comment != 0 {
			return r, errors.New("assess accepts repo, issue and previous; it assesses the stored record")
		}
	default:
		return r, errors.New("command must be inspect, artifact, evidence, checkpoint or assess")
	}
	return r, nil
}
