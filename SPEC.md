# Lights Out - Specification

Status: Draft · October 15, 2026 · Design only; nothing is built yet. Section 5 lists the open decisions and which of them the owner settles before the pilot. This file is temporary: the pilot build ends by moving its lasting requirements to durable owners and deleting it (section 6).

This file defines what Lights Out must do. [AGENTS.md](AGENTS.md) guides the agents that build and maintain this repository.

## 1. Purpose

Lights Out is the owner's personal autonomous software factory. Invoke an agent team on a task; it independently delivers a verified resolution or seeks a genuinely necessary product decision.

The measure of success is correct, maintainable software - not code generated, PRs merged, agents occupied, or reports produced. A resolution may be a verified change, proof that existing behavior already meets the request, or a demonstrated no-change outcome.

Humans define product direction. Agents own the engineering needed to realize it. As agents improve, verification must improve with them; trust comes from evidence, never an agent's confidence. Compute, latency, and human attention are scarce resources.

Lights Out is built for the owner's projects, not for commercial adoption. It must remain portable across agent models, runtimes, orchestrators, code hosts, and work-tracking systems.

Lights Out is a thin conductor. Its own code calls the tools that already own each job and never re-implements them (requirement 21).

## 2. Required behavior

In this specification, an issue is one task and its record, whatever channel the task came from.

### Task ownership and execution

1. **Issue-driven work.** Tasks reach Lights Out through existing channels such as Linear, Discord, Slack, or another source. A team works when invoked; Lights Out does not need standing agents, issue watchers, PR-comment listeners, or bespoke feedback hooks. The owner invokes a team by giving one agent the task and the Lights Out entry point; that agent needs no other setup knowledge.
2. **One accountable coordinator per issue.** A capable, strong-reasoning coordinator interprets intent, plans execution, allocates work, integrates results, and owns the outcome through verification. It is the coordinator that zero-trust-review names (ZT-ROLE-1) for the issue's changes. It delegates bounded work to suitable agents, uses cheaper models where adequate, and parallelizes only when doing so helps.
3. **Outcome over artifact.** Finish by demonstrating that the original task's intended outcome has been met. A PR, merge, or code change is not inherently required.
4. **Human vision, autonomous engineering.** The owner decides what to build, for whom, what behavior is desired, and which product trade-offs are acceptable. Agents choose architecture, implementation, remediation, and proof within that intent. Escalate only choices requiring product judgment, such as drift, a pivot, or a change to accepted behavior or priorities; present options, consequences, and a recommendation.
5. **Persistent engineering ownership.** Engineering difficulty, failed attempts, or a retry count do not by themselves justify human escalation. The coordinator changes strategy, seeks expertise, or delegates as needed. Unresolved technical work remains visibly owned; agents never claim success without proof or bypass safeguards to force progress. The exception is the fix-round bound in zero-trust-review (ZT-LOOP-3 and ZT-LOOP-6, with the engineering-lead seat set by D10). When that bound ends the fix loop, the owner's re-scope decision is a product decision under requirement 4.
6. **Immediate defect response with preserved scope.** Every confirmed defect triggers corrective work upon discovery. A defect is confirmed when it is reproduced and zero-trust-review's proportionality rules (ZT-PROP-1, ZT-PROP-2) find a real trigger and harm; its response is the smallest one that meets the stated intent. The current team fixes in-scope findings; unrelated findings are immediately assigned to a separate accountable agent, team, or orchestrator, which the coordinator may start itself. The discovering team retains responsibility until a receiving owner accepts the work, without absorbing unrelated work into its own issue; a filed task that nobody has accepted is not a handoff. A defect in something Lights Out may not change goes to its owner as a report with the fix ready.
7. **Independent issues, aware of overlap.** Issue teams retain separate owners and scopes. They discover relevant concurrent work, avoid duplicate effort, and resolve dependencies or conflicts before delivery. No fleet-wide coordinator is assumed.

### Verification and delivery

