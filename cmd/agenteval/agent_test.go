package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// scriptedReply is one reply a scripted endpoint sends: a status and a JSON body.
type scriptedReply struct {
	status int
	body   any
}

// scriptedEndpoint is a fake chat completions endpoint: it answers each request with the next of
// its replies, repeating the last one once they run out, and records every request it was sent.
type scriptedEndpoint struct {
	server   *httptest.Server
	mu       sync.Mutex
	replies  []scriptedReply
	requests []chatRequest
	headers  []http.Header
}

// newScriptedEndpoint starts an endpoint answering with replies; it is closed when the test ends.
func newScriptedEndpoint(t *testing.T, replies ...scriptedReply) *scriptedEndpoint {
	t.Helper()
	endpoint := &scriptedEndpoint{replies: replies}
	endpoint.server = httptest.NewServer(http.HandlerFunc(endpoint.serve))
	t.Cleanup(endpoint.server.Close)
	return endpoint
}

func (e *scriptedEndpoint) serve(writer http.ResponseWriter, request *http.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if request.URL.Path != chatCompletionsPath {
		http.NotFound(writer, request)
		return
	}
	var body chatRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	reply := e.replies[min(len(e.requests), len(e.replies)-1)]
	e.requests = append(e.requests, body)
	e.headers = append(e.headers, request.Header.Clone())
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(reply.status)
	if err := json.NewEncoder(writer).Encode(reply.body); err != nil {
		panic(err) // a test reply that does not encode is a defect of the test
	}
}

// textReply is a 200 reply whose message is content with no tool call.
func textReply(content string) scriptedReply {
	return scriptedReply{status: http.StatusOK, body: map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}},
	}}
}

// shellReply is a 200 reply whose message calls the shell tool with command, under call id id.
func shellReply(id, command string) scriptedReply {
	arguments, err := json.Marshal(shellArguments{Command: command})
	if err != nil {
		panic(err)
	}
	return toolReply(id, shellToolName, string(arguments))
}

// toolReply is a 200 reply whose message calls the tool name with the arguments text given.
func toolReply(id, name, arguments string) scriptedReply {
	return scriptedReply{status: http.StatusOK, body: map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{
			"role":    "assistant",
			"content": nil,
			"tool_calls": []any{map[string]any{
				"id":       id,
				"type":     "function",
				"function": map[string]any{"name": name, "arguments": arguments},
			}},
		}}},
	}}
}

// newTestAgentLoop is a loop against endpoint, in dir, printing to out.
func newTestAgentLoop(endpoint *scriptedEndpoint, dir string, out *bytes.Buffer) agentLoop {
	return agentLoop{
		client:         endpoint.server.Client(),
		baseURL:        endpoint.server.URL,
		model:          "test-model",
		maxTurns:       defaultMaxTurns,
		dir:            dir,
		commandTimeout: shellTimeout,
		out:            out,
	}
}

// lastMessage is the last message of a recorded request.
func lastMessage(t *testing.T, request chatRequest) chatMessage {
	t.Helper()
	if len(request.Messages) == 0 {
		t.Fatal("the request carries no messages")
	}
	return request.Messages[len(request.Messages)-1]
}

func TestAgentLoop_AShellCallsOutputReachesTheNextRequest(t *testing.T) {
	endpoint := newScriptedEndpoint(t, shellReply("call-1", "echo hello-from-shell"), textReply("done"))
	var out bytes.Buffer
	loop := newTestAgentLoop(endpoint, t.TempDir(), &out)

	err := loop.run(context.Background(), strings.NewReader("say hello"))

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(endpoint.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(endpoint.requests))
	}
	first := endpoint.requests[0]
	if first.Model != "test-model" || len(first.Tools) != 1 || first.Tools[0].Function.Name != shellToolName {
		t.Errorf("first request: model %q, tools %+v; want test-model and the shell tool", first.Model, first.Tools)
	}
	if prompt := lastMessage(t, first); prompt.Role != roleUser || prompt.Content != "say hello" {
		t.Errorf("first request's last message = %+v, want the prompt", prompt)
	}
	result := lastMessage(t, endpoint.requests[1])
	if result.Role != roleTool || result.ToolCallID != "call-1" {
		t.Errorf("second request's last message = %+v, want the tool result for call-1", result)
	}
	if !strings.Contains(result.Content, "hello-from-shell") || !strings.Contains(result.Content, "[exit code 0]") {
		t.Errorf("tool result = %q, want the echo output and exit code 0", result.Content)
	}
	for _, want := range []string{"say hello", "== turn 1", "hello-from-shell", "== turn 2", "done"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("transcript is missing %q:\n%s", want, out.String())
		}
	}
}

