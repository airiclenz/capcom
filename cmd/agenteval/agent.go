package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// The agent verb's defaults: the environment variable holding the API key, and how many replies
// the loop asks for before it gives up on the model finishing.
const (
	defaultAPIKeyEnv = "OPENAI_API_KEY"
	defaultMaxTurns  = 50
)

// The bounds on one shell command: how long it may run, how long its output pipe is waited on
// once it has exited or been killed, and how much of its output reaches the model.
const (
	shellTimeout     = 2 * time.Minute
	shellWaitDelay   = 5 * time.Second
	shellOutputLimit = 16 * 1024
)

// errorBodyLimit is how much of a non-200 reply's body an error quotes.
const errorBodyLimit = 1024

// chatCompletionsPath is the endpoint the loop posts to, under --base-url.
const chatCompletionsPath = "/chat/completions"

// agentsFileName is the agent guide the loop adds to its system message when the working
// directory has one.
const agentsFileName = "AGENTS.md"

// The tool the loop offers the model: its one function and that function's one argument.
const (
	shellToolName     = "shell"
	shellToolArgument = "command"
)

// systemBrief opens every system message, ahead of the working directory's AGENTS.md.
const systemBrief = "You are a coding agent working in a git repository, which is your working " +
	"directory. You have one tool, shell, which runs a command there and returns its combined " +
	"output and exit code. Do the task you are given with it. When the task is done, reply with a " +
	"short summary and no tool call."

// The roles of the chat messages the loop sends and receives.
const (
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"
)

// agentOptions is the agent verb's command line.
type agentOptions struct {
	baseURL   string
	model     string
	apiKeyEnv string
	maxTurns  int
}

// agentLoop is one conversation with an OpenAI-compatible endpoint: where it posts, how it runs
// the shell commands the model asks for, and where it prints each turn.
type agentLoop struct {
	client         *http.Client
	baseURL        string
	model          string
	apiKey         string
	maxTurns       int
	dir            string
	commandTimeout time.Duration
	out            io.Writer
}

// chatMessage is one message of the conversation, in the chat completions wire format.
type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// toolCall is one function call the model asked for; its arguments are a JSON object as text.
type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

// functionCall names the function a tool call calls and carries its arguments.
type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// chatRequest is the body the loop posts.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []toolSpec    `json:"tools"`
}

// toolSpec declares one tool to the model.
type toolSpec struct {
	Type     string       `json:"type"`
	Function functionSpec `json:"function"`
}

// functionSpec is a tool's function: its name, what it does and its JSON Schema parameters.
type functionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// chatResponse is the part of a reply the loop reads: the first choice's message.
type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// shellArguments is the JSON object a shell tool call carries.
type shellArguments struct {
	Command string `json:"command"`
}

// shellTools is the tool list every request carries: the shell function alone.
var shellTools = []toolSpec{{
	Type: "function",
	Function: functionSpec{
		Name:        shellToolName,
		Description: "Run a shell command in the working directory and return its output and exit code.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				shellToolArgument: map[string]any{"type": "string", "description": "the command to run"},
			},
			"required": []string{shellToolArgument},
		},
	},
}}

