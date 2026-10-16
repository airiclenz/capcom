---
title: "ADR 0008 rejected options: a hidden subcommand, publishing from CI, a Makefile, granular publish verbs and one agent host"
labels: [background]
---

- **A hidden `capcom release` subcommand.** Puts maintainer-only code into every user's binary for
  nothing; the people who install capcom never release it.
- **Publishing from CI with stored secrets.** Needs a stored personal token for the tap, the
  bucket and winget, a second trigger the CI-made tag cannot fire, and notes that an agent writes.
- **A Makefile, bash scripts and `-ldflags` version stamping, as apogee had before its port.**
  `make`, `zip` and bash are missing on Windows, and the build info already carries the commit.
  A root `Makefile` for local builds came later (`adr-0008-amend-makefile`); no release step
  calls it.
- **Granular publish verbs that the skill sequences and resumes.** Resume logic written in agent
  prose is exactly the part that should be code; one converging `publish` replaces it.
- **One agent host only.** Pinning the repository's release to one vendor's agent defeats putting
  it in the repository.