func TestAgentLoop_AReplyWithNoToolCallEndsTheLoop(t *testing.T) {
	endpoint := newScriptedEndpoint(t, textReply("nothing to do"))
	var out bytes.Buffer
	loop := newTestAgentLoop(endpoint, t.TempDir(), &out)

	err := loop.run(context.Background(), strings.NewReader("prompt"))

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(endpoint.requests) != 1 {
		t.Errorf("requests = %d, want 1", len(endpoint.requests))
	}
}

func TestAgentLoop_AnUnknownToolOrBadArgumentsAreToldToTheModel(t *testing.T) {
	tests := []struct {
		name  string
		reply scriptedReply
		want  string
	}{
		{name: "an unknown tool", reply: toolReply("c", "python", `{"code":"1"}`), want: `no tool "python"`},
		{name: "arguments that are not JSON", reply: toolReply("c", shellToolName, "echo hi"), want: "arguments must be"},
		{name: "no command", reply: toolReply("c", shellToolName, `{}`), want: "arguments must be"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint := newScriptedEndpoint(t, test.reply, textReply("ok"))
			var out bytes.Buffer
			loop := newTestAgentLoop(endpoint, t.TempDir(), &out)

			if err := loop.run(context.Background(), strings.NewReader("prompt")); err != nil {
				t.Fatalf("run: %v", err)
			}

			if result := lastMessage(t, endpoint.requests[1]); !strings.Contains(result.Content, test.want) {
				t.Errorf("tool result = %q, want it to contain %q", result.Content, test.want)
			}
		})
	}
}

func TestAgentLoop_ANon200ReplyNamesTheStatus(t *testing.T) {
	endpoint := newScriptedEndpoint(t, scriptedReply{status: http.StatusServiceUnavailable, body: "model loading"})
	var out bytes.Buffer
	loop := newTestAgentLoop(endpoint, t.TempDir(), &out)

	err := loop.run(context.Background(), strings.NewReader("prompt"))

	if err == nil {
		t.Fatal("run succeeded on a 503 reply")
	}
	for _, want := range []string{"503 Service Unavailable", "model loading"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
}

func TestAgentLoop_AReplyWithNoChoicesIsAnError(t *testing.T) {
	endpoint := newScriptedEndpoint(t, scriptedReply{status: http.StatusOK, body: map[string]any{"choices": []any{}}})
	var out bytes.Buffer
	loop := newTestAgentLoop(endpoint, t.TempDir(), &out)

	err := loop.run(context.Background(), strings.NewReader("prompt"))

	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error = %v, want one saying the reply has no choices", err)
	}
}

func TestAgentLoop_AgentsFileLandsInTheSystemMessage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, agentsFileName), []byte("# guide\nuse the tracker\n"), runFileMode); err != nil {
		t.Fatal(err)
	}
	endpoint := newScriptedEndpoint(t, textReply("done"))
	var out bytes.Buffer
	loop := newTestAgentLoop(endpoint, dir, &out)

	if err := loop.run(context.Background(), strings.NewReader("prompt")); err != nil {
		t.Fatalf("run: %v", err)
	}

	system := endpoint.requests[0].Messages[0]
	if system.Role != roleSystem || !strings.HasPrefix(system.Content, systemBrief) {
		t.Errorf("first message = %+v, want the system brief", system)
	}
	if !strings.Contains(system.Content, "use the tracker") {
		t.Errorf("system message is missing AGENTS.md:\n%s", system.Content)
	}
}

func TestAgentLoop_WithoutAgentsFileTheSystemMessageIsTheBrief(t *testing.T) {
	endpoint := newScriptedEndpoint(t, textReply("done"))
	var out bytes.Buffer
	loop := newTestAgentLoop(endpoint, t.TempDir(), &out)

	if err := loop.run(context.Background(), strings.NewReader("prompt")); err != nil {
		t.Fatalf("run: %v", err)
	}

	if system := endpoint.requests[0].Messages[0]; system.Content != systemBrief {
		t.Errorf("system message = %q, want the brief alone", system.Content)
	}
}

func TestAgentLoop_TheAPIKeyIsSentOnlyWhenSet(t *testing.T) {
	for _, key := range []string{"", "secret-key"} {
		endpoint := newScriptedEndpoint(t, textReply("done"))
		var out bytes.Buffer
		loop := newTestAgentLoop(endpoint, t.TempDir(), &out)
		loop.apiKey = key

		if err := loop.run(context.Background(), strings.NewReader("prompt")); err != nil {
			t.Fatalf("run: %v", err)
		}

		got := endpoint.headers[0].Get("Authorization")
		want := ""
		if key != "" {
			want = "Bearer " + key
		}
		if got != want {
			t.Errorf("key %q: Authorization = %q, want %q", key, got, want)
		}
	}
}

