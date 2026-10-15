---
title: "ADR 0011 rejected options: a ui key in .capcom/config.yaml, a repo default with a user override, environment variables, creation by any command"
labels: [background]
---

- **A `ui:` key in `.capcom/config.yaml`.** One theme per repository, committed for everyone, and
  every new key raises the store's format (ADR 0009) for a setting no other command reads.
- **A repo default with a user override.** Two sources to explain for a cosmetic setting, and the
  repo half still raises the store's format.
- **Environment variables only.** Fine for one switch, awkward for per-role colour overrides, and
  invisible: there is no file to discover and edit.
- **Creating the file on any command's first run.** Agents and CI run capcom constantly; a file
  appearing in the home directory of a machine where no one asked for a UI is a surprise.
