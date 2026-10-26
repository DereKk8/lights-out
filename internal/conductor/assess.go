package conductor

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/DereKk8/lights-out/internal/boundary"
)

// assess is the acceptance gate. It refuses unless the stored review record
// proves the exact candidate under the followed policy, then appends the
// verified outcome. It never merges.
func assess(f *forge, previous int64, followed string) (Response, error) {
	h, err := current(f, previous)
	if err != nil {
		return Response{}, err
	}
	if h.record == nil {
		return Response{}, errors.New("the issue has no record to assess")
	}
	r := *h.record
	if err := ready(r); err != nil {
		return Response{}, err
	}
	order, err := reviewed(f, r, followed)
	if err != nil {
		return Response{}, err
	}
	if err := proven(f, r, order); err != nil {
		return Response{}, err
	}
	if err := resolved(f, r); err != nil {
		return Response{}, err
	}
	outcome, err := integrated(f, *r.Candidate)
	if err != nil {
		return Response{}, err
	}
	latest, err := current(f, h.latest)
	if err != nil {
		return Response{}, fmt.Errorf("the issue changed during assessment: %w", err)
	}
	r.Phase, r.Outcome = boundary.PhaseVerified, &outcome
	r.Next = "Verified. Integrate head " + outcome.Head + " through the forge's revision-bound merge, under the merge authority in the entry point."
	if r.Candidate.Kind == "no-change" {
		r.Next = "Verified no-change outcome at " + outcome.Head + ". Report it on the issue; nothing needs integration."
	}
	latest, err = f.append(latest, r)
	if err != nil {
		return Response{}, err
	}
	return recorded("verified", latest)
}

// ready holds the checks that need no forge call.
func ready(r boundary.Record) error {
	if r.Phase != boundary.PhaseReview {
		return fmt.Errorf("the latest record is in phase %s; assess needs phase review", r.Phase)
	}
	if r.FailedRechecks >= 2 {
		return errors.New("two re-checks failed: the next step is the owner's re-scope decision (ZT-LOOP-6), not another assessment")
	}
	if r.Review == nil {
		return errors.New("the record has no independent review")
	}
	covered := map[string]bool{}
	for _, p := range r.Proofs {
		if p.Head != r.Candidate.Head {
			return fmt.Errorf("the proof of %s is for %s, not the candidate %s: stale evidence", p.Criterion, p.Head, r.Candidate.Head)
		}
		covered[p.Criterion] = true
	}
	for _, c := range r.Acceptance {
		if !covered[c.ID] {
			return fmt.Errorf("acceptance %s has no proof", c.ID)
		}
	}
	for _, f := range r.Findings {
		if f.Disposition == "open" {
			return fmt.Errorf("finding %s is open: fix or disprove an in-scope finding, and hand off an out-of-scope one", f.ID)
		}
	}
	return nil
}

// window is the span of comment IDs in which proof had to be posted: after
// the saved review and before the receipt that endorses it.
type window struct{ after, before int64 }

