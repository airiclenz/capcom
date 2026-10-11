# Statuses are configurable, and behaviour reads only their category

A repository declares its own issue statuses in config, and each status maps to one of five fixed
categories: `todo`, `doing`, `waiting`, `done`, `dropped`. Every rule in capcom (what is ready,
when a dependency is satisfied, what prime reports, which statuses need a closing reason) reads
the category, never the status name. capcom must serve any kind of agentic work, so a fixed status
set would not do. And bd shows what goes wrong when custom names are allowed but its logic matches
the literal `open`: a custom `planned` status hid the issues that were most ready.

## Considered Options

- **A fixed status set.** Simple, but it blocks review queues, triage, in-progress tracking and
  any workflow capcom's authors did not foresee.
- **Free-form statuses with no category.** The bd failure: capcom could not know what a new
  status means, so `ready` would be wrong.
