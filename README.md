# lights-out

The owner's personal autonomous software factory: invoke an agent team on a task, and it delivers a verified resolution or asks for a product decision that only the owner can make.

[AGENTS.md](AGENTS.md) guides the agents that build and maintain this repository.

Status: pilot build in progress. Start an issue team with [ENTRY.md](ENTRY.md). The [command contract](docs/commands.md) describes the Go conductor's inspect, checkpoint, and assess commands. [AGENTS.md](AGENTS.md#commands) lists build and verification commands.

[SPEC.md](SPEC.md) remains the temporary contract until the real pilot demonstrations have evidence. Verilex integration is deferred by the owner (D13). The pilot patches Verilex issue 55 using regression tests and independent CLI observations, not Verilex as a verifier.

The owner controls product intent, accepted behavior, priorities, and product trade-offs. Agents own engineering and proof. An outcome may be a verified change or a demonstrated no-change result. Neither a PR nor a merge is the measure of success.

This personal factory uses existing tools. It has no standing agents, issue watchers, deployment system, or fleet supervisor. GitHub is the first integration, with Discord next (D8). Keep private context out of public evidence. Public CI costs do not predict private-project costs.

The pilot never merges. Its assessment checks evidence and returns `merge_authorized: false`. The owner must arrange a separate external zero-trust-review before integration. A future merge connection must preserve the authority described in the entry point.

The name comes from a lights-out factory: one that runs with no human in the room, because its output can be trusted.
