# Journal

Dated notes recording what was investigated and what was changed.

[`agents/agents.md`](../../agents/agents.md) sets the rule this directory exists to serve:
*when something breaks, gather before changing anything, prefer a reversible fix, ask before
anything destructive, and record what you found.* The recording is the part that gets skipped,
and it is the part that matters a year later when the same machine fails again and nobody
remembers what was tried.

## Format

One file per day, named `YYYY-MM-DD.md`. Keep entries short — the symptom, the cause, the fix,
and the evidence that the cause was the cause. Evidence is what makes a note worth reading
twice.

## What belongs here

- A failure traced to an update, and which update.
- A machine that needed a hardware profile, and what the profile had to say about it.
- A decision made from something measured, that would otherwise be lost or re-litigated.
- A trap: something that presents as one fault and is a different one underneath.
- A reversible fix that was applied instead of a destructive one, and what to undo if it
  turns out to be wrong.

## What does not

Design rationale belongs in [`docs/architecture.md`](../architecture.md). Durable constraints
and style patterns belong in [`agents/agents.md`](../../agents/agents.md). Durability is the
test: if the note is only interesting on the day it was written, it does not need to be here.

The journal is the trail, not the destination.