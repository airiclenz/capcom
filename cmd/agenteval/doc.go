// Command agenteval is capcom's agent evaluation dev tool: it runs seeded capcom tasks against
// an agent command, judges each from the store files the agent left behind, counts the capcom
// calls the agent made, and writes a markdown report.
//
//	agenteval run --out eval.md [--capcom <path>] [--only <id,…>] [--timeout 15m] -- <agent argv…>
//
// run gives each scenario a fresh git repo, seeded by the capcom build under test, and runs the
// agent command there with the task's prompt on stdin. Each agent's output goes to
// <out stem>.transcripts/<id>.log and its capcom calls to <id>.calls.jsonl beside it.
//
// Started under the basename capcom (capcom.exe on Windows), the same executable is the call
// shim instead: it runs the capcom build under test with its own arguments and standard
// streams, exits with that build's exit code, and appends one JSON line per call to the call
// log. The harness puts a copy of itself under that name first on the agent's PATH, which is
// how every capcom call the agent makes is counted.
//
//	AGENTEVAL_CAPCOM=/path/to/capcom AGENTEVAL_CALL_LOG=/path/to/calls.jsonl capcom <args>
//
// Each call log line reads {"args":[…],"exit":n,"ms":n}: the arguments, the exit code and the
// wall time in milliseconds.
//
//	agenteval agent --base-url <url> --model <name> [--api-key-env OPENAI_API_KEY] [--max-turns 50]
//
// agent is a minimal agent loop for a model served behind an OpenAI-compatible endpoint, meant
// as the agent command of a run. It reads the prompt on stdin, posts to <base-url>/chat/completions
// with one tool, shell, runs each command the model asks for in the working directory (killed at
// 2 minutes; output cut to 16 KiB, with the exit code) and returns the output as the tool result.
// It prints every turn to stdout, which makes that the transcript, and stops on a reply with no
// tool call (exit 0) or after --max-turns replies (exit 1). The system message is a short brief
// plus the working directory's AGENTS.md when there is one.
//
// It is a dev tool, never a release asset: dist does not ship it. Unlike cmd/release it may
// import capcom's internal packages, so it reads store files with capcom's own parsers.
//
// # Files
//
// Every non-test file of the package, with its responsibility. TestDocFileMap_MatchesDirectory
// fails when a file is missing from this map or the map names a file that does not exist.
//
//   - `agent.go` — the agent verb: a minimal chat completions loop with one shell tool
//   - `agent_other.go` — the agent's shell and command kill outside Unix: cmd /C, the shell alone
//   - `agent_unix.go` — the agent's shell and command kill on Unix: sh -c, the whole process group
//   - `doc.go` — the package comment and this file map
//   - `main.go` — main, the shim-or-command dispatch, the root command and the exit codes
//   - `report.go` — renders a run's task results as the markdown report
//   - `run.go` — the run verb: resolves the build and the agent, runs each task, writes the report
//   - `scenario.go` — the scenario type, the scenarios a run runs, and each one's seed and check
//   - `score.go` — reads a task's call log and scores it to calls, failed and repeated failures
//   - `shim.go` — the capcom call shim: runs the build under test and appends each call to the call log
//   - `store.go` — reads what the agent left: issue and Note files, Log entries, the Tree lock
//   - `workspace.go` — seeds a task's throwaway git repo and runs the agent in it behind the shim
package main
