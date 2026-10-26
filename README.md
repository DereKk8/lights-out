# lights-out

Lights Out is the owner's personal autonomous software factory. The owner gives an agent a task, and the agent's team comes back with a verified resolution, or with a product decision that only the owner can make.

The measure of success is correct, maintainable software. Code generated, pull requests merged, agents kept busy, and reports produced do not count. A resolution can be a verified change, or proof that the product already does what the task asks. Trust comes from evidence, never from an agent's confidence. Compute, latency, and human attention are scarce, so the factory spends them only where they change the outcome.

Lights Out is built for the owner's projects, not for commercial use. It stays portable across agent models, runtimes, orchestrators, code hosts, and work trackers.

## How the owner uses it

1. A task arrives through a channel the owner already uses. The task and its record live in one issue. GitHub issues are the only channel today; Discord is the next one (D8).
2. The owner gives one agent the task and [ENTRY.md](ENTRY.md). That agent needs no other setup knowledge. It becomes the task's accountable coordinator and runs the team.
3. The team works only while it is invoked. There are no standing agents, issue watchers, pull request listeners, or deployment system.
4. The issue holds the whole state: the acceptance contract, evidence, findings, and the next action. Any fresh session can pick the task up from the issue alone.
5. When the gate passes, the issue holds a verified record and a short outcome report for the owner. Feedback after delivery is a new task.

The conductor is a small Go executable that keeps the issue record and runs the acceptance gate. Its contract is [docs/commands.md](docs/commands.md). [AGENTS.md](AGENTS.md) is for agents that maintain this repository.

## Who decides

The owner decides what to build, for whom, what behavior is wanted, and which product trade-offs are acceptable. Agents decide the architecture, the implementation, the remediation, and the proof within that intent.

A team asks the owner only for a choice that needs product judgment: drift from the intent, a pivot, or a change to accepted behavior or priorities. It presents the options, the consequence of each, and a recommendation. Engineering difficulty is never a reason to ask.

## Public and private

The factory's reusable mechanics may be public. Project secrets, credentials, private hosts, client details, and business strategy never go into its public artifacts, including issue records on public repositories. The cost of public CI says nothing about the cost of CI for a private project.

## Status

The pilot ran on [DereKk8/verilex issue 55](https://github.com/DereKk8/verilex/issues/55), whose record holds its evidence. On October 26, 2026, its gate verified [PR 57](https://github.com/DereKk8/verilex/pull/57), after four owner decisions along the way. Teams verify their own work. An outside verification system, such as Verilex, joins only on the owner's word (D13).

## Decisions

The owner settled these. Each one lives in the file named here. Open decisions are issues on this repository.

- **D1. Pilot target:** [DereKk8/verilex issue 55](https://github.com/DereKk8/verilex/issues/55). Settled October 15, 2026.
- **D7. Pilot review:** the owner runs a separate, full zero-trust-review before the pilot's change merges. Settled October 15, 2026. Its same-family review part was retired on October 18, 2026: zero-trust-review names the seat models itself (ZT-MODEL-4, ZT-MODEL-5), and teams follow it ([ENTRY.md](ENTRY.md)).
- **D8. Channels:** GitHub issues first, and Discord next (this file). Settled October 15, 2026.
- **D10. Engineering-lead seat:** the coordinator for engineering stops, and the owner for product trade-offs ([ENTRY.md](ENTRY.md)). Settled October 15, 2026.
- **D11. Merge authority:** the coordinator, on PASS and green required checks for the same revision, with the owner's word for high-risk changes ([ENTRY.md](ENTRY.md)). Settled October 15, 2026.
- **D12. Coordination across issues:** each team's overlap check alone, until a real collision shows a need. No layer above the teams ([ENTRY.md](ENTRY.md)). Settled October 18, 2026.
- **D13. Verification:** teams verify their own work. An outside verification system joins only on the owner's word, as one general proof kind that names the system, and Lights Out may run with none. None is approved today ([ENTRY.md](ENTRY.md), [AGENTS.md](AGENTS.md)). Settled during the pilot build, extended October 18, 2026.

Also settled on October 18, 2026:

- **Defects for outside owners:** the team reports the defect with the fix ready, and a tracking team in Lights Out accepts the finding ([ENTRY.md](ENTRY.md)).
- **Changes to this repository:** small patches go straight to `main`. Big changes, gate changes, and every contribution from others go through a pull request ([AGENTS.md](AGENTS.md)).
- **Seats and the broker:** every seat starts under the broker. A runtime uses a fallback only where the broker documents one for that exact failure ([ENTRY.md](ENTRY.md)).

The name comes from a lights-out factory: one that runs with no human in the room, because its output can be trusted.

Settled on October 26, 2026:

- **Policy version:** Lights Out follows one exact zero-trust-review version, and syncs to each new version before teams review under it ([AGENTS.md](AGENTS.md)).
- **Re-check recount:** the owner may rule that a review was not a re-check. The checkpoint that lowers the count pins that ruling as `recheck_basis` ([docs/commands.md](docs/commands.md)).
- **Below-floor findings:** a finding below zero-trust-review's severity floor is filed as its own issue and does not block the gate ([docs/commands.md](docs/commands.md)).

Open work with the owner's direction recorded: [#6](https://github.com/DereKk8/lights-out/issues/6) (an outside verification system), [#10](https://github.com/DereKk8/lights-out/issues/10) (cloud-first seats and their secrets), [#11](https://github.com/DereKk8/lights-out/issues/11) (resume a stopped team), and [#12](https://github.com/DereKk8/lights-out/issues/12) (Grok work through the Grok Bot team).
