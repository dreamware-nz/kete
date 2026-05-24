---
id: NNN-slug
date: YYYY-MM-DD
status: draft         # draft | accepted | rejected
brief: NNN-slug
adrs: []              # ADRs this design cites or that resolve its open questions
plan: null            # filled when a plan references this design
---

# {{Title}}

## Principles

What this design optimises for, in this brief, in this context. Honest and specific. No generic platitudes. Five at most; three is better.

## Considerations

What bounds the decision space without being a decision. Prior art, references, competitive examples, accessibility / performance / security / compatibility / cost constraints that matter for *this* thing. Cite real sources where useful.

## Integrated picture

The thing itself, concrete enough that a reader can recognise what gets built. Use whichever apply; skip the rest:

- **User journey** — what a person does, step by step, to get value.
- **System sketch** — boxes, arrows, data flow. ASCII diagram inline or link to a source-controlled diagram file in this design's folder.
- **Surface** — API shape, CLI shape, screen-by-screen wireframes, message format. Link out to wireframes / mocks where they live.
- **Data shapes** — the nouns the system traffics in, with their fields.
- **Behaviour over time** — sequence diagrams, state machines, lifecycle.

Don't pad. A design doc for a CLI is mostly *surface* and *user journey*; a design doc for a daemon is mostly *system sketch* and *behaviour*.

## Trade-offs

Honest analyses that *don't* fork into a single decision. "We lose X to get Y; we accept that because Z." If a trade-off does fork into a yes/no choice, that's an ADR — link it from `adrs:` instead.

## Open design questions

Things that still need to be decided. Same role as the brief's open questions, but design-shaped. Feed the ADR pipeline.

## Revisions

Append-only after acceptance. Earlier accepted shape is preserved above; revisions log changes here.

- `YYYY-MM-DD` — what changed and why.