// reviewed checks the review record against the followed policy: its parts in
// the required order, its sync map and the verifier's receipt.
func reviewed(f *forge, r boundary.Record, followed string) (window, error) {
	policy, err := f.policy()
	if err != nil {
		return window{}, err
	}
	if policy.Identity != followed {
		return window{}, fmt.Errorf("zero-trust-review is now %q, but this Lights Out follows %q: run sync-zero-trust and update Lights Out first", policy.Identity, followed)
	}
	v := r.Review
	var parts []span
	for _, a := range []boundary.Artifact{v.Sync, v.Intent, v.Suspicions, v.Surface, v.Attack, v.Report, v.Receipt} {
		c, err := f.evidence(a)
		if err != nil {
			return window{}, err
		}
		parts = append(parts, c)
	}
	syncPart, intent, suspicions, surface, attack, report, receipt := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], parts[6]
	if syncPart.last >= intent.first || intent.last >= suspicions.first {
		return window{}, errors.New("the sync, the saved intent reading and the saved review must be posted in that order (ZT-INDEP-1)")
	}
	if suspicions.last >= min(surface.first, attack.first) || max(surface.last, attack.last) >= report.first || report.last >= receipt.first {
		return window{}, errors.New("proof must follow the saved review, and the report and receipt must follow proof (ZT-INDEP-7)")
	}
	sync, err := boundary.SyncFrom(syncPart.body)
	if err != nil {
		return window{}, err
	}
	if sync.Identity != followed {
		return window{}, fmt.Errorf("the review synced to %q, not %q", sync.Identity, followed)
	}
	if !slices.Equal(sync.IDs, policy.IDs) {
		return window{}, errors.New("the sync map must give each requirement ID of the policy exactly once, in file order")
	}
	if len(sync.Blocking) > 0 || sync.Incomplete != "none" {
		return window{}, fmt.Errorf("the sync reports blocking gaps [%s] and incomplete parts %q: those parts are INCOMPLETE", strings.Join(sync.Blocking, " "), sync.Incomplete)
	}
	return window{suspicions.last, receipt.first}, endorsed(r, receipt.body, followed)
}

// endorsed checks that the verifier's receipt binds a PASS to this candidate,
// contract, proof set and report, from seats independent of the author.
func endorsed(r boundary.Record, body, followed string) error {
	receipt, err := boundary.ReceiptFrom(body)
	if err != nil {
		return err
	}
	contract, err := boundary.Digest(r.Acceptance)
	if err != nil {
		return err
	}
	proofs, err := boundary.Digest(r.Proofs)
	if err != nil {
		return err
	}
	switch {
	case receipt.Policy != followed:
		return fmt.Errorf("the receipt follows %q, not %q", receipt.Policy, followed)
	case receipt.Head != r.Candidate.Head:
		return fmt.Errorf("the receipt is for %s, not the candidate %s: stale verdict", receipt.Head, r.Candidate.Head)
	case receipt.Verdict != "PASS":
		return fmt.Errorf("the review verdict is %s, not PASS", receipt.Verdict)
	case receipt.ContractSHA256 != contract:
		return errors.New("the receipt does not endorse this acceptance contract")
	case receipt.ProofsSHA256 != proofs:
		return errors.New("the receipt does not endorse these proofs")
	case receipt.ReportSHA256 != r.Review.Report.SHA256:
		return errors.New("the receipt does not endorse this report")
	case receipt.Author != r.Author:
		return fmt.Errorf("the receipt names author %s, but the record names %s", receipt.Author, r.Author)
	case receipt.Reviewer == r.Author || receipt.Verifier == r.Author:
		return errors.New("the author cannot hold the reviewer or verifier seat")
	case strings.EqualFold(strings.TrimSpace(receipt.AuthorFamily), strings.TrimSpace(receipt.ReviewerFamily)):
		return errors.New("the reviewer must be a different model family from the author (ZT-MODEL-1)")
	}
	return nil
}

// proven fetches every proof again and checks that it was posted inside the
// proof window.
func proven(f *forge, r boundary.Record, w window) error {
	for _, p := range r.Proofs {
		for _, a := range []boundary.Artifact{p.Evidence, p.Observation} {
			c, err := f.evidence(a)
			if err != nil {
				return err
			}
			if c.first <= w.after || c.last >= w.before {
				return fmt.Errorf("proof %s of %s was not posted between the saved review and the receipt", a.URL, p.Criterion)
			}
		}
	}
	return nil
}

