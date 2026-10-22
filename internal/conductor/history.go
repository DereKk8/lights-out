package conductor

import (
	"errors"
	"fmt"

	"github.com/DereKk8/lights-out/internal/boundary"
)

// Ignored is a record-like comment that is not part of the issue's history.
type Ignored struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// history is an issue and its record chain, as read from the forge.
type history struct {
	issue   boundary.Issue
	latest  int64
	url     string
	record  *boundary.Record
	ignored []Ignored
}

// load reads every comment page and rebuilds the record chain. A broken,
// edited or forked chain stops the command: it never picks a winner.
func (f *forge) load() (history, error) {
	issue, err := f.loadIssue()
	if err != nil {
		return history{}, err
	}
	h := history{issue: issue}
	for page := 1; page <= 100; page++ {
		comments, err := f.commentsPage(page)
		if err != nil {
			return history{}, err
		}
		for _, c := range comments {
			if err := h.add(f, c); err != nil {
				return history{}, fmt.Errorf("record comment %s: %w", c.URL, err)
			}
		}
		if len(comments) < 100 {
			return h, nil
		}
	}
	return history{}, errors.New("issue has more than 10000 comments; Lights Out does not recover a partial history")
}

func (h *history) add(f *forge, c boundary.Comment) error {
	if boundary.IsRetiredRecord(c.Body) {
		h.ignored = append(h.ignored, Ignored{c.URL, "an earlier record format; read it as history, not state"})
		return nil
	}
	if !boundary.IsRecord(c.Body) {
		return nil
	}
	ok, err := f.writer(f.repo, c.Author)
	if err != nil {
		return err
	}
	if !ok {
		h.ignored = append(h.ignored, Ignored{c.URL, c.Author + " cannot write to " + f.repo})
		return nil
	}
	if c.Updated != c.Created {
		return errors.New("it was edited after it was posted; records are append-only")
	}
	stored, err := boundary.StoredFrom(c.Body)
	if err != nil {
		return err
	}
	if stored.Previous != h.latest {
		return fmt.Errorf("it names predecessor %d, but the latest record is %d: reconcile the competing writers before you continue", stored.Previous, h.latest)
	}
	if h.record != nil {
		if err := boundary.Transition(*h.record, stored.Record); err != nil {
			return err
		}
	}
	h.latest, h.url, h.record = c.ID, c.URL, &stored.Record
	return nil
}

// append writes next after the latest record, then reads the chain again to
// prove that the write landed as the new latest record.
func (f *forge) append(h history, next boundary.Record) (history, error) {
	if h.record != nil {
		if err := boundary.Transition(*h.record, next); err != nil {
			return h, err
		}
	}
	body, err := boundary.Render(boundary.Stored{Previous: h.latest, Record: next})
	if err != nil {
		return h, err
	}
	posted, err := f.post(body)
	if err != nil {
		return h, err
	}
	after, err := f.load()
	if err != nil {
		return h, err
	}
	if after.latest != posted.ID {
		return h, fmt.Errorf("comment %d did not land as the latest record; inspect before you continue", posted.ID)
	}
	return after, nil
}
