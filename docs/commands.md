# Command contract

Build with `go build -o /tmp/lights-out ./cmd/lights-out`. The executable reads one JSON object from stdin. Success writes JSON to stdout. Errors write JSON to stderr and exit nonzero. Unknown fields and malformed records fail. Requests are limited to 4 MiB. All GitHub calls use `gh` with explicit JSON fields because `gh-axi` presents prose/TOON. Use the machine's approved credential wrapper before invoking the executable. No secret is an argument.

## Inspect

```json
{"command":"inspect","repo":"DereKk8/verilex","issue":55}
```

The response contains the original `issue`, its `intent_sha256`, the latest record comment ID in `latest` (zero initially), and `record` when present. It also returns canonical `contract_sha256` and `proofs_sha256` when a record exists. The source issue and comment history, not local files, hold progress. The command reads every page or fails. A broken record or a fork in the predecessor chain stops recovery.

## Checkpoint

Send `command: "checkpoint"`, `repo`, `issue`, `previous` equal to the last `latest`, and `record`. The initial record has this shape:

```json
{
  "version":1,
  "intent_sha256":"<inspect result>",
  "coordinator":"<accountable team identity>",
  "author":"<author session identity>",
  "ci_owner":"<named CI-health owner>",
  "phase":"active",
  "acceptance":[{"id":"A1","requirement":"<source requirement>"}],
  "candidate":{"kind":"change","head":"<40-character SHA>","pr":56},
  "proofs":[],
  "findings":[],
  "failed_rechecks":0,
  "next":"<specific next action>"
}
```

A changed candidate needs a PR. Create a draft through the existing forge before the first checkpoint. A no-change candidate uses `kind: "no-change"`, the existing revision, and `pr: 0`. Phases are `active`, `review`, and `blocked`. A checkpoint cannot declare verification. The intent digest, acceptance requirements, coordinator and author cannot silently change. A fresh session continues the same accountable team identity. The failed re-check count cannot decrease. Product changes to a frozen contract require a new owner-scoped task, not an overwrite.

Each finding contains `id`, `scope` (`in` or `out`), `owner`, `disposition`, `evidence`, and `accepted`. In-scope findings must be fixed or disproved before acceptance. An out-of-scope finding needs a receiving owner's acceptance reference. The coordinator records trigger, harm, and the smallest disposition in the linked evidence.

The command appends a typed comment with a predecessor ID, then reads the history again. A concurrent fork fails loudly, including on subsequent recovery. It never silently picks a winner. A network failure after posting is uncertain: inspect before retrying. Do not blindly retry a write.

## Evidence and review

Post evidence as ordinary comments on the source issue. Each artifact is:

```json
{"sha256":"<SHA256 of the exact comment body bytes>","url":"https://github.com/DereKk8/verilex/issues/55#issuecomment-123"}
```

Assessment fetches the comment again and checks the digest. Preserve raw commands, inputs, expected and observed values, exits, and independent side effects in these comments. Do not put credentials or private context there.

Each `proofs` entry has `criterion` (an acceptance ID), `head`, `kind: "surface"`, `build` (the tested artifact identity), `evidence` (an artifact), and `observation` (an independently observed artifact). Checkpoint the proofs before the final review handoff to obtain their canonical digest.

The independent verifier posts a JSON receipt as its own issue comment, not inside a Markdown fence:

```json
{
  "policy":"Identity: zero-trust-review 3.2.0 0c71c96e973420f96912aa61fbbf313cfb5c85db b83638f62ab25178cbf140eebf9a72db00caf562",
  "head":"<tested revision>",
  "verdict":"PASS",
  "contract_sha256":"<checkpoint result>",
  "proofs_sha256":"<checkpoint result>",
  "author":"<record author identity>",
  "reviewer":"<independent session identity>",
  "verifier":"<proof session identity>",
  "gaps":[],
  "report":"<complete zero-trust-review report>"
}
```

The `review` field of the next checkpoint has `identity` (the same policy identity), `head`, `verdict`, `reviewer`, `verifier`, `author_family`, `reviewer_family`, `same_family_reason`, `gaps`, and five artifacts: `intent`, `report` (the receipt), `surface`, `attack`, and `sync`. The sync artifact contains the external policy's full per-ID mechanism map. Only the independent verifier issues the receipt. Do not create one from the author's summary.

This is a connection to human/agent review, not a second review policy or a cryptographic identity service. GitHub credentials authorize comment writes. The external runtime must isolate review sessions and enforce who may issue the receipt. A user with those credentials can forge comments and identities. Lights Out does not claim to defend against that user. Digest checks prevent accidental edits and stale reuse, not credential misuse.

## Assess

Send `command: "assess"` with the exact latest record and predecessor. Assessment rejects missing proof, stale revisions, wrong receipts, changed evidence, unaccepted handoffs, blocking sync gaps, and failed required checks. It reads the PR head before and after the required-check query. It returns `status: "verified"` only on success.

For no-change outcomes, the reviewed revision must exist in the target repository. For changes, all forge-required checks must pass. The pilot has no merge command and always returns `merge_authorized: false`. The owner must arrange the separate full external review before integration. A verified candidate is not a merged or delivered change.

Publish the assessment response and final outcome on the issue. This preserves the result for the next session. The external review authority still owns semantic correctness, staffing, saved independent interpretation, product proof, attacks, and its bounded fix loop. Missing implementation of one of its requirements is a sync gap, never an exception.
