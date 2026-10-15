# Software factory: specification so far

Status: draft, Oct 15th, 2026. Nothing is built yet.
Section 5 lists the decisions that must be settled before an agent starts building.

## 1. Goal

Point any coding agent at an issue and at this repository, and the agent keeps working until its output can be trusted.
Trusted means one of two endings, and only these two:

- **Merged:** a merge-ready pull request that passed blind review, real CI, and the author's own proof, then merged.
- **Escalated:** a rare decision that only a human owner can make, stated so the owner can answer it in one read.

The factory must work for every kind of agent: a local CLI agent, a cloud agent, or a pull-request bot.
It must not depend on any one orchestrator, harness, or vendor.

## 2. Settled decisions

1. **Entry point.** `AGENTS.md` is the standard entry file. An agent that reads it learns the whole loop.
2. **Prose for judgment, scripts for guarantees.** Rules that only exist as text break again and again in practice. Every fixed, checkable rule lives in a script, a CI check, or a forge setting that refuses a bad state. Prose covers only the judgment that a script cannot make.
3. **The loop lives where every tool meets.** Some agents cannot be stopped from ending a turn, so the loop cannot live in an agent hook. It lives in the pull request: its required checks, and a script the agent re-runs to learn its next step.
4. **The loop ends only in merged or escalated.** The agent keeps fixing while each round fixes a new, independent defect. It escalates when a defect of the same kind returns, or when two fixes for one symptom fail.
5. **Fix findings at discovery.** A finding is fixed in the current change or in a separate pull request opened now. Nothing is filed for later, because a deferred finding has no owner. A finding in a dependency the team does not own is reported to its owner instead.
6. **Proof through verify skills.** Each consumer project owns a `verify-*` skill: how to run the real product, a map of its features, and attack scenarios. The author proves the change on the artifact the head ships, with a script anyone can re-run. CI runs that proof script.
7. **Blind review.** A reviewer agent reads only the issue and the pinned diff: no CI results, no author claims, no author summary. A separate verifier then tries to prove or disprove each suspicion, and attacks the change. The reviewer runs on a dedicated API budget (about $100 of Claude API credit for the pilot).
8. **Model family.** The reviewer and verifier prefer a different model family from the author whenever one is available. When the chain cannot mix families, the same family may check, and the review still counts.
9. **Attack lists are a floor, not a ceiling.** A project's attack scenarios are the minimum a checker must try. Every checker also invents and runs a few new attacks of its own and reports them separately. A new attack that finds a real bug joins the list.
10. **Visual recap.** Each delivery ends with a visual recap a human can read in one glance. (Its form is open, see section 5.)
11. **Public repository.** The factory is public, so CI has no billing limit and anyone can verify it. Private material stays out: client and product names, hosts, credential setup, business strategy, and any orchestrator vocabulary.
12. **Self-improving.** When an agent finds a loophole in the factory, it improves the factory through a pull request to this repository, under the same loop.
13. **Small and modular.** At most 10 skills at the start. Each rule has exactly one owner. Scripts and tool configs reference the skill that says how something is done; they never copy its text.
14. **One review contract.** The zero-trust review contract moves into this repository and becomes its single owner. Consumers load it by exact commit and blob and map every requirement to a named mechanism. The contract changes in items 4, 8 and 9 are made through its own proposal path.

## 3. Principles carried in from current practice

These come from a survey of the working system that ships trusted software today. Each one is a candidate for a script, not only text.

- The intent is the requester's own words from the task source. The author's summary never amends it.
- Nothing the author says, and no green proxy such as "it compiles", counts as proof.
- The reviewed head is pinned from the forge, and the merge is bound to that exact head.
- Review staffing scales with the changed-line count, tests and docs included.
- Never weaken a check to get green: no skipped tests, loosened thresholds, retries, or dropped required checks.
- A test calls the code the way users do and asserts a literal observed result. A test that greps source is not evidence.
- Work in small units that each end in a passing check. Order commits so the sequence proves itself: failing test, then fix.
- Reproduce a bug the way the user hits it before fixing it.
- Real CI runs on every change. An empty or unobserved check list is not green.
- Drive and clean up only the instances this run started, and keep the evidence through cleanup.
- Decide alone when a step is reversible and in scope. Stop for irreversible, shared, or scope-changing actions.
- An escalation states the requirement, the smallest compliant option, the consequences, and a recommendation.
- Base every branch on a fresh base, and check for overlapping work before starting and before ready.
- A repeated mistake is made impossible, not discouraged, and the new check must catch a real past case.
- Three verdicts: pass, fail, inconclusive. An environment failure is never a product failure.
- Checks are proportionate: state the realistic trigger and harm before demanding a fix.
- No agent co-author and no orchestration trace in project artifacts.
- Cost is judged per task done: strong models where judgment matters, cheap ones for clerical work.
- Instruction text states the intent and the proof of done, and is tested on the target models before it is trusted.

## 4. Reusable parts to port (generic form only)

- A rebase guard that classifies a branch as clean, stale, duplicated, or in real conflict by patch identity.
- A linker that lets every agent runtime find a project's `verify-*` skill.
- An author-prompts checker that puts the author's reasons on the pull request in self-contained form.
- A self-improvement ledger: no change without a recorded proposal, rejections remembered, rollback available.
- A skill-layout checker that requires a version bump on every change.
- A merge check that refuses unless a passing review verdict exists for the live head. A missing review record must refuse too, not only a failed one.
- Forge rulesets for required checks, which are free on public repositories.

## 5. Decisions to settle before an agent builds

Each item has a recommendation. Settle these first; a builder agent must not guess them.

| # | Decision | Recommendation |
|---|----------|----------------|
| D1 | The first issue and repository the pilot ships end to end | A small, real issue in a repository the owner controls, with a working verify skill |
| D2 | Who fixes the reviewer's findings: the same author agent or a fresh agent | The same author, for continuity; a fresh agent only after a repeated-theme escalation |
| D3 | The form of the visual recap | One generated page per pull request, linked in the PR body: what changed, the proof output, the review verdict |
| D4 | Which skills make the first 10 | Start from: review contract, verify-skill authoring, verify-skill maintenance, proof, attack, escalation, author prompts, self-improvement, rebase guard, and one entry skill |
| D5 | Guard for self-changes: a factory change must pass a replay check against real past cases, and no change may loosen a gate without the owner's word | Adopt it |
| D6 | Repository name | See the README |
| D7 | How the reviewer gets API credit in CI without exposing a secret in a public repository | A repository secret used only by a workflow that never runs on untrusted fork code |
| D8 | How a cloud agent or pull-request bot enters the loop | Through the same PR checks and the next-step script; no local tooling required |
| D9 | What proves the pilot works | One real issue taken from open to merged with every gate exercised, plus one seeded defect that the loop catches |

## 6. Out of scope for the pilot

- Fleet supervision, multi-agent scheduling, and quota management.
- Any project-specific verify skill. Those live in each consumer project.
- Production deploys and releases.
