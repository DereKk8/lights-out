# Lights Out - Agent contributor guide

This file is for agents that build and maintain this repository.

## During the pilot build

This section and SPEC.md exist only until the pilot build ends.

- Read SPEC.md, the build's contract, before you propose or edit a change. Its sections 2 and 3 apply to work on this repository, and this file does not repeat them.
- SPEC.md section 5 shows which owner decisions are settled and which are open. Change SPEC.md only to record a decision the owner made.
- The build's last change follows SPEC.md section 6: it moves each lasting requirement to its durable owner, records the settled decisions, and deletes SPEC.md and this section.

## Authority

- The owner merges changes to this repository.
- Do not guess a decision that is open for the owner. Build the parts that do not depend on it, and ask the owner in an issue on this repository.

## How to work

1. Change the smallest coherent surface: no speculative infrastructure, duplicated rule, or incidental edit.
2. Work in small units that each end in a passing check. Test a capability through its public entry point, and assert the observed result.
3. Every change to this repository gets a zero-trust-review before it merges: through Lights Out's own loop once that loop exists, and directly until then.
4. Update the instructions, tests, and docs that your change makes wrong, in the same change.

## Go code

Go code in this repository keeps these guards. The first change that adds Go code adds them, and lists their commands under Commands:

- `golangci-lint` with a `version: "2"` config that sets the `errcheck` settings `check-blank: true` and `check-type-assertions: true`. With them, `_ = f()`, `_, _ = f()`, an unchecked call, and an unchecked type assertion all fail the lint. The default settings miss `_ = f()`.
- A `forbidigo` rule with `analyze-types: true` and the pattern `^json\.(Unmarshal|NewDecoder)$`, excluded only for the one package that decodes boundary records. No other code decodes JSON.
- In that package, each record is a named struct that is validated before it is returned. Records that Lights Out defines are decoded with `DisallowUnknownFields`. Tool output is requested as JSON with only the fields the conductor needs.
- A fixture that the lint must reject, with one violation per line, because golangci-lint reports only one issue per line by default. CI checks that the lint still rejects it, so a config change that weakens the guard fails.
- A required CI job that runs the lint and `go test ./...` on every change.

## Writing

For user facing documentation, write in plain English: short sentences, one idea each, active voice, and common words. Use "-", never an em dash.

## Maintaining this file

- Keep this file short. 
- Do not repeat what the codebase already shows; point to the authoritative file or command instead.
- Prefer rewriting or pruning existing entries over appending new ones.
