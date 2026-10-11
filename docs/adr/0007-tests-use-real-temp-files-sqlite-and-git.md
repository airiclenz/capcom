# Tests use real temp files, SQLite and git as local stand-ins

capcom's whole job is files, a SQLite index, git and OS file locks, so faking them would test the
fakes. Tests therefore use `t.TempDir()`, a real SQLite database inside it and a real `git` in a
throwaway repository the test creates. These count as the "local stand-ins" the house coding
standards allow, and do not break their no-real-file-system rule. They stay hermetic: no network,
nothing outside the temp dir, and no user-global state (`GIT_CONFIG_GLOBAL=/dev/null`, fixed
author and committer environment). The test surface is the CLI, driven in process as
`run(args, stdin) → stdout, stderr, exit` against a temp repository, plus unit tests for the pure
parts (parsing, the ready rules, cycle detection, id rules).
