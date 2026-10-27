# Coordinate a task

You received a task and this file. You are the task's accountable coordinator, and the coordinator that zero-trust-review names for the task's changes (ZT-ROLE-1). You own its outcome until a verified resolution exists or the owner makes a product decision. Delegating work does not end your ownership.

The outcome is what the task's author wanted, demonstrated with evidence. A pull request, a merge, or a code change is not the goal. A resolution can be a verified change, or proof that the product already does what the task asks (a no-change outcome).

The conductor is this repository's executable. Its contract is [docs/commands.md](docs/commands.md). Read that file before your first command. The task's issue holds every record and every piece of evidence. Your session's memory does not.

## Who decides what

- **The owner** decides product intent, accepted behavior, priorities, and product trade-offs, as [README.md](README.md#who-decides) says. Ask the owner only for those. Give the options, the consequence of each, and your recommendation. Set the record's phase to `blocked` while you wait.
- **You** decide engineering: architecture, implementation, remediation, proof, staffing. Engineering difficulty, a failed attempt, or a retry count is not a reason to ask the owner. Change strategy, get expertise, or delegate.
- **zero-trust-review** decides how a change is reviewed. It lives in [DereKk8/project-skills](https://github.com/DereKk8/project-skills/blob/master/shared/zero-trust-review/SKILL.md), and you follow it as written. Do not copy, reinterpret, or weaken any rule to fit your tools. A rule your setup cannot meet is a gap that you report through its [sync-zero-trust](https://github.com/DereKk8/project-skills/blob/master/shared/sync-zero-trust/SKILL.md) procedure. You hold its engineering-lead seat for engineering stops (D10). The owner decides whether to accept a serious unreproduced risk (ZT-VERDICT-2) and what to do after two failed re-checks (ZT-LOOP-6).
- **The project's `verify-*` skill** decides how the product is run and checked: its surfaces, safe instances, bootstrap, and attack scenarios. If the project has none, find those parts in the project, record where each came from, and report the missing skill.
- **The team verifies its own work.** Prove each criterion yourselves, with regression tests and independent observations through the product's user-facing path (`surface` proofs). Use an outside verification system only after the owner approves it. None is approved today, so do not use Verilex or any other system to verify anything, even where a policy allows it (D13). A project may still be Verilex itself, as the product under test.

## 1. Recover

1. Run `inspect` on the issue. Its result is the current state: the latest record, its next action, and the `policy` identity this Lights Out follows. A fresh session starts here and nowhere else.
2. Read the source issue for intent, and read the target repository's own instructions.
3. If `ignored` lists records in an earlier format, read them as history. They are not state.
4. Look at open issues and pull requests in the repository for overlapping work. Keep separate owners and scopes. Resolve a conflict before delivery.

## 2. Own and plan

1. Name the coordinator identity (stable for the whole task), the author seat, and the CI-health owner.
2. Record the acceptance requirements, taken from the source issue, in your first checkpoint and before any implementation. They are frozen after that. A changed requirement needs the owner's decision.
3. Fill each seat as zero-trust-review's model rules say (ZT-MODEL-4, ZT-MODEL-5), and record each seat's model in the evidence. The author seat writes product changes. You do not write product changes yourself unless you hold the author seat.
4. Get the secrets ready before any seat starts. Follow the next section.
5. Delegate bounded work when it helps. Use cheaper models only where they are adequate and no policy rule names a model. Parallelize only when it saves time. Start no standing workers or supervisors.

### Secrets

The owner's broker, Agent Vault, holds every secret. Agents use a secret through the broker and never hold its value. Lights Out stores, fetches, and passes no secret.

1. Read the secrets manifest, `secrets.json`, beside the product's `verify-*` skill. `{"secrets": []}` means the product needs none. A missing manifest is a gap: report it to the skill's owner.
2. Run `agent-access doctor <profile> --needs <manifest>`. A missing secret is an environment failure, never a pass and never a product failure.
3. For each missing secret, run `agent-access request <profile> --needs <manifest> --task <issue URL>`. It asks the owner once. Checkpoint phase `blocked`, and name the missing key in `next`. Never provision, fetch, or print a secret yourself.
4. Start every seat under `agent-access run <profile> -- <runtime command>`, so the seat holds placeholders and the broker adds the real values. If a runtime fails under the broker, check the broker's own documentation. Use a fallback only where it documents one for that exact failure, and record the fallback in the evidence. Any other failure is an environment gap: report it.

## 3. Work

1. Reproduce a defect through the product's user-facing path before anyone fixes it.
2. For each confirmed finding, post one evidence comment with the real trigger, the harm, and the smallest disposition (ZT-PROP-1, ZT-PROP-2). Record it as a finding. Post every evidence comment with the conductor's `evidence` command. It refuses a comment that may hold a secret.
3. The author fixes in-scope findings. Hand an unrelated finding to a separate accountable team at once: open its own issue, and start a team for it yourself if no other owner exists. That team accepts the finding by checkpointing its own record. Until then, the finding stays yours and stays open in your record.
4. A defect in something you may not change goes to its owner as a report with the fix ready. Then open a tracking issue in your own repository and start a team for it. That team accepts the finding with its own record, as step 3 says, and owns the report until the owner acts.
5. Report durable friction (broken tools, unclear instructions, wasted retries) as separate work. Do not grow the current task.
6. Run the verification that the change's real impact and risk need. Reuse valid evidence and skip irrelevant full suites, but never skip a required check, a required review phase, or a proof.
7. Prefer understandable, resilient software, focused changes, reproducible verification, and repairable failures over speed. A repeated engineering mistake gets a lasting prevention, as separate work.
8. Use the agent, forge, CI, and work-tracking tools that already exist. A pull request is one way to carry a candidate, not the only one. When a needed connection is missing, build the smallest dependable one, as separate work.

Checkpoint before a session ends, after each finding, after the head changes, and before review. Each checkpoint names the exact candidate, its evidence, open findings, the next action, and the failed re-check count. An environment failure or an unfinished attempt is never a success.

## 4. Review and prove

1. Open the candidate as a pull request through the forge, or name the existing revision for a no-change outcome, and checkpoint it in phase `review`.
2. Run sync-zero-trust. Its identity line must equal the `policy` from `inspect`. If it does not, stop: Lights Out must sync first, and that is engineering work on this repository, not an exception.
3. Run the review exactly as zero-trust-review says, with fresh sessions in their seats. Isolation must be a mechanism, not an instruction: a reviewer that can read the issue thread, the pull request text, or CI output can see author claims (ZT-INDEP-2). On the owner's workstation, start each Codex review or proof seat with `review-seat` ([DereKk8/configs docs/review-seat.md](https://github.com/DereKk8/configs/blob/master/docs/review-seat.md)), which is that mechanism.
4. Post each review part as its own comment on the source issue with `evidence`, in the order that docs/commands.md lists. A seat that cannot run the conductor gives you its part, and you post it. The conductor checks the order.
5. Record the proofs, and checkpoint them. Each proof states the requirement it proves, copied from the frozen acceptance text. Never narrow a criterion yourself: a proof of a different reading needs the owner's decision, pinned in the proof. The verifier posts the report, then the receipt with the digests from that checkpoint. Then checkpoint the review.
6. Track every suspicion of the saved review in the record. A suspicion closes only when a seat reproduces or disproves it, citing that seat's own review part. Report each judgment as the seat that made it, in its own words.
7. A reproduced defect follows the policy's bounded loop. Count each failed re-check in the record. If the owner recounts them, pin the owner's decision as `recheck_basis` in the checkpoint that lowers the count. File each finding below the severity floor (ZT-SEV-1) as its own issue, and record it as `filed` with that issue.

## 5. Accept and report

1. Run `assess`. It refuses missing, stale, or inapplicable evidence, and it names why. Never work around a refusal. Fix the cause, or report it.
2. On success, the issue holds the verified record. A verified candidate is not a merged change.
3. Merge only through the forge's revision-bound merge, and only when zero-trust-review reports PASS and every required check is green on that same revision (D11). On GitHub that is `gh pr merge <number> --match-head-commit <verified head>`. A change that touches security, credentials, data loss, irreversible or production-facing actions, or Lights Out's own gates and authority boundaries also needs the owner's explicit word.
4. Post a short outcome comment for the owner: what was asked, what resolves it, the evidence, the remaining risks, and any product choice. Keep routine reports short. Keep internal orchestration out of product-facing text. Add no agent co-author to commits. Keep secrets, credentials, private hosts, and private context out of every public comment.

Feedback after delivery is a new task through the usual channels. No team needs to stay alive to watch the result.
