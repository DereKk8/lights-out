package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/DereKk8/lights-out/internal/boundary"
)

// The policy location is fixed by sync-zero-trust.
const (
	policyRepo   = "DereKk8/project-skills"
	policyBranch = "master"
	policyPath   = "shared/zero-trust-review/SKILL.md"
)

const commentFields = "{id,body,url:.html_url,author:.user.login,created:.created_at,updated:.updated_at}"

// forge is the GitHub connection for one command on one issue. Every call
// goes through gh, and every response is decoded by the boundary package.
type forge struct {
	run     Runner
	ctx     context.Context
	repo    string
	issue   int
	writers map[string]bool
}

func (f *forge) gh(stdin []byte, args ...string) ([]byte, error) {
	return f.run(f.ctx, stdin, "gh", args...)
}

func (f *forge) api(endpoint, jq string) ([]byte, error) {
	return f.gh(nil, "api", endpoint, "--jq", jq)
}

func (f *forge) issuePath() string { return fmt.Sprintf("repos/%s/issues/%d", f.repo, f.issue) }

func (f *forge) issueURL() string {
	return fmt.Sprintf("https://github.com/%s/issues/%d", f.repo, f.issue)
}

func (f *forge) loadIssue() (boundary.Issue, error) {
	data, err := f.api(f.issuePath(), "{number,title,body,url:.html_url}")
	if err != nil {
		return boundary.Issue{}, err
	}
	issue, err := boundary.IssueFrom(data)
	if err != nil {
		return issue, err
	}
	if issue.Number != f.issue || issue.URL != f.issueURL() {
		return issue, fmt.Errorf("forge returned %s for %s: name the repository exactly as GitHub does", issue.URL, f.issueURL())
	}
	return issue, nil
}

func (f *forge) commentsPage(page int) ([]boundary.Comment, error) {
	data, err := f.api(fmt.Sprintf("%s/comments?per_page=100&page=%d", f.issuePath(), page), "[.[]|"+commentFields+"]")
	if err != nil {
		return nil, err
	}
	return boundary.CommentsFrom(data)
}

func (f *forge) comment(repo string, id int64) (boundary.Comment, error) {
	data, err := f.api(fmt.Sprintf("repos/%s/issues/comments/%d", repo, id), commentFields)
	if err != nil {
		return boundary.Comment{}, err
	}
	c, err := boundary.CommentFrom(data)
	if err != nil {
		return c, err
	}
	if c.ID != id {
		return c, fmt.Errorf("forge returned comment %d for %d", c.ID, id)
	}
	return c, nil
}

func (f *forge) post(body string) (boundary.Comment, error) {
	payload, err := json.Marshal(struct {
		Body string `json:"body"`
	}{body})
	if err != nil {
		return boundary.Comment{}, err
	}
	data, err := f.gh(payload, "api", f.issuePath()+"/comments", "--method", "POST", "--input", "-", "--jq", commentFields)
	if err != nil {
		return boundary.Comment{}, fmt.Errorf("the write may or may not have landed; inspect before you retry: %w", err)
	}
	return boundary.CommentFrom(data)
}

// writer reports whether login may write to repo, as GitHub decides it.
func (f *forge) writer(repo, login string) (bool, error) {
	key := repo + " " + login
	if ok, seen := f.writers[key]; seen {
		return ok, nil
	}
	data, err := f.api(fmt.Sprintf("repos/%s/collaborators/%s/permission", repo, url.PathEscape(login)), "{permission}")
	if err != nil {
		return false, err
	}
	ok, err := boundary.WriterFrom(data)
	if err != nil {
		return false, err
	}
	f.writers[key] = ok
	return ok, nil
}

// source fetches comment id and checks that it is on the source issue and
// that a repository writer wrote it.
func (f *forge) source(id int64) (boundary.Comment, error) {
	c, err := f.comment(f.repo, id)
	if err != nil {
		return c, err
	}
	if want := fmt.Sprintf("%s#issuecomment-%d", f.issueURL(), id); c.URL != want {
		return c, fmt.Errorf("comment %s is not on the source issue %s", c.URL, f.issueURL())
	}
	ok, err := f.writer(f.repo, c.Author)
	if err != nil {
		return c, err
	}
	if !ok {
		return c, fmt.Errorf("comment %s was written by %s, who cannot write to %s", c.URL, c.Author, f.repo)
	}
	return c, nil
}

// evidence fetches an artifact's comment again and checks that it is still
// the pinned bytes, on the source issue, from a repository writer.
func (f *forge) evidence(a boundary.Artifact) (boundary.Comment, error) {
	ref, err := boundary.ParseCommentURL(a.URL)
	if err != nil {
		return boundary.Comment{}, err
	}
	if ref.Repo != f.repo || ref.Issue != f.issue {
		return boundary.Comment{}, fmt.Errorf("evidence %s is not a comment on the source issue", a.URL)
	}
	c, err := f.source(ref.ID)
	if err != nil {
		return c, err
	}
	if boundary.BodyDigest(c.Body) != a.SHA256 {
		return c, fmt.Errorf("evidence %s changed since it was pinned, or its digest is wrong", a.URL)
	}
	return c, nil
}

func (f *forge) pull(pr int) (boundary.Pull, error) {
	data, err := f.gh(nil, "pr", "view", strconv.Itoa(pr), "--repo", f.repo, "--json", "headRefOid,state,url", "--jq", "{head:.headRefOid,state,url}")
	if err != nil {
		return boundary.Pull{}, err
	}
	return boundary.PullFrom(data)
}

func (f *forge) requiredChecks(pr int) ([]boundary.Check, error) {
	data, err := f.gh(nil, "pr", "checks", strconv.Itoa(pr), "--repo", f.repo, "--required", "--json", "name,bucket,link")
	if err != nil {
		return nil, err
	}
	return boundary.ChecksFrom(data)
}

// shipped reports whether the default branch contains head.
func (f *forge) shipped(head string) (bool, error) {
	data, err := f.api("repos/"+f.repo, "{default_branch}")
	if err != nil {
		return false, err
	}
	branch, err := boundary.DefaultBranchFrom(data)
	if err != nil {
		return false, err
	}
	data, err = f.api(fmt.Sprintf("repos/%s/compare/%s...%s", f.repo, head, url.PathEscape(branch)), "{status}")
	if err != nil {
		return false, err
	}
	return boundary.ContainedFrom(data)
}

// policy loads the current zero-trust-review the way sync-zero-trust step 1
// does: the newest commit that changed the file, then the file at that commit.
func (f *forge) policy() (boundary.Policy, error) {
	data, err := f.api(fmt.Sprintf("repos/%s/commits?sha=%s&path=%s&per_page=1", policyRepo, policyBranch, policyPath), "[.[]|{sha}]")
	if err != nil {
		return boundary.Policy{}, err
	}
	commits, err := boundary.CommitsFrom(data)
	if err != nil {
		return boundary.Policy{}, err
	}
	if len(commits) != 1 {
		return boundary.Policy{}, errors.New("found no commit of the zero-trust-review policy")
	}
	data, err = f.api(fmt.Sprintf("repos/%s/contents/%s?ref=%s", policyRepo, policyPath, commits[0]), "{sha,content}")
	if err != nil {
		return boundary.Policy{}, err
	}
	return boundary.PolicyFrom(commits[0], data)
}