// newAgentCommand builds the agent verb: a minimal agent loop, used as the agent command of a run
// when the model under evaluation is served behind an OpenAI-compatible endpoint.
func newAgentCommand() *cobra.Command {
	var options agentOptions
	cmd := &cobra.Command{
		Use:   "agent --base-url <url> --model <name> [flags]",
		Short: "A minimal agent loop for an OpenAI-compatible endpoint",
		Long: "agent reads a prompt on stdin and talks to <base-url>/chat/completions with one tool,\n" +
			"shell, whose commands it runs in the working directory, each for at most 2 minutes,\n" +
			"returning their combined output (cut to 16 KiB) and exit code. The system message is\n" +
			"a short brief plus AGENTS.md from the working directory when there is one. Every turn\n" +
			"is printed to stdout. It exits 0 on a reply with no tool call, and 1 after --max-turns\n" +
			"replies that all still called a tool. The API key, when its variable is set, is sent\n" +
			"as a bearer token.",
		Example: "  agenteval run --out eval.md -- agenteval agent --base-url http://localhost:8080/v1 --model local",
		Args:    cobra.NoArgs,
		PreRunE: func(*cobra.Command, []string) error {
			return checkAgentOptions(options)
		},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			loop := agentLoop{
				client:         &http.Client{},
				baseURL:        strings.TrimSuffix(options.baseURL, "/"),
				model:          options.model,
				apiKey:         os.Getenv(options.apiKeyEnv),
				maxTurns:       options.maxTurns,
				dir:            dir,
				commandTimeout: shellTimeout,
				out:            cmd.OutOrStdout(),
			}
			return loop.run(cmd.Context(), cmd.InOrStdin())
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.baseURL, "base-url", "", "the endpoint's base URL, e.g. http://localhost:8080/v1 (required)")
	flags.StringVar(&options.model, "model", "", "the model name to request (required)")
	flags.StringVar(&options.apiKeyEnv, "api-key-env", defaultAPIKeyEnv, "the environment variable holding the API key")
	flags.IntVar(&options.maxTurns, "max-turns", defaultMaxTurns, "how many replies to ask for before giving up")
	for _, name := range []string{"base-url", "model"} {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(err) // the flag is declared just above; failing here is a defect of this program
		}
	}
	return cmd
}

// checkAgentOptions refuses a --max-turns that is not positive.
func checkAgentOptions(options agentOptions) error {
	if options.maxTurns <= 0 {
		return fmt.Errorf("--max-turns must be positive, got %d", options.maxTurns)
	}
	return nil
}

// run reads the prompt from stdin and converses until a reply calls no tool, which ends the loop
// without error, or until maxTurns replies have all called one, which is an error saying so.
// Every turn is printed to out.
func (l agentLoop) run(ctx context.Context, stdin io.Reader) error {
	prompt, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("read the prompt from stdin: %w", err)
	}
	system, err := l.systemMessage()
	if err != nil {
		return err
	}
	messages := []chatMessage{
		{Role: roleSystem, Content: system},
		{Role: roleUser, Content: string(prompt)},
	}
	fmt.Fprintf(l.out, "== prompt\n%s\n", strings.TrimRight(string(prompt), "\n"))

	for turn := 1; turn <= l.maxTurns; turn++ {
		reply, err := l.post(ctx, messages)
		if err != nil {
			return fmt.Errorf("turn %d: %w", turn, err)
		}
		fmt.Fprintf(l.out, "\n== turn %d\n", turn)
		if reply.Content != "" {
			fmt.Fprintf(l.out, "%s\n", strings.TrimRight(reply.Content, "\n"))
		}
		messages = append(messages, reply)
		if len(reply.ToolCalls) == 0 {
			return nil
		}
		for _, call := range reply.ToolCalls {
			result := l.runToolCall(ctx, call)
			fmt.Fprintf(l.out, "%s\n", result)
			messages = append(messages, chatMessage{Role: roleTool, Content: result, ToolCallID: call.ID})
		}
	}
	return fmt.Errorf("stopped after --max-turns %d replies, every one still calling a tool", l.maxTurns)
}

// systemMessage is the fixed brief, followed by the working directory's AGENTS.md when it has one.
func (l agentLoop) systemMessage() (string, error) {
	path := filepath.Join(l.dir, agentsFileName)
	guide, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return systemBrief, nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return systemBrief + "\n\nThe repository's " + agentsFileName + " follows.\n\n" + string(guide), nil
}