8. **Existing review authority.** zero-trust-review remains the authoritative review policy in DereKk8/project-skills. Lights Out invokes and honors it; it does not move, duplicate, reinterpret, or weaken its requirements. Lights Out records which version it follows and maps each requirement ID to the mechanism that meets it, through that policy's own sync procedure (sync-zero-trust). A requirement it cannot meet is a gap reported through that procedure, never a local exception.
9. **Existing product-proof authority.** Projects describe how to run and check their product - its surfaces, feature map, safe instances, and attack scenarios - in their `verify-*` skills. Verilex executes the applicable claims and verification words, records evidence, and manages its freshness. Zero-trust reviewers and verifiers call Verilex; it does not replace their roles. Lights Out consumes that proof rather than implementing a competing verifier. Section 4 states how Verilex may later become part of Lights Out.
10. **Exact, applicable evidence.** Each change candidate is identified by its exact artifact or revision and is tied to the original intent, its target, the acceptance requirements that apply, and its evidence, so anyone can check it without trusting its author. Missing, inconclusive, stale, or inapplicable proof is not a pass. An agent's own assertions cannot substitute for evidence. Distinguish product failures from environment failures; neither an inconclusive run nor an environment error is proof.
11. **Adaptive execution, unchanged standards.** Run the verification required for a change's actual impact and risk. Reuse valid evidence and avoid irrelevant full-suite runs; strengthen checking where warranted. Efficiency never excuses skipping required review, relaxing checks, or treating absent proof as green.
12. **Continuation beyond a session.** The issue's current state, evidence, outstanding findings, and next action survive individual agents and sessions. Neither an agent hook nor a particular PR workflow is the sole keeper of progress. A fresh agent given only the issue and the entry point can recover that state and continue.
13. **Portable integration.** Use the agent, forge, CI, and work-tracking mechanisms already available. Agents may build the smallest dependable missing connection. A pull request is one possible carrier for a change candidate, not the factory's universal interface.
14. **Maintainability is part of done.** Prefer understandable, resilient software, focused changes, reproducible verification, and repairable failures over agent throughput. Reproduce behavioral defects through the affected user-facing path when possible. Repeated engineering mistakes should lead to lasting prevention.
15. **Feedback becomes new work.** Product feedback after delivery enters as fresh, independently owned tasks through existing channels. The original team is not required to remain alive or monitor the software indefinitely. Delivery and recovery can use existing systems; Lights Out need not own deployments.

### Improvement and communication

16. **Friction drives improvement.** Identify durable, reproducible obstacles, such as wasted retries, unclear instructions, broken tools, unnecessary work, and recurring failures. Route actionable friction to separately owned improvements without expanding the delivering issue. Prefer removing causes to documenting workarounds.
17. **Governed self-improvement.** Agents autonomously maintain and improve engineering mechanisms, with observable benefit and preservation of existing guarantees. Modifications that change product intent, approved trade-offs, or authority boundaries require the owner's decision. External review and verification authorities remain external unless the owner decides otherwise.
18. **Adaptive modularity.** Reuse working capabilities before adding new ones. Add, combine, or retire skills and integrations based on demonstrated value, with clear ownership and no duplicated authority. No arbitrary skill-count limit.
19. **Human-attention discipline.** Every completed task leaves an inspectable outcome and its evidence. Proactively surface material results, risks, and product choices; adapt format and depth to significance. No mandatory visual recap for routine changes. Product-facing artifacts need not expose internal agent orchestration, and commits carry no agent co-author line.
20. **Public mechanics, private context.** The reusable factory may be public. Project secrets, credentials, private hosts, client details, and business strategy do not belong in its public artifacts. Do not assume that the economics of public CI carry over to private projects.

### Thin conductor

21. **Thin conductor.** Lights Out's own code is a thin conductor. It calls the tools that already own each job - the code host, the work tracker, agent CLIs, zero-trust-review, Verilex, and `verify-*` skills - and passes typed records between them. Judgment stays with the coordinator and its agents (requirement 2). The conductor never owns source hosting, branches, pull requests, or merge state; task tracking or notification; model execution or agent sessions; review policy; proof execution or evidence freshness; how a product is run and checked; or deployments. When a needed job has no owner, the conductor adds the smallest dependable connection (requirement 13), not a replacement.
22. **Go for the conductor.** The conductor's scripts and wrappers are written in Go, chosen for loud failure, structured data in and out, tests through the public entry point, and agent maintainability. Each conductor command takes and returns structured data, not prose.
23. **Loud failure.** A conductor command that meets an error stops, exits with a failure status, and names the error; it never drops an error or reports success after one. Because Go lets a careless `_` drop an error, a lint that rejects dropped and unchecked errors is a required check from the first commit that adds Go code.
24. **Strong typing.** Every record that crosses a boundary - issue input, tool output, evidence record, and verdict - is a typed value, validated where it enters. A boundary record is never an untyped map or loosely parsed text, and a record that fails validation is an error under requirement 23.

## 3. Enforcement boundary

Requirements that can be checked must be enforced by executable mechanisms, not promises in prompts. The factory must reject unsupported acceptance (accepting a change without its required evidence), stale verdicts (a verdict for a revision other than the one being accepted), and unauthorized weakening of safeguards. The external authorities define their own detailed proof and review contracts; this specification defines how Lights Out uses them.

Work completion, review, and integration must remain correct when agents or implementations change. Introduce only the smallest mechanisms needed to make those guarantees real.

## 4. Pilot

Objective: Prove a complete issue-to-outcome loop with a single accountable coordinator and useful delegation where appropriate, using existing tools and authorities.

The pilot builds the smallest working form of:

- the entry point an agent follows to coordinate an issue (requirements 1 and 2);
- the issue record that holds state, evidence, findings, and next action outside any session (requirement 12);
- the executable checks of section 3, bound to the exact revision (requirement 10);
- the outcome report each completed issue leaves (requirement 19).

The pilot's code follows requirements 21 to 24 from its first commit.

