# Coordinate a task

Give an agent the issue URL and this file. That agent is the accountable coordinator, not a dispatcher that leaves after assigning work. Read the source issue and the target repository's instructions before acting. GitHub is the pilot's only integration. Discord is next (D8), not implemented.

## Recover and own

1. Build `go build -o /tmp/lights-out ./cmd/lights-out` from this repository. Commands take one JSON request on stdin and return JSON. Read [the command contract](docs/commands.md).
2. Run `inspect` with the repository and issue number. The source issue defines intent. Its append-only Lights Out comments hold state, evidence, findings, failed re-check count, and next action. Recover those records before doing work. A fresh session uses the same issue and this entry point, not a predecessor's memory.
3. Name one coordinator and one CI-health owner in the record. Check concurrent issues and PRs for overlap. Keep scopes and owners separate. Resolve conflicts before delivery. Use the existing runtime to delegate bounded work when useful, with cheaper models only where adequate. Do not start standing workers or a fleet supervisor.
4. Record the acceptance requirements before implementation. Preserve the source intent and the acceptance requirements through review. A changed requirement needs the owner's decision, not a convenient new test.

## Execute and preserve scope

Reproduce defects through the affected product entry point before fixing them. Work in small, maintainable changes with reproducible checks. Engineering difficulty belongs to the coordinator: change strategy or seek expertise rather than asking the owner to solve it. The review authority's fix-round limits still apply.

Confirmed findings need a real trigger, harm, and the smallest proportionate disposition under the external review policy. Fix in-scope defects. Start a separate accountable owner for unrelated defects immediately. Keep responsibility until that owner accepts the handoff in the issue record. A filed issue alone is not a handoff. For code outside the team's authority, report the defect and a prepared fix to its owner. Record durable tool friction as separate work without expanding the current task.

Checkpoint before a session ends, after a finding, after a head changes, and before review. Include the exact target revision, evidence locations, outstanding findings, next action, accountable owners, and the cumulative failed re-check count. Never turn an environment failure or an unfinished attempt into success.

## Review and proof authorities

The authority is [zero-trust-review](https://github.com/DereKk8/project-skills/blob/master/shared/zero-trust-review/SKILL.md), not a local copy. Before the first review and each version change, run its [sync-zero-trust](https://github.com/DereKk8/project-skills/blob/master/shared/sync-zero-trust/SKILL.md) procedure. Save the exact identity and the per-ID mechanism map in the issue record. Blocking gaps prevent acceptance. Do not reinterpret a rule to fit the runtime.

Use fresh review sessions with only the inputs permitted by that policy. The independent reviewer saves its intent reading before seeing the implementation. Keep author conversations, summaries, CI and earlier findings out of that session. Save the review before proof. Use the policy's staffing, pinning, proof, attack, verdict, and bounded-loop procedures directly. The coordinator holds its engineering-lead seat for engineering stops, model refusals, runner health, and sync drift (D10). The owner decides serious unresolved risk and re-scope after the policy's failed re-check bound. Preserve that count across sessions.

The project's `verify-*` skill owns surfaces, bootstrap, safe instances, feature coverage, and attacks. The owner deferred Verilex integration because it is not ready (D13). Do not use Verilex as a verification engine. Use regression tests and independent product-surface observations under zero-trust-review. Reviewers and verifiers retain their respective judgments. The pilot only patches Verilex issue 55, with Verilex as the product under test.

Choose verification for actual impact and risk. Evidence must apply to the exact candidate. Avoid irrelevant full suites, but never skip required checks or either required review phase. Every acceptance requirement needs applicable evidence on the exact candidate. Record actions, inputs, expected and observed results, and side effects. An author's assertion, a test count, or a link without its evidence is not proof.

## Accept and report

Run `assess` against the exact candidate using the recorded contract, independent review, proof records, and live forge checks. Missing, stale, inconclusive, or inapplicable evidence must fail. Do not bypass a refusal. The command is not a merge operation. Use the forge's revision-bound merge only after the review authority reports PASS and every required check is green on that same revision (D11).

Security, credentials, data loss, irreversible or production-facing actions, and changes to Lights Out gates or authority boundaries need the owner's explicit word before integration. The pilot also requires the owner's separate full zero-trust-review outside Lights Out before merge (D7). Do not treat a review PASS as merge permission.

Every resolution, including no-change, leaves an inspectable outcome tied to original intent and evidence. Surface material results, remaining risks, and product choices. Keep routine reports short. Do not require visual recaps or expose internal orchestration in product-facing artifacts. Never add agent commit co-authors. Keep private context and credentials out of public artifacts. After delivery, feedback becomes a new independently owned task through existing channels. No team must remain alive to monitor it.
