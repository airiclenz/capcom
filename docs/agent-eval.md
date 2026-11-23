# The agent eval harness

`cmd/agenteval` measures how well a coding agent uses capcom. It runs 12 seeded capcom tasks
against any agent command, judges each task from the store files the agent left behind, counts
the capcom calls the agent made, and writes a markdown report. It is a dev tool, never a release
asset; `cmd/agenteval/doc.go` holds the package overview and the map of its files.

## Running it

Build the harness and run it from the root of a capcom checkout:

```sh
go build -o agenteval ./cmd/agenteval
./agenteval run --out eval.md -- <agent argv…>
```

Everything after `--` is the agent command. For each task the harness runs it once, in a fresh
seeded git repo as its working directory, with the task's prompt on stdin. The agent's program is
looked up on the harness's own `PATH` and made absolute before the run starts.

| Flag | Default | What it does |
| --- | --- | --- |
| `--out <file.md>` | (required) | the markdown report to write |
| `--capcom <path>` | build `./cmd/capcom` | the capcom build under test; without it, `./cmd/capcom` of the working directory is built into a temp dir first |
| `--only <id,…>` | every scenario | run only these scenario ids; repeat the flag or separate with commas |
| `--timeout <d>` | `15m` | how long each task's agent may run before it is killed and the task fails |
| `--thresholds <file>` | none | the YAML file of each model's pass threshold, a map `models: {<name>: <min passed>}`; needs `--name` |
| `--name <model>` | none | the model whose threshold in `--thresholds` the run must meet; needs `--thresholds` |

The exit code is 0 once the report is written, whichever tasks failed; 1 when the run itself
failed (the build, a seed, a file it could not write); 2 for a bad command line, such as a missing
agent command, an unknown `--only` id, or a `--name` the `--thresholds` file has no entry for,
refused before any task runs; 3 when the passed count is below the `--name` model's threshold,
once the report is written. A run that meets its threshold prints a line saying so.

```sh
./agenteval run --out eval.md --thresholds cmd/agenteval/thresholds.yaml --name qwen -- ./my-agent
```

### With Claude Code

`claude -p` reads its prompt on stdin, so it is an agent command as it stands. In print mode no one
answers a permission prompt, so allow the shell tool up front:

```sh
./agenteval run --out eval.md -- claude -p --allowedTools Bash
```

### With a local model

