package batch_test

import (
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/batch"
)

// fuzzName is the apply file name FuzzParseApply hands Parse.
const fuzzName = "batch.yaml"

// applySeeds are the inputs FuzzParseApply starts from: the worked example of the apply-file
// reference, an update and a log, an empty list, two documents and a refused file key.
var applySeeds = []string{
	`- op: create
  id: session-store
  title: Store sessions in the database
  parent: login-flow
  body: Keep each session in the sessions table.
  acceptance: A session survives a restart
  label: auth, db
- op: create
  id: session-refresh
  title: Refresh the session token
  parent: login-flow
  priority: 1
  body: Refresh the token before it expires.
  dep: session-store
- op: create
  id: session-expiry
  title: Expire idle sessions
  parent: login-flow
  body: End a session after 30 idle minutes.
  dep: [session-store]
- op: dep add
  id: login-flow
  dep: session-refresh
- op: close
  id: db-schema
  reason: designed in the plan
  if-match: f2cbae1354f0f58e
`,
	`- op: update
  id: login-flow
  status: active
  add-label: [auth]
  body-replace-old: [old]
  body-replace-new: [new]
- op: log
  id: login-flow
  text: started
`,
	"[]\n",
	"- op: log\n  id: a\n  text: one\n---\n- op: log\n  id: b\n  text: two\n",
	"- op: create\n  id: a\n  title: A\n  body-file: body.md\n",
}

// FuzzParseApply parses arbitrary content as an apply file: Parse never panics, and every error it
// returns names the file it was given.
func FuzzParseApply(f *testing.F) {
	for _, seed := range applySeeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := batch.Parse(fuzzName, data)
		if err != nil && !strings.Contains(err.Error(), fuzzName) {
			t.Errorf("Parse(%q) error %q does not name %q", data, err, fuzzName)
		}
	})
}
