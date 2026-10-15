# Lights Out — Specification

Status: Draft · October 15, 2026 · Design only; nothing is built yet.

## 1. Purpose

Lights Out is the owner's personal autonomous software factory. Invoke an agent team on a task; it independently delivers a verified resolution or seeks a genuinely necessary product decision.

The measure of success is correct, maintainable software—not code generated, PRs merged, agents occupied, or reports produced. A resolution may be a verified change, proof that existing behavior already meets the request, or a demonstrated no-change outcome.

Humans define product direction. Agents own the engineering needed to realize it. As agents improve, verification must improve with them; trust comes from evidence, never an agent's confidence. Compute, latency, and human attention are scarce resources.

Lights Out is built for the owner's projects, not for commercial adoption. It must remain portable across agent models, runtimes, orchestrators, code hosts, and work-tracking systems.

## 2. Required behavior

### Task ownership and execution

1. **Issue-driven work.** Tasks reach Lights Out through existing channels such as Linear, Discord, Slack, or another source. A team works when invoked; Lights Out does not need standing agents, issue watchers, PR-comment listeners, or bespoke feedback hooks.
2. **One accountable coordinator per issue.** A capable, strong-reasoning coordinator interprets intent, plans execution, allocates work, integrates results, and owns the outcome through verification. It delegates bounded work to suitable agents, uses cheaper models where adequate, and parallelizes only when doing so helps.
3. **Outcome over artifact.** Finish by demonstrating that the original task's intended outcome has been met. A PR, merge, or code change is not inherently required.
4. **Human vision, autonomous engineering.** The owner decides what to build, for whom, what behavior is desired, and which product trade-offs are acceptable. Agents choose architecture, implementation, remediation, and proof within that intent. Escalate only choices requiring product judgment, such as drift, a pivot, or a change to accepted behavior or priorities; present options, consequences, and a recommendation.
5. **Persistent engineering ownership.** Engineering difficulty, failed attempts, or a fixed retry count do not justify human escalation. The coordinator changes strategy, seeks expertise, or delegates as needed. Unresolved technical work remains visibly owned; agents never claim success without proof or bypass safeguards to force progress.
6. **Immediate defect response with preserved scope.** Every confirmed defect triggers corrective work upon discovery. The current team fixes in-scope findings; unrelated findings are immediately assigned to a separate accountable agent, team, or orchestrator. The discovering team retains responsibility until handoff is acknowledged, without absorbing unrelated work into its own issue.
7. **Independent issues, aware of overlap.** Issue teams retain separate owners and scopes. They discover relevant concurrent work, avoid duplicate effort, and resolve dependencies or conflicts before delivery. No fleet-wide coordinator is assumed.

### Verification and delivery

8. **Existing review authority.** zero-trust-review remains the authoritative review policy in DereKk8/project-skills. Lights Out invokes and honors it; it does not move, duplicate, reinterpret, or weaken its requirements.
9. **Existing product-proof authority.** Projects define product intent and executable checks in their verify-* skills. Verilex executes applicable claims and words, records evidence, and manages freshness. Lights Out consumes that proof rather than implementing a competing verifier.
10. **Exact, applicable evidence.** A proposed change remains identifiable independently of its author and tied to original intent, the exact artifact or revision, its target, applicable acceptance requirements, and supporting evidence. Missing, inconclusive, stale, or inapplicable proof is not a pass. An agent's own assertions cannot substitute for evidence. Distinguish product failures from environment failures; neither an inconclusive run nor an environment error is proof.
11. **Adaptive execution, unchanged standards.** Run the verification required for a change's actual impact and risk. Reuse valid evidence and avoid irrelevant full-suite runs; strengthen checking where warranted. Efficiency never excuses skipping required review, relaxing checks, or treating absent proof as green. Review-policy decisions remain with zero-trust-review.
12. **Continuation beyond a session.** The issue's current state, evidence, outstanding findings, and next action survive individual agents and sessions. Neither an agent hook nor a particular PR workflow is the sole keeper of progress.
13. **Portable integration.** Use the agent, forge, CI, and work-tracking mechanisms already available. Agents may build the smallest dependable missing connection. A pull request is one possible carrier for a change candidate, not the factory's universal interface.
14. **Maintainability is part of done.** Prefer understandable, resilient software, focused changes, reproducible verification, and repairable failures over agent throughput. Reproduce behavioral defects through the affected user-facing path when possible. Repeated engineering mistakes should lead to lasting prevention.
15. **Feedback becomes new work.** Product feedback after delivery enters as fresh, independently owned tasks through existing channels. The original team is not required to remain alive or monitor the software indefinitely. Delivery and recovery can use existing systems; Lights Out need not own deployments.

### Improvement and communication

16. **Friction drives improvement.** Identify durable, reproducible obstacles—wasted retries, unclear instructions, broken tools, unnecessary work, and recurring failures. Route actionable friction to separately owned improvements without expanding the delivering issue. Prefer removing causes to documenting workarounds.
17. **Governed self-improvement.** Agents autonomously maintain and improve engineering mechanisms, with observable benefit and preservation of existing guarantees. Modifications that change product intent, approved trade-offs, or authority boundaries require the owner's decision. External review and verification authorities remain external.
18. **Adaptive modularity.** Reuse working capabilities before adding new ones. Add, combine, or retire skills and integrations based on demonstrated value, with clear ownership and no duplicated authority. No arbitrary skill-count limit.
19. **Human-attention discipline.** Every completed task leaves an inspectable outcome and its evidence. Proactively surface material results, risks, and product choices; adapt format and depth to significance. No mandatory visual recap for routine changes. Product-facing artifacts need not expose internal agent orchestration.
20. **Public mechanics, private context.** The reusable factory may be public. Project secrets, credentials, private hosts, client details, and business strategy do not belong in its public artifacts. Do not assume that the economics of public CI carry over to private projects.

## 3. Enforcement boundary

Requirements that can be checked must be enforced by executable mechanisms, not promises in prompts. The factory must reject unsupported acceptance, stale verdicts, and unauthorized weakening of safeguards. The external authorities define their own detailed proof and review contracts; this specification defines how Lights Out uses them.

Work completion, review, and integration must remain correct when agents or implementations change. Introduce only the smallest mechanisms needed to make those guarantees real.

## 4. Pilot

Objective: Prove a complete issue-to-outcome loop with a single accountable coordinator and useful delegation where appropriate, using existing tools and authorities.

The pilot must demonstrate:

- One real owner-controlled issue taken from intent through independently supported resolution.
- A confirmed defect found and repaired, including separate ownership when outside the issue's scope.
- A seeded defect caught; unsupported or stale evidence rejected.
- Required checks honored while an unrelated low-risk change avoids irrelevant full-suite work.
- Persistent issue state and an inspectable result without relying on one live agent session.

Use existing PR checks or a next-step command if helpful; neither defines the lasting architecture. Do not build an orchestrator, verification engine, or new work-tracking platform merely to demonstrate the loop.

Not part of the pilot: permanent fleet supervision, standing worker pools, quota management, project-specific verify-* implementations, production release automation, or replacing zero-trust-review or Verilex.

## Choices not settled

- Pilot target: the first real issue and repository.
- Initial integration: which existing agent runtime, work channel, and code-host mechanisms to use.
- Credentials and cost: the safe source and limits for any required reviewer/API spending.
- Cross-issue coordination: the lightest effective mechanism beyond overlap awareness; no extra leadership layer is authorized by default.

Implementation choices may be made autonomously if they do not alter product intent, permissions, or accepted guarantees. The owner decides genuine product trade-offs.
