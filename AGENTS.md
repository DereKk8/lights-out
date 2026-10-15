# Lights Out — Agent contributor guide

This file is for agents modifying or maintaining this repository. It is not the runtime contract for teams working on other projects.

Source of truth: [SPEC.md](SPEC.md) defines what Lights Out must do. Read it before proposing or editing a change. Do not repeat its product requirements here or treat these instructions as permission to amend them.

## How to contribute and maintain

1. **Orient.** Inspect the actual repository, applicable instructions, existing checks, and relevant work before editing. Follow existing entry points and skills where present. This repository is currently specification-only; do not invent build or test commands.
2. **Stay within the contract.** Implement, repair, refactor, and simplify autonomously when the change preserves SPEC.md. Do not silently revise product behavior, priorities, accepted trade-offs, or authority boundaries. Bring those decisions to the owner and update the spec only when approved.
3. **Change the smallest coherent surface.** Reuse existing tools and contracts; avoid competing abstractions, speculative infrastructure, duplicated rules, and incidental edits. Check for overlapping work before starting and before delivery.
4. **Preserve external authorities.** For review, follow the authoritative zero-trust-review skill. For executable product proof, integrate Verilex and project-owned verify-* skills. Do not copy or amend their policies here.
5. **Prove repository changes.** Exercise relevant available checks against the exact changed revision. Add a reproducible regression check for fixed defects where practical. Never weaken a gate to obtain a passing result. If verification cannot run, report what remains unproven; never manufacture a green result.
6. **Maintain what you touch.** Update affected instructions, tests, and documentation in the same change. Surface recurring friction as separately owned work rather than expanding an unrelated patch. Keep credentials and private project context out of public artifacts.

## Before declaring completion

State what changed, how it satisfies SPEC.md, what was actually verified, and any unresolved limitations or independently assigned follow-up work. Keep the report proportionate to significance.

When code, tooling, or skills are added, document their real entry points and verification commands here. Until they exist, this file remains a short contributor guide, not a speculative implementation plan.
