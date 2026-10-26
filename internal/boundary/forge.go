package boundary

import (
	"errors"
	"fmt"
)

// The records below are GitHub output that the conductor shapes with jq to
// exactly these fields.

type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}

func IssueFrom(data []byte) (Issue, error) {
	var v Issue
	if err := decode(data, &v, true); err != nil {
		return v, fmt.Errorf("issue: %w", err)
	}
	if v.Number < 1 || blank(v.Title, v.URL) {
		return v, errors.New("forge returned an invalid issue")
	}
	if blank(v.Body) {
		return v, errors.New("the issue has no body: state the intent in the issue before a team takes it")
	}
	return v, nil
}

// Comment is an issue comment with the facts that decide whether to trust it.
type Comment struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	URL     string `json:"url"`
	Author  string `json:"author"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

func (c Comment) validate() error {
	if c.ID < 1 || blank(c.URL, c.Author, c.Created, c.Updated) {
		return errors.New("forge returned an invalid comment")
	}
	return nil
}

func CommentsFrom(data []byte) ([]Comment, error) {
	var v []Comment
	if err := decode(data, &v, true); err != nil {
		return nil, fmt.Errorf("comments: %w", err)
	}
	if v == nil {
		return nil, errors.New("forge returned no comment array")
	}
	for _, c := range v {
		if err := c.validate(); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func CommentFrom(data []byte) (Comment, error) {
	var v Comment
	if err := decode(data, &v, true); err != nil {
		return v, fmt.Errorf("comment: %w", err)
	}
	return v, v.validate()
}

type permission struct {
	Permission string `json:"permission"`
}

// WriterFrom reads a repository permission. Only people who can write to the
// repository may write its records and evidence.
func WriterFrom(data []byte) (bool, error) {
	var v permission
	if err := decode(data, &v, true); err != nil {
		return false, fmt.Errorf("permission: %w", err)
	}
	switch v.Permission {
	case "admin", "maintain", "write":
		return true, nil
	case "triage", "read", "none":
		return false, nil
	}
	return false, fmt.Errorf("forge returned unknown permission %q", v.Permission)
}

type Pull struct {
	Head  string `json:"head"`
	State string `json:"state"`
	URL   string `json:"url"`
}

func PullFrom(data []byte) (Pull, error) {
	var v Pull
	if err := decode(data, &v, true); err != nil {
		return v, fmt.Errorf("pull request: %w", err)
	}
	if !shaPattern.MatchString(v.Head) || blank(v.State, v.URL) {
		return v, errors.New("forge returned an invalid pull request")
	}
	return v, nil
}

func ChecksFrom(data []byte) ([]Check, error) {
	var v []Check
	if err := decode(data, &v, true); err != nil {
		return nil, fmt.Errorf("checks: %w", err)
	}
	if len(v) == 0 {
		return nil, errors.New("the forge requires no checks for this pull request, so no check can show it green")
	}
	for _, c := range v {
		switch c.Bucket {
		case "pass", "fail", "pending", "skipping", "cancel":
		default:
			return nil, fmt.Errorf("forge returned check %q with unknown state %q", c.Name, c.Bucket)
		}
		if blank(c.Name) {
			return nil, errors.New("forge returned a check without a name")
		}
	}
	return v, nil
}

type commit struct {
	SHA string `json:"sha"`
}

// CommitsFrom reads a commit list shaped as [{sha}] and returns the SHAs.
func CommitsFrom(data []byte) ([]string, error) {
	var v []commit
	if err := decode(data, &v, true); err != nil {
		return nil, fmt.Errorf("commits: %w", err)
	}
	var shas []string
	for _, c := range v {
		if !shaPattern.MatchString(c.SHA) {
			return nil, errors.New("forge returned an invalid commit SHA")
		}
		shas = append(shas, c.SHA)
	}
	return shas, nil
}

type comparison struct {
	Status string `json:"status"`
}

// ContainedFrom reads a comparison of the candidate (base) with the default
// branch (head). The branch contains the candidate when it is identical or ahead.
func ContainedFrom(data []byte) (bool, error) {
	var v comparison
	if err := decode(data, &v, true); err != nil {
		return false, fmt.Errorf("comparison: %w", err)
	}
	switch v.Status {
	case "identical", "ahead":
		return true, nil
	case "behind", "diverged":
		return false, nil
	}
	return false, fmt.Errorf("forge returned unknown comparison status %q", v.Status)
}

type repository struct {
	DefaultBranch string `json:"default_branch"`
}

func DefaultBranchFrom(data []byte) (string, error) {
	var v repository
	if err := decode(data, &v, true); err != nil {
		return "", fmt.Errorf("repository: %w", err)
	}
	if blank(v.DefaultBranch) {
		return "", errors.New("forge returned no default branch")
	}
	return v.DefaultBranch, nil
}