The pilot must demonstrate, with evidence kept in the issue record:

- One real owner-controlled issue (D1) taken from intent through independently supported resolution.
- A confirmed defect found and repaired, including separate ownership when outside the issue's scope. Where the real run meets no defect for one of these paths (in scope or out of scope), a seeded defect, labeled as seeded, exercises that path.
- A seeded defect caught; unsupported or stale evidence rejected.
- Required checks honored while an unrelated low-risk change avoids irrelevant full-suite work.
- Persistent issue state and an inspectable result without relying on one live agent session: a fresh session resumes the issue from its record alone, and none of the sessions that run the issue is one that built Lights Out.
- What Verilex contributed: the proof it ran, and what it caught, missed, or could not run.

Use existing PR checks or a next-step command if helpful; neither defines the lasting architecture. Do not build an orchestrator, verification engine, or new work-tracking platform merely to demonstrate the loop.

Verilex stays an external proof authority during the pilot (requirement 9). It may become part of Lights Out after the running pilot demonstrates its real value. The owner makes that decision on the pilot's evidence; the pilot builds nothing for it.

Not part of the pilot: permanent fleet supervision, standing worker pools, quota management, project-specific `verify-*` implementations, production release automation, self-improvement machinery for requirements 16 and 17 (friction met during the pilot is reported, not tooled), or replacing zero-trust-review or Verilex.

## 5. Open decisions

The owner settles each decision here, with the recommendation or another answer. The IDs continue the list in issue 1.

Needed before the pilot:

- **D1. Pilot target.** The real issue and repository the pilot runs on. Recommendation: a small, real issue in an owner-controlled repository that already has a `verify-*` skill Verilex can run, on a forge that can refuse a merge without the required checks.
- **D7. Review-seat access and spend.** Where the reviewer and verifier models come from, given that zero-trust-review requires a model family other than the author's where one is available (ZT-MODEL-1), and the pilot's spend limit. Recommendation: use an agent CLI of another strong-tier family that is already signed in where the team runs, set a pilot spend cap, and keep model credentials out of CI; CI checks the recorded verdict for the exact revision instead of running the reviewer.
- **D8. Initial integration.** Which channel carries the task, its record, escalations, and the outcome, and which agent runtime and code host the pilot uses. Recommendation: the pilot repository's own issue tracker and forge, with an agent runtime the owner already uses; the entry point stays runtime-neutral.
- **D10. Engineering-lead seat.** Who holds the engineering-lead role that zero-trust-review reports to (ZT-LOOP-3, ZT-MODEL-3, ZT-TEAM-3, ZT-VERDICT-2, and sync drift). Recommendation: the issue's coordinator for engineering stops, and the owner for accepting a serious unreproduced risk (ZT-VERDICT-2), because that is a product trade-off. The re-scope after ZT-LOOP-6 stays with the owner either way.
- **D11. Merge authority.** Who may merge an accepted change. Recommendation: the coordinator, when zero-trust-review reports PASS and every required check is green on the same revision, with the merge bound to that revision; the owner's word for changes that touch security, credentials, data loss, irreversible or production-facing actions, or Lights Out's own gates and authority boundaries.

Not needed for the pilot:

- **D12. Cross-issue coordination.** The lightest effective mechanism beyond overlap awareness (requirement 7). Recommendation: keep overlap awareness alone until a real cross-issue conflict shows a need. No extra leadership layer is authorized by default.

## 6. Lifecycle of this file

SPEC.md is the temporary contract for the pilot build, not a lasting document. After the demonstrations in section 4 have evidence, the build's last change does three things:

1. It moves every lasting requirement to exactly one durable owner: README.md for the product behavior the owner sees, the entry point for what invoked teams do, AGENTS.md for how agents develop and maintain this repository, and executable checks with tests for every rule that section 3 says must be enforced. Other files point to that owner instead of repeating it.
2. It records each settled decision from section 5 with the requirement it settles, and moves each unsettled decision to an issue on this repository.
3. It deletes SPEC.md and every reference to it, including the build-only parts of AGENTS.md and README.md.

The change's description maps every requirement number to its new owner, so the owner can confirm that nothing was lost. Because the move could change a requirement, the owner reviews and merges that change. Section 4 is not lasting; its evidence stays in the pilot issue's record.

Durable owners for the requirements that most need to last:

| Requirement | Durable owner | Enforced by |
|---|---|---|
| 21. Thin conductor | AGENTS.md | - |
| 22. Go for the conductor | AGENTS.md | - |
| 23. Loud failure | AGENTS.md | The dropped-error lint and its fixture |
| 24. Strong typing | AGENTS.md | The decode lint and the decode package's tests |
| 8 and 9. Review and proof authorities | The entry point | The checks of section 3 |
| 4. Human-owned product decisions | README.md | - |
| 17. Governed self-improvement | AGENTS.md | - |
| Section 3. Enforcement boundary | AGENTS.md, which lists each check's command | The checks and their tests |
