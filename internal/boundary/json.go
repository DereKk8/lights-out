package boundary

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Go otherwise accepts duplicate and case-insensitive struct keys. Both can
// make a signed-off record mean different things to different consumers.
func uniqueFields(d *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON nesting exceeds 64 levels")
	}
	token, err := d.Token()
	if err != nil {
		return fmt.Errorf("decode JSON token: %w", err)
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
				return err
			}
			key, ok := token.(string)
			if !ok {
				return errors.New("JSON object key is not a string")
			}
			if key != strings.ToLower(key) || seen[key] {
				return errors.New("duplicate or noncanonical JSON field")
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
		return err
	}
	return nil
}
