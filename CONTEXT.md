# capcom

capcom is a cross-platform issue tracker for code repositories, built for coding agents first and
still easy for people to read. On top of the issues it records who holds the repository and keeps
the project knowledge agents need, all as plain files under git.

## Language

### Structure

**Tree**:
The hierarchy that both **Issues** and **Knowledge** are organised in. Every node has at most one
parent and any number of children.
_Avoid_: folder structure, taxonomy

**Id**:
The short, permanent, human-meaningful name of one node in a **Tree**. It says in a few words what
the node is about and is unique across the whole **Tree**.
_Avoid_: key, hash, number

**Alias**:
A former **Id** that still resolves to its node, kept after a rename or an import so old citations
stay valid. A node has any number of **Aliases**; no **Id** may reuse one.
_Avoid_: old id, redirect

**Title**:
The one-line statement every node carries, written to be read on its own, so a reader can browse a
**Tree** without opening the nodes. For a **Note** the **Title** is the gist of the fact.
_Avoid_: summary, name, heading

**Parent**:
The node directly above another in a **Tree**. An **Issue** with children is a **Parent**; there is
no separate epic kind.
_Avoid_: epic, container

**Format version**:
The integer a repository's capcom files declare in the `format:` key of `config.yaml`, saying which
file format they are written in; a missing key means 1. A binary refuses a store whose **Format
version** is above the highest it reads, and adding any key to the files raises it.
_Avoid_: schema version, store version, config version

### Work

**Issue**:
One unit of tracked work or one known defect in the repository.
_Avoid_: bead, ticket, task, card

**Status**:
The workflow state of an **Issue**, named by the repository (`open`, `review`, `parked`, ...).
Every **Status** belongs to exactly one **Status category**.
_Avoid_: state

**Status category**:
One of the five fixed meanings a **Status** can have: `todo`, `doing`, `waiting`, `done`,
`dropped`. capcom's behaviour depends only on the category.
_Avoid_: status type, phase

**Ready**:
An **Issue** whose **Status** is in the `todo` category, whose **Dependencies** are all `done` or
`dropped`, which has no open children and which has no `waiting` or `dropped` ancestor.
_Avoid_: available, unblocked

**Closed**:
An **Issue** whose **Status** is in the `done` or `dropped` category. Closing always records a
reason.
_Avoid_: resolved, finished

**Log**:
The dated, append-only record of progress and remarks kept inside an **Issue**.
_Avoid_: notes, comments, history

**Plan**:
A design document kept outside capcom that describes how one or more **Issues** will be done.
An **Issue** linked to a **Plan** is **Planned**.
_Avoid_: spec, design doc

**Dependency**:
A directed edge between two **Issues**: the dependent **Issue** cannot be worked until the one it
depends on is closed.
_Avoid_: link, relation

**Elsewhere**:
A change to an **Issue** that exists on another local branch, worktree or remote-tracking branch
but not on the current one, reported beside a result and never merged into it.
_Avoid_: remote state, pending

### Coordination

**Tree lock**:
The advisory claim one agent session holds on one working tree (one checkout or worktree) while it
changes it, naming the holder, the purpose and the time it was taken. Each worktree of a clone has
its own **Tree lock**, and the locks of all worktrees can be seen together.
_Avoid_: repo lock, lease, claim, mutex

**Lock token**:
The secret a holder receives when it takes a **Tree lock** and presents to renew, release or resume
it. Whoever holds the token is the holder.
_Avoid_: session id, key

**Stale**:
A **Tree lock** whose holder has not renewed it within the configured time. A stale lock is still
held until its holder resumes it, someone takes it over on purpose, or it is broken.
_Avoid_: expired, dead, abandoned

### Knowledge

**Knowledge**:
The committed, shared body of facts about the repository that agents consult on demand:
conventions, gotchas and the reasons behind them. It holds **Notes**.
_Avoid_: memory, docs, wiki

**Note**:
One self-contained fact or rule in **Knowledge**.
_Avoid_: entry, memory, fact

**Ref**:
A file, directory or symbol in the repository that a **Note** depends on. A **Note** with a ref
that no longer exists is suspect until someone checks it. ADRs and the glossary are referenced,
never copied.
_Avoid_: link, anchor, dependency (that word belongs to **Issues**)

**Label**:
An optional cross-cutting marker on an **Issue** or a **Note**, for groupings the **Tree** and the
**Status** cannot express (`owner-run`, `invariant`).
_Avoid_: tag, category

### Agent interface

**Briefing**:
What capcom tells an agent at the start of a session or after its context is compacted: the
**Ready** work, the **Tree locks**, anything **Elsewhere** and the **Notes** whose **Refs** are gone.
_Avoid_: prime output, status, summary

**Managed block**:
The marked section of a repository's agent guide (AGENTS.md) that capcom writes and keeps current:
the pointer to capcom, the core commands and the top level of the **Knowledge** **Tree**. Text
outside it is never touched.
_Avoid_: generated section, injected instructions

## Flagged ambiguities

- "Memory" means an agent's own private, uncommitted store (for example Claude Code's auto-memory),
  never capcom's. A fact about the repository belongs in **Knowledge**; a fact about one machine or
  one user belongs in an agent's memory.
