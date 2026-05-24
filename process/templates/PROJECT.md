---
project: <name>
date: YYYY-MM-DD
status: needs-attention     # needs-attention | ready | in-progress | shipped | archived
---

# {{Project name}}

> This file tells agents and humans what *this project* is — independent of the process that governs it. Without it, agents anchor on the process repo's own framing (Nygard/Beck/Poppendieck) instead of the project's domain.
>
> **Flip `status` from `needs-attention` to `ready` once every section below is filled in with real content. All `process-*` skills refuse to run while `status: needs-attention`.**

## One line

What this is, in one sentence a stranger could understand.

## What it does

One paragraph. The job to be done, who uses it, why now.

## Domain

The problem space. Used by `process-personas` to pick domain experts. Be specific — "sqlite tooling and schema diff" not "developer tools"; "double-entry bookkeeping" not "fintech".

## Primary users

Real people, teams, or systems by name. If you can't name them, say so.

## Non-negotiables

Constraints every brief in this project inherits: language, platform, compatibility, performance, regulatory, dependency restrictions, deploy targets. Short list; only the things that *really* don't move.

## Out of scope (for this project)

Things this project deliberately doesn't try to be. Cuts ambient scope before ambient scope cuts you.

## Documentation

How this project communicates with its users. `process-write` and the docs concern hook off this.

- **Audience(s):** end users, contributors, operators, agents — name them.
- **Shape (Diátaxis):** which of tutorial / how-to / reference / explanation this project needs, and where each lives. `[none]` is a legitimate answer for an internal tool.
- **Home:** `docs/` in repo, `process/MANUAL.md`, a hosted site — name it.

## Notes

Anything else an agent should know before its first brief. Optional.
