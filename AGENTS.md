# Lights Out - Agent contributor guide

This file is for agents that build and maintain this repository. [SPEC.md](SPEC.md) defines what Lights Out must do; read it before you propose or edit a change. This file does not restate SPEC.md, and it does not give permission to amend it. It is not the runtime contract for the teams Lights Out runs on other projects; that contract is the entry point SPEC.md requirement 1 describes.

## Current state

The repository holds only the specification. No build, test, or run command exists yet. When you add code, tooling, or skills, list each real entry point and verification command under Commands in the same change. Never list a command that does not exist.

## Commands

None yet.

## Authority

- Change SPEC.md only to record a decision the owner made (requirements 4 and 17).
- Do not guess an open decision in SPEC.md section 5. Build the parts that do not depend on it, and ask the owner as requirement 4 says, in an issue on this repository.
- Until decision D11 is settled, the owner merges changes to this repository.

## How to work

1. Inspect the repository, its instructions, its checks, and related open work before you edit. Start from a fresh `main`.
2. Change the smallest coherent surface: no speculative infrastructure, duplicated rule, or incidental edit.
3. Work in small units that each end in a passing check. Test a capability through its public entry point, and assert the observed result.
4. Work on this repository is a task like any other, so SPEC.md sections 2 and 3 apply to it. Until Lights Out can run its own loop, run zero-trust-review on your change directly (requirement 8).
5. Update the instructions, tests, and docs that your change makes wrong, in the same change.

## Writing

Write in plain English: short sentences, one idea each, active voice, and common words. Use "-", never an em dash.

## Before you report done

Your report is the inspectable outcome that requirement 19 asks for. State what changed and which SPEC.md requirements it serves, what you verified (the command and the observed result), what is still unverified, and every finding with its fix or the owner who accepted it.

## Maintaining this file

Keep this file short. Point to SPEC.md and to real commands instead of repeating them.
