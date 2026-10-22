// Package boundary owns the decoding and validation of every record that
// enters Lights Out: requests, issue records, review records and tool output.
// No other package decodes JSON.
package boundary

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// MaxRecord bounds every request, record and tool response.
const MaxRecord = 4 << 20

var (
	repoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+$`)
	shaPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// decode reads exactly one JSON value into v. Strict decoding, for records
// that Lights Out defines or shapes with jq, also rejects unknown fields and
// duplicate or non-lowercase keys.
func decode(data []byte, v any, strict bool) error {
	if len(data) > MaxRecord {
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
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra json.RawMessage
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("expected exactly one JSON record")
	}
	return nil
}

// Go otherwise accepts duplicate and case-insensitive struct keys. Both can
// make one record mean different things to different readers.
func uniqueFields(d *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON nesting exceeds 64 levels")
	}
	token, err := d.Token()
	if err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			token, err = d.Token()
			if err != nil {
				return fmt.Errorf("decode JSON: %w", err)
			}
			key, ok := token.(string)
			if !ok {
				return errors.New("JSON object key is not a string")
			}
			if key != strings.ToLower(key) || seen[key] {
				return fmt.Errorf("duplicate or non-lowercase JSON field %q", key)
			}
			seen[key] = true
			if err = uniqueFields(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err = uniqueFields(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	if _, err = d.Token(); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

// Digest is the SHA-256 of Go's compact JSON encoding of v. Field order
// follows the typed record, so callers copy digests from conductor output
// instead of computing them with other JSON tools.
func Digest(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return BodyDigest(string(data)), nil
}

// BodyDigest is the SHA-256 of the exact bytes of a comment body.
func BodyDigest(body string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
}

func blank(s ...string) bool {
	for _, v := range s {
		if strings.TrimSpace(v) == "" {
			return true
		}
	}
	return false
}