`agenteval agent` is a minimal agent loop for a model served behind an OpenAI-compatible endpoint
(llama.cpp's server, vLLM, Ollama and the like). Use it as the agent command:

```sh
./agenteval run --out eval.md -- \
  ./agenteval agent --base-url http://localhost:8080/v1 --model local
```

It reads the prompt on stdin and posts to `<base-url>/chat/completions` with one tool, `shell`. It
runs each command the model asks for in the working directory, kills it after 2 minutes, cuts its
combined output to 16 KiB and returns that with the exit code as the tool result. The system
message is a short brief plus the working directory's `AGENTS.md`. Every turn is printed to
stdout, which makes that the task's transcript. It stops with exit 0 on a reply that calls no tool,
and with exit 1 after `--max-turns` replies (default 50) that all still called one. When the
variable `--api-key-env` names (default `OPENAI_API_KEY`) is set, its value is sent as a bearer
token.

## The scenarios

Each scenario has a stable id: a report row, a transcript file and `--only` all name it by that id.
A prompt names ids and goals, never a capcom command: the agent learns capcom from the `AGENTS.md`
block that `capcom agents` writes into every seeded repo. The run takes them in this order.

| Id | The task | Passes when |
| --- | --- | --- |
| `next-claim-close` | work the most urgent of three Ready issues through: claim it, note progress, close it | that issue alone is closed, with a reason and at least one Log entry |
| `file-with-dep` | file an issue that waits for a seeded one, with a why and an Acceptance section | exactly one issue was added, depending on `auth-tokens`, with an `## Acceptance` section |
| `find-lock-note` | find the Note that explains the Tree lock by what it says, and record its id on an issue | a Log entry of `onboarding` names `tree-lock` |
| `resume-lost-token` | resume a claimed issue whose Lock token the agent is never given, log progress, release the lock | the lock is free, `cache-layer` has a Log entry, and the agent never broke the lock |
| `find-by-content` | find one of eight issues only by what its body says; make it priority 1 and label it urgent | that issue alone has both, and every other issue keeps its priority, has no label and stays open |
| `context-before-start` | start an issue linked to a plan, with a Note that applies, and log the name of its plan | `export-csv` is active and a Log entry of it holds the plan's basename |
| `blocked-start` | start an issue that waits for an open one, and start what blocks it when it cannot | `ci-green` is active and `deploy` is not |
| `move-under-parent` | move a top-level issue below another | `invoice-pdf`'s file sits in `billing`'s children directory and no longer at the top level |
| `rename-id` | rename an issue two others depend on, keeping references to it working | `fix-upload-timeout` exists with the alias `fix-thing`, and both dependents' deps name it |
| `add-note` | record a fact as a new Note below a seeded one | a Note below `testing` has a body that holds `GOMEMLIMIT=2GiB` |
| `reopen-with-reason` | reopen an issue closed by mistake, saying why | `login-bug` is open and has a Log entry |
| `close-children` | close the remaining children of a standing Parent, but not the Parent | every child is closed with a reason, and `v2-migration` is open and still standing |

The prompts, seeds and checks live in one file per scenario group: `cmd/agenteval/scenario_work.go`
(claiming, locking and closing), `cmd/agenteval/scenario_shape.go` (filing and reshaping the issue
tree) and `cmd/agenteval/scenario_find.go` (finding issues and Notes, and recording a Note).

## How a task is judged

The harness seeds each task's repo with the capcom build under test: a git repo with its own git
identity, a capcom store, the `AGENTS.md` block and the task's starting state, all in one commit.
Seeding runs that build outside the shim, so its calls are never counted as the agent's, and as
the child of a short-lived agenteval helper process that exits with each call, so a lock the seed
takes records that helper as its holder process, never a process the agent descends from.

A task passes or fails on what the agent left behind, never on its transcript: the scenario's check
reads the repo's `.capcom/` files, the Tree lock state under the git directory and the task's call
log. A task whose agent runs past `--timeout` is killed and fails as timed out, whatever it got
done. On Unix the agent runs in a process group of its own: a timeout sends the group SIGTERM, and
once the agent has returned, timed out or not, whatever it left running is killed, so nothing it
started outlives its workspace. That reaches past the agent's group to the shell commands an
`agenteval agent` loop started, each in a group of its own: the harness names a ledger file in
`AGENTEVAL_PROCESS_GROUPS` (outside the repo), the loop lists each command's group there, and the
harness kills every listed group still running, even when the loop ignored SIGTERM. Another agent,
such as `claude -p`, gets the variable too but lists nothing, so only its own group is killed. Ctrl-C (or SIGTERM) stops the agent the same way and ends the run
without judging that task or writing a report; a second Ctrl-C kills the harness at once. An agent that ends with a non-zero exit, or is killed, gets a line saying so at the end of
its transcript; its exit status never judges the task.

The counts come from a call shim. The harness copies its own executable into the task's
workspace as `bin/capcom` (`capcom.exe` on Windows) and puts that directory first on the agent's
`PATH`. Started under the name `capcom`, the executable runs the build under test with the same
arguments and standard streams, exits with its exit code, and appends one JSON line per call to the
call log: `{"args":[…],"exit":n,"ms":n}`. So every `capcom` the agent runs by name is counted; one
run by an absolute path to another binary is not. From the call log the report counts:

- **Calls**: every capcom call the agent made.
- **Failed**: the calls that exited non-zero, whatever the code.
- **Repeated failures**: the failed calls whose arguments and exit code match an earlier failed
  call of the same task: the agent retrying what it was already told does not work.

## The report and the transcripts

The report goes to `--out`. Its header names the date, the capcom version under test and the agent
command; one table follows, a row per task in the run's order with its Result (`pass` or `fail`),
Calls, Failed, Repeated failures, Time and, for a failed task, Why. A totals row counts the passed
tasks and sums the counts and the time. A rerun to the same `--out` replaces the report.

Beside the report, a directory named after it without its extension, plus `.transcripts`
(`eval.transcripts/` for `--out eval.md`), holds two files per task, both started afresh by each
run:

- `<id>.log`: the agent's stdout and stderr, ending with how it ended when that was an error.
- `<id>.calls.jsonl`: the task's call log.

Each task's seeded repo lives in a temp dir that is removed when the task ends, so the transcript
and the call log are what is left to read when a task fails.

## In CI

`.github/workflows/agent-eval.yml` runs the eval every Monday at 04:00 UTC and on demand from the
Actions tab (`workflow_dispatch`). It lives apart from `ci.yml`, so a failing eval never blocks a
release: the release gate reads `ci.yml`'s runs only. Runs never overlap; one waits for the one
before it to finish.

It runs one job per model the repository variable `AGENT_EVAL_MODELS` lists, and one for Claude
Code when the secret `ANTHROPIC_API_KEY` is set. With neither, it runs no eval job and passes.
`AGENT_EVAL_MODELS` is a JSON list of `{name, base_url, model, key_secret}`:

```json
[
  {"name": "qwen", "base_url": "https://llm.example.com/v1", "model": "qwen3-coder", "key_secret": "QWEN_API_KEY"}
]
```

- `name` labels the job, the report and the artifact, and is the model's key in the thresholds
  file. It is letters, digits, `.`, `_` and `-` only, and no two entries share one.
- `base_url` and `model` go to `agenteval agent` as `--base-url` and `--model`.
- `key_secret` names the repository secret that holds the endpoint's API key; the job hands its
  value to `agenteval agent` as the bearer token.

A list that is not valid JSON, or an entry missing a field, fails the run before any eval job
starts. The Claude Code job installs the current `@anthropic-ai/claude-code` and runs
`claude -p --allowedTools Bash` with `ANTHROPIC_API_KEY`, under the name `claude`.

Every job runs with `--thresholds cmd/agenteval/thresholds.yaml --name <name>`, so each model in
`AGENT_EVAL_MODELS`, and `claude` when its job runs, needs an entry in that file: the fewest tasks
its run must pass. A job whose model passes fewer goes red (exit 3); a model with no entry fails
the job before any task runs (exit 2). Add the entry in the same change that adds the model, and
raise it as the model gets better. The file starts with no entries; for example:

```yaml
models:
  qwen: 9
  claude: 11
```

Each job uploads its report and transcripts (the `eval/` directory) as the artifact
`agent-eval-<name>`, also when the run failed, and writes the report table's heading and Total row
to the run's summary. Reports from CI are never committed: to keep one, download the artifact and
copy the report into `docs/reviews/`.