// post sends the conversation so far and returns the reply's message. A reply that is not 200, or
// carries no choice, is an error naming what came back.
func (l agentLoop) post(ctx context.Context, messages []chatMessage) (chatMessage, error) {
	body, err := json.Marshal(chatRequest{Model: l.model, Messages: messages, Tools: shellTools})
	if err != nil {
		return chatMessage{}, err
	}
	url := l.baseURL + chatCompletionsPath
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return chatMessage{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if l.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+l.apiKey)
	}
	response, err := l.client.Do(request)
	if err != nil {
		return chatMessage{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		excerpt, _ := io.ReadAll(io.LimitReader(response.Body, errorBodyLimit)) // the status alone is the error
		return chatMessage{}, fmt.Errorf("POST %s: %s: %s", url, response.Status, strings.TrimSpace(string(excerpt)))
	}
	var reply chatResponse
	if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
		return chatMessage{}, fmt.Errorf("POST %s: read the reply: %w", url, err)
	}
	if len(reply.Choices) == 0 {
		return chatMessage{}, fmt.Errorf("POST %s: the reply has no choices", url)
	}
	message := reply.Choices[0].Message
	message.Role = roleAssistant
	return message, nil
}

// runToolCall runs one tool call and returns the text the model gets back for it. A call the loop
// cannot run, an unknown tool or arguments that are not {"command": string}, gets a result saying
// so, so the model can correct itself.
func (l agentLoop) runToolCall(ctx context.Context, call toolCall) string {
	if call.Function.Name != shellToolName {
		return fmt.Sprintf("agenteval: no tool %q; the one tool is %s", call.Function.Name, shellToolName)
	}
	var arguments shellArguments
	if err := json.Unmarshal([]byte(call.Function.Arguments), &arguments); err != nil || arguments.Command == "" {
		return fmt.Sprintf("agenteval: the %s arguments must be {\"%s\": string}, got %s",
			shellToolName, shellToolArgument, call.Function.Arguments)
	}
	return "$ " + arguments.Command + "\n" + runShellCommand(ctx, l.dir, arguments.Command, l.commandTimeout)
}

// runShellCommand runs command through the platform's shell in dir with empty stdin, and returns
// its combined output, cut to shellOutputLimit, followed by a line giving its exit code. At timeout
// the command is killed, on Unix with every process it started, and its output pipe is waited on
// for at most shellWaitDelay more, so a child left holding the pipe cannot stall the turn.
func runShellCommand(ctx context.Context, dir, command string, timeout time.Duration) string {
	timed, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	argv := shellArgv(command)
	process := exec.CommandContext(timed, argv[0], argv[1:]...)
	process.Dir = dir
	output := &cappedBuffer{limit: shellOutputLimit}
	process.Stdout, process.Stderr = output, output
	process.WaitDelay = shellWaitDelay
	isolateProcessGroup(process)

	err := process.Run()

	var status string
	switch {
	case errors.Is(timed.Err(), context.DeadlineExceeded):
		status = fmt.Sprintf("[killed after the %s timeout]", timeout)
	case process.ProcessState != nil:
		status = fmt.Sprintf("[exit code %d]", process.ProcessState.ExitCode())
	default:
		status = fmt.Sprintf("[could not run: %v]", err)
	}
	return output.String() + status
}

// cappedBuffer keeps the first limit bytes written to it and counts the rest, so a command's
// output costs at most limit bytes of memory however much it prints.
type cappedBuffer struct {
	kept    bytes.Buffer
	limit   int
	dropped int
}

// Write keeps what fits under the limit and reports every byte written, so the writer never fails.
func (b *cappedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.kept.Len()
	if room >= len(p) {
		return b.kept.Write(p)
	}
	b.kept.Write(p[:max(room, 0)])
	b.dropped += len(p) - max(room, 0)
	return len(p), nil
}

// String is the kept output, ending in a newline when there is any, and a line saying how much
// was cut when anything was.
func (b *cappedBuffer) String() string {
	text := b.kept.String()
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if b.dropped > 0 {
		text += fmt.Sprintf("[output cut at %d bytes; %d more not shown]\n", b.limit, b.dropped)
	}
	return text
}
