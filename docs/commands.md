# Command contract

The conductor is one executable. It reads one JSON request on stdin. On success it writes one JSON result to stdout and exits 0. On any error it writes `{"status":"error","kind":"<kind>","error":"<reason>"}` to stderr and exits 1. Kind `refused` means Lights Out checked and refused. Kind `environment` means a tool call failed, so nothing was proved either way: never read it as a product failure or as a pass. The conductor never reports success after an error.

Run it from a checkout of this repository:

```sh
printf '%s' '{"command":"inspect","repo":"OWNER/NAME","issue":55}' | go -C <lights-out checkout> run ./cmd/lights-out
```

The conductor calls `gh`, which must be signed in as a user who can write to the repository. No secret is ever an argument. Requests are limited to 4 MiB. Unknown, duplicate, or non-lowercase fields fail. Name the repository exactly as GitHub does, including case.

## Commands

| Command | Request fields | What it does |
|---|---|---|
| `inspect` | `repo`, `issue` | Reads the issue and its whole record chain. Returns `policy` (the zero-trust-review identity line that Lights Out follows), `issue`, `intent_sha256`, `latest` (the newest record comment ID, 0 when none), `url`, `record`, `contract_sha256`, `proofs_sha256`, and `ignored`. |
| `artifact` | `repo`, `issue`, `comment` | Pins one comment on the source issue. Returns `artifact`: `{"sha256":"…","url":"…"}`. Use it for every artifact; do not compute digests yourself. |
| `checkpoint` | `repo`, `issue`, `previous`, `record` | Appends `record` after the record `previous` (the `latest` you last saw), then reads the chain again to prove the write landed. Returns the new `latest`, `url`, `record`, and digests. |
| `assess` | `repo`, `issue`, `previous` | Runs the acceptance gate on the stored record `previous`. On success it appends the verified record and returns `status: "verified"`. On refusal it writes nothing. |

`ignored` lists record-like comments that are not part of the history: records in an earlier format, and records written by someone who cannot write to the repository. Read the earlier-format records as history, not state.

A network failure during a write is uncertain. Run `inspect` before you retry, and never retry a write blindly.

## The record

Each record is one issue comment. It starts with `<!-- lights-out:v2 -->`, then one summary line for people, then the typed record folded below it. The conductor writes this form, and it reads only this exact form back. A record comment that someone edited, or that does not match the form, stops recovery. A record that names the wrong predecessor is a fork, and it stops recovery: the conductor never picks a winner.

```json
{
  "intent_sha256": "<from inspect>",
  "coordinator": "<accountable team identity, never changes>",
  "session": "<the coordinator session that writes this checkpoint>",
  "author": "<author seat identity, never changes; the author and its fix sessions use it>",
  "ci_owner": "<the one owner of CI-runner health>",
  "phase": "active",
  "acceptance": [{"id": "A1", "requirement": "<from the source issue>"}],
  "proofs": [],
  "findings": [],
  "failed_rechecks": 0,
  "next": "<the specific next action>"
}
```

- `phase` is `active`, `review`, or `blocked`. Use `blocked` while a decision waits for the owner. Only `assess` writes `verified`.
- `candidate` is absent until a candidate exists. Then it is `{"kind":"change","head":"<40-hex>","pr":<number>}`, or `{"kind":"no-change","head":"<40-hex>","pr":0}` for an existing revision. Phase `review` needs a candidate.
- These never change once recorded: `intent_sha256`, `acceptance`, `coordinator`, `author`. A changed issue or contract needs the owner's decision.
- `failed_rechecks` never decreases.

### Proofs

Each entry proves one acceptance criterion on the exact candidate:

```json
{"criterion": "A1", "head": "<candidate head>", "kind": "surface", "build": "<identity of the tested artifact>", "evidence": <artifact>, "observation": <artifact>}
```

`evidence` holds the action and its result. `observation` is a separate comment with a second, independent observation of the result. `kind` is `surface`. The owner deferred Verilex (D13), so there is no Verilex proof kind.

### Findings

```json
{"id": "F1", "scope": "in", "owner": "<team>", "disposition": "open", "evidence": <artifact>}
```

`evidence` records the trigger, the harm, and the smallest disposition. An in-scope finding is `open`, `fixed`, or `disproved`. An out-of-scope finding is `open` until a separate team accepts it. Then it is `handed-off`, `owner` names that team's coordinator, and `accepted` is the URL of that team's own Lights Out record comment on another issue. A filed issue that nobody took is not a handoff.

### Review

`review` pins one zero-trust-review run as seven artifacts, all comments on the source issue:

| Field | Holds | Must be posted |
|---|---|---|
| `sync` | the sync-zero-trust reply | first |
| `intent` | the reviewer's saved reading of the intent | after `sync` |
| `suspicions` | the saved review: suspicions, coverage, uncertainty | after `intent` |
| `surface` | the product-surface run | after `suspicions` |
| `attack` | the attack phase | after `suspicions` |
| `report` | the complete report | after `surface`, `attack`, and every proof |
| `receipt` | the verifier's receipt | last |

Every proof artifact must be posted after `suspicions` and before `receipt`.

The receipt is one JSON object in a json code fence. Copy `contract_sha256` and `proofs_sha256` from the checkpoint that recorded the proofs. `report_sha256` is the `sha256` of the report artifact.

```json
{
  "policy": "<the identity line Lights Out follows>",
  "head": "<the tested head>",
  "verdict": "PASS",
  "contract_sha256": "<from checkpoint>",
  "proofs_sha256": "<from checkpoint>",
  "report_sha256": "<from the report artifact>",
  "author": "<the record's author>",
  "author_family": "<model family of the author seat>",
  "reviewer": "<reviewer seat identity>",
  "reviewer_family": "<model family of the reviewer>",
  "verifier": "<verifier seat identity>"
}
```

## The acceptance gate

`assess` returns `verified` only when all of these hold. Each failure names its reason.

1. The caller saw the latest record, the source issue is unchanged, and the record is in phase `review`.
2. Fewer than two re-checks failed (ZT-LOOP-6).
3. Every acceptance criterion has a proof, and every proof is for the candidate head.
4. No finding is `open`. Each handoff links a record of the named owner, on another issue, by a different coordinator.
5. The zero-trust-review identity on the policy's master branch is the one Lights Out follows. The conductor loads the file as sync-zero-trust step 1 does and checks its bytes against the blob ID.
6. Every artifact is still its pinned bytes, is on the source issue, and was written by a repository writer.
7. The review parts were posted in the order above.
8. The sync reply names the followed identity, maps every requirement ID of that version in file order, and has no blocking gap and no incomplete part.
9. The receipt follows the same identity, says PASS for the candidate head, and endorses this contract, these proofs, and this report. The reviewer and verifier are not the author, and the reviewer's model family differs from the author's.
10. For a change: the pull request is open, its head is the candidate, every required check passes, and the head does not move while checks are read. For no-change: the default branch contains the candidate.
11. Nothing moved during assessment.

The verified record repeats the assessed record with phase `verified` and an `outcome`: the head and the required checks that passed. `assess` never merges.

## What the conductor does not defend against

GitHub decides who may write. Anyone who can write to the repository can also write records and evidence under any role name. Lights Out ignores records from people who cannot write. It refuses evidence from them. It does not prove which model or session wrote a comment. The runtime that starts the sessions owns their isolation. The digests catch edits and stale reuse. They do not catch misuse of a writer's credentials.
