package lintfixture

import (
	"encoding/json"
	"os"
	"strings"
)

func Violations(value any) {
	os.Remove("missing")      // guard: errcheck
	_ = os.Remove("missing")  // guard: errcheck
	_, _ = os.Open("missing") // guard: errcheck
	_ = value.(string)        // guard: errcheck
	var target struct{ Name string }
	if err := json.Unmarshal([]byte(`{}`), &target); err != nil { // guard: forbidigo
		panic(err)
	}
	decoder := json.NewDecoder(strings.NewReader(`{}`)) // guard: forbidigo
	if err := decoder.Decode(&target); err != nil {
		panic(err)
	}
}