func TestAgentCommand_MaxTurnsExits1SayingSo(t *testing.T) {
	endpoint := newScriptedEndpoint(t, shellReply("c", "echo again"))

	exitCode, stdout, errorText := executeAgentCommand(t, "prompt",
		"agent", "--base-url", endpoint.server.URL+"/", "--model", "m", "--max-turns", "2")

	if exitCode != exitRunFailed {
		t.Errorf("exit = %d, want %d: %s", exitCode, exitRunFailed, errorText)
	}
	if !strings.Contains(errorText, "--max-turns 2") {
		t.Errorf("error %q does not say the loop stopped at --max-turns 2", errorText)
	}
	if len(endpoint.requests) != 2 {
		t.Errorf("requests = %d, want 2", len(endpoint.requests))
	}
	if !strings.Contains(stdout, "== turn 2") {
		t.Errorf("stdout is not the transcript:\n%s", stdout)
	}
}

func TestAgentCommand_ANoToolReplyExits0(t *testing.T) {
	endpoint := newScriptedEndpoint(t, textReply("all done"))

	exitCode, stdout, errorText := executeAgentCommand(t, "prompt",
		"agent", "--base-url", endpoint.server.URL, "--model", "m")

	if exitCode != 0 {
		t.Errorf("exit = %d, want 0: %s", exitCode, errorText)
	}
	if !strings.Contains(stdout, "all done") {
		t.Errorf("stdout is missing the reply:\n%s", stdout)
	}
}

func TestAgentCommand_BadUsageExits2(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no --base-url", args: []string{"agent", "--model", "m"}},
		{name: "no --model", args: []string{"agent", "--base-url", "http://127.0.0.1:1"}},
		{name: "a zero --max-turns", args: []string{"agent", "--base-url", "http://127.0.0.1:1", "--model", "m", "--max-turns", "0"}},
		{name: "a stray argument", args: []string{"agent", "--base-url", "http://127.0.0.1:1", "--model", "m", "stray"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exitCode, _, errorText := executeAgentCommand(t, "prompt", test.args...)

			if exitCode != exitBadUsage {
				t.Errorf("exit = %d, want %d: %s", exitCode, exitBadUsage, errorText)
			}
		})
	}
}

// executeAgentCommand runs the command tree with args and stdin, as main does, and returns the
// exit code main would exit with, the command's stdout and its error text.
func executeAgentCommand(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	root := newRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetArgs(args)
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	err := root.ExecuteContext(context.Background())

	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	return exitCodeFor(err), stdout.String(), stderr.String() + err.Error()
}

func TestAgentShell_ReportsTheExitCode(t *testing.T) {
	result := runShellCommand(context.Background(), t.TempDir(), "exit 3", shellTimeout)

	if !strings.Contains(result, "[exit code 3]") {
		t.Errorf("result = %q, want exit code 3", result)
	}
}

func TestAgentShell_RunsInTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("x"), runFileMode); err != nil {
		t.Fatal(err)
	}
	command := "ls"
	if runtime.GOOS == "windows" {
		command = "dir /B"
	}

	result := runShellCommand(context.Background(), dir, command, shellTimeout)

	if !strings.Contains(result, "marker.txt") {
		t.Errorf("result = %q, want the working directory's marker.txt listed", result)
	}
}

func TestAgentShell_ATimedOutCommandWithABackgroundChildReturnsAtTheTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are killed as a group on Unix only")
	}
	const timeout = 200 * time.Millisecond
	started := time.Now()

	result := runShellCommand(context.Background(), t.TempDir(), "sleep 30 & echo started; sleep 30", timeout)

	if elapsed := time.Since(started); elapsed > shellWaitDelay/2 {
		t.Errorf("returned after %s, want soon after the %s timeout", elapsed, timeout)
	}
	if !strings.Contains(result, "started") || !strings.Contains(result, "killed after the 200ms timeout") {
		t.Errorf("result = %q, want the output so far and the timeout", result)
	}
}

func TestAgentOutput_KeepsTheLimitAndCountsTheRest(t *testing.T) {
	buffer := &cappedBuffer{limit: 4}

	for _, chunk := range []string{"ab", "cdef", "gh"} {
		if n, err := buffer.Write([]byte(chunk)); n != len(chunk) || err != nil {
			t.Fatalf("Write(%q) = %d, %v; want %d, nil", chunk, n, err, len(chunk))
		}
	}

	if got, want := buffer.String(), "abcd\n[output cut at 4 bytes; 4 more not shown]\n"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
