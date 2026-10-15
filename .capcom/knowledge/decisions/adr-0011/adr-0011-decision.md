---
title: "ADR 0011 decision: <os.UserConfigDir>/capcom/config.yaml holds personal settings; only capcom ui reads it and creates it on first start"
---

`.capcom/config.yaml` is versioned with the code (ADR 0004) and shared by everyone on the
repository, so it holds what the store means: statuses, the format version (ADR 0009). A colour
theme for `capcom ui` is a person's taste, and committing one would impose it on the whole team.
capcom therefore gets a second, per-user config file.

**The file.** `<os.UserConfigDir>/capcom/config.yaml`: `~/.config/capcom/config.yaml` on Linux
(or `$XDG_CONFIG_HOME`), `~/Library/Application Support/capcom/config.yaml` on macOS,
`%AppData%\capcom\config.yaml` on Windows. It is never inside a repository and never part of the
store, so it carries no `format:` key and does not move the store's format.

**Only `capcom ui` reads it, and only `capcom ui` creates it.** The first time `capcom ui` starts
and finds no file, it writes one with every key present as a commented default, so the file
documents itself without pinning any value. Every other command neither reads nor creates it:
normal capcom use, by an agent or in CI, leaves nothing behind in the user's home. When the
directory cannot be created or written, `capcom ui` warns in its status bar and runs on the
defaults.

**It is lenient where the store is strict.** One user file is read by every capcom binary on the
machine, of whatever release each repository's agents run, so an unknown key is a warning in the
status bar, never an error; a strict reader would let a newer release's key break an older binary
for that user everywhere. Malformed YAML and a value of the wrong type are still errors, reported
with the file and line, and `capcom ui` then runs on the defaults.

**Precedence.** `NO_COLOR` beats the file; nothing in a repository can override it.