// resolved checks each finding's evidence and each handoff's acceptance.
func resolved(f *forge, r boundary.Record) error {
	for _, finding := range r.Findings {
		if _, err := f.evidence(finding.Evidence); err != nil {
			return fmt.Errorf("finding %s: %w", finding.ID, err)
		}
		if finding.Scope == "out" {
			if err := accepted(f, r, finding); err != nil {
				return fmt.Errorf("finding %s: %w", finding.ID, err)
			}
		}
		if finding.Disposition == "filed" {
			if err := filed(f, finding); err != nil {
				return fmt.Errorf("finding %s: %w", finding.ID, err)
			}
		}
	}
	if r.RecheckBasis != nil {
		if _, err := f.evidence(*r.RecheckBasis); err != nil {
			return fmt.Errorf("recheck_basis: %w", err)
		}
	}
	return nil
}

// filed checks that a finding below the severity floor (ZT-SEV-1) has its
// own issue in the project's tracker, separate from the source issue.
func filed(f *forge, finding boundary.Finding) error {
	ref, err := boundary.ParseIssueURL(finding.Filed)
	if err != nil {
		return err
	}
	if ref.Repo != f.repo || ref.Issue == f.issue {
		return errors.New("a filed finding needs its own issue in the source repository")
	}
	_, err = f.issueAt(ref.Repo, ref.Issue)
	return err
}

// accepted checks that a separate accountable owner took the finding: its
// own Lights Out record on another issue, under the owner the finding names.
func accepted(f *forge, r boundary.Record, finding boundary.Finding) error {
	ref, err := boundary.ParseCommentURL(finding.Accepted)
	if err != nil {
		return err
	}
	if ref.Repo == f.repo && ref.Issue == f.issue {
		return errors.New("a handoff must be accepted on another issue")
	}
	c, err := f.comment(ref.Repo, ref.ID)
	if err != nil {
		return err
	}
	ok, err := f.writer(ref.Repo, c.Author)
	if err != nil {
		return err
	}
	if c.URL != finding.Accepted || !ok || c.Updated != c.Created {
		return errors.New("the acceptance must be an unedited record comment from a writer of its repository")
	}
	stored, err := boundary.StoredFrom(c.Body)
	if err != nil {
		return fmt.Errorf("the acceptance is not a Lights Out record: %w", err)
	}
	if stored.Record.Coordinator == r.Coordinator || stored.Record.Coordinator != finding.Owner {
		return fmt.Errorf("the acceptance is by %s; it must be by the finding's owner %s, separate from this team", stored.Record.Coordinator, finding.Owner)
	}
	return nil
}

// integrated checks the candidate on the forge: an open pull request whose
// head stays the candidate while every required check passes, or for a
// no-change outcome a revision that the default branch contains.
func integrated(f *forge, c boundary.Candidate) (boundary.Outcome, error) {
	if c.Kind == "no-change" {
		ok, err := f.shipped(c.Head)
		if err != nil {
			return boundary.Outcome{}, err
		}
		if !ok {
			return boundary.Outcome{}, fmt.Errorf("the no-change candidate %s is not on the default branch", c.Head)
		}
		return boundary.Outcome{Head: c.Head, Checks: []boundary.Check{}}, nil
	}
	before, err := f.pull(c.PR)
	if err != nil {
		return boundary.Outcome{}, err
	}
	if before.State != "OPEN" {
		return boundary.Outcome{}, fmt.Errorf("pull request %d is %s, not open", c.PR, before.State)
	}
	if before.Head != c.Head {
		return boundary.Outcome{}, fmt.Errorf("pull request %d is at %s, not the candidate %s: the candidate is stale", c.PR, before.Head, c.Head)
	}
	checks, err := f.requiredChecks(c.PR)
	if err != nil {
		return boundary.Outcome{}, err
	}
	for _, check := range checks {
		if check.Bucket != "pass" {
			return boundary.Outcome{}, fmt.Errorf("required check %s is %s", check.Name, check.Bucket)
		}
	}
	after, err := f.pull(c.PR)
	if err != nil {
		return boundary.Outcome{}, err
	}
	if after != before {
		return boundary.Outcome{}, fmt.Errorf("pull request %d changed during assessment", c.PR)
	}
	return boundary.Outcome{Head: c.Head, Checks: checks}, nil
}
