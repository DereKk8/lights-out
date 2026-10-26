# Lights Out - Agent contributor guide

This file is for agents that build and maintain this repository. A team that coordinates a task follows [ENTRY.md](ENTRY.md) instead.

## Authority

- Changes reach `main` in one of two ways. The owner, and agents working under the owner's account, commit a small patch straight to `main`: under 100 changed lines (added plus deleted, as ZT-STAFF-1 counts them) that touches no gate, authority boundary, or followed policy version. Every other change, and every contribution from anyone else, goes through a pull request that the owner merges.
- Do not guess a decision that is open for the owner. Build the parts that do not depend on it, and ask the owner in an issue on this repository.
- Agents maintain and improve the engineering mechanisms here on their own, when the benefit is observable and every existing guarantee holds. A change to product intent, an approved trade-off, a gate, or an authority boundary needs the owner's decision. The review and verification authorities stay external unless the owner decides otherwise.

## Thin conductor

Lights Out's own code is a thin conductor. It calls the tools that already own each job and passes typed records between them. Judgment stays with the coordinator and its agents.

The conductor never owns source hosting, branches, pull requests, or merge state; task tracking or notification; model execution or agent sessions; review policy; proof execution or evidence freshness; how a product is run and checked; or deployments. When a needed job has no owner, add the smallest dependable connection, not a replacement.

Reuse a working capability before you add one. Add, combine, or retire skills and integrations by their demonstrated value, with one clear owner each and no duplicated authority. Teams prove criteria themselves (`surface` proofs). An outside verification system joins only when the owner approves it, as one general proof kind that names the system, never as code for one system. None is approved today (D13).

The conductor follows one exact zero-trust-review version, `conductor.Followed` in `internal/conductor/conductor.go`. Change it only together with a sync-zero-trust run for the new version, and with the conductor changes that the new version needs.

## Go code

The conductor is written in Go: it fails loudly, takes and returns structured data, and is tested through its public entry point.

- **Loud failure.** A command that meets an error stops, exits nonzero, and writes a structured error that names the reason. It never drops an error or reports success after one.
- **Strong typing.** Every record that crosses a boundary (request, issue record, evidence, verdict, tool output) is a typed value, validated where it enters. Only `internal/boundary` decodes JSON. Each record there is a named struct that is validated before it is returned. Records that Lights Out defines are decoded with `DisallowUnknownFields`. Tool output is requested as JSON with only the fields the conductor needs.
- **No package import starts work.** Pass collaborators in at entry points, as `conductor.Run` takes its tool runner and policy identity.

These guards enforce the rules above. Keep them, and keep their commands under Commands:

- `golangci-lint` with a `version: "2"` config that sets the `errcheck` settings `check-blank: true` and `check-type-assertions: true`. With them, `_ = f()`, `_, _ = f()`, an unchecked call, and an unchecked type assertion all fail the lint. The default settings miss `_ = f()`.
- A `forbidigo` rule with `analyze-types: true` and the pattern `^json\.(Unmarshal|NewDecoder)$`, excluded only for `internal/boundary`.
- A fixture that the lint must reject, `testdata/lintfixture/violations.go`, with one violation per line, because golangci-lint reports only one issue per line by default. `lint-guard` checks that the lint still rejects each one, so a config change that weakens a guard fails.
- A CI job, `ci`, that runs the lint, the guard, the tests, and the build on every push and every pull request. The `main` ruleset requires it for pull requests, and the owner bypasses the ruleset for direct commits. When a push turns `ci` red, whoever pushed fixes it at once.

## Enforcement boundary

A requirement that can be checked is enforced by an executable check, not by a promise in a prompt. The conductor must reject unsupported acceptance (accepting a change without its required evidence), stale verdicts (a verdict for a revision other than the one being accepted), and unauthorized weakening of safeguards. The external authorities define their own proof and review contracts. Work completion, review, and integration must stay correct when agents or implementations change. Add only the smallest mechanism that makes a guarantee real.

[docs/commands.md](docs/commands.md#the-acceptance-gate) lists every gate. These tests prove them through `conductor.Run`, and `go test ./...` runs them all:

| Guarantee | Test |
|---|---|
| Acceptance needs exact-revision proof, a PASS receipt for the candidate, independent seats, the followed policy and a complete sync, resolved findings, accepted handoffs, and green required checks | `TestAssessRefusesUnsupportedAcceptance`, `TestAssessRecordsTheVerifiedOutcome`, `TestAssessVerifiesNoChangeOnTheDefaultBranch` |
| The intent, the acceptance contract, and the coordinator and author stay fixed, and the re-check count falls only with a new owner basis | `TestCheckpointRefusesUnauthorizedChanges`, `TestOwnerDecisionRecountsFailedRechecks` |
| A below-floor finding counts as resolved only with its own existing issue | `TestAssessAcceptsFiledBelowFloorFindings` |
| The history is append-only, unforked, canonical, and written only by repository writers | `TestInspectRefusesABrokenHistory`, `TestInspectSeparatesHistoryFromOtherComments`, `TestCheckpointProvesTheWriteLanded` |
| State survives the session that wrote it | `TestCheckpointPersistsStateForAFreshSession` |
| Evidence is pinned to its exact bytes, across every comment it spans | `TestArtifactPinsACommentForAssessment`, `TestLongEvidenceIsPinnedAcrossComments` |
| Nothing goes public before a pinned secret scan passes | `TestEvidenceIsScannedBeforeItGoesPublic`, `TestExecutableRefusesASecretBeforeItGoesPublic` |
| Malformed input fails before any tool runs | `TestEntryRejectsMalformedRequestsBeforeAnyToolRuns`, `TestExecutableRefusesAMalformedRequestBeforeAnyToolRuns` |
| A tool failure stops the command and names the call | `TestExecutableNamesTheFailingToolCall` |

## How to work

1. Change the smallest coherent surface: no speculative infrastructure, duplicated rule, or incidental edit.
2. Work in small units that each end in a passing check. Test a capability through its public entry point, and assert the observed result.
3. Every pull request gets a zero-trust-review before it merges: through Lights Out's own loop on an issue of this repository, or directly. A small direct commit skips it, and the commit log and `ci` are its checks. Keep each commit focused, with a message that says what changed and why, because agents audit the history through it.
4. Update the instructions, tests, and docs that your change makes wrong, in the same change.
5. This repository is public. Keep secrets, credentials, private hosts, and private context out of it.

## Commands

- Build: `go build -o /tmp/lights-out ./cmd/lights-out`
- Test the public entry point: `go test ./...` (needs `gitleaks` 8.30.1 on PATH)
- Lint: `golangci-lint run` (v2.14.0)
- Prove the lint rejects seeded violations: `go run ./cmd/lint-guard`
- Run: `go run ./cmd/lights-out < request.json` (see [docs/commands.md](docs/commands.md))
- CI: `.github/workflows/ci.yml`, job `ci`, runs the lint, the guard, the tests, and the build.

For a temporary Nix tool environment, prefix each command with `nix shell nixpkgs#golangci-lint nixpkgs#gitleaks -c`.

## Writing

For user facing documentation, write in plain English: short sentences, one idea each, active voice, and common words. Use "-", never an em dash.

## Maintaining this file

- Keep this file short.
- Do not repeat what the codebase already shows; point to the authoritative file or command instead.
- Prefer rewriting or pruning existing entries over appending new ones.
