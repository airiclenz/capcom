package proc

import (
	"errors"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"testing"
)

// fakeReader serves entries by PID and fails for any other PID.
func fakeReader(entries ...entry) reader {
	byPID := make(map[int]entry, len(entries))
	for _, listed := range entries {
		byPID[listed.process.PID] = listed
	}
	return func(pid int) (entry, error) {
		listed, ok := byPID[pid]
		if !ok {
			return entry{}, errors.New("no such process")
		}
		return listed, nil
	}
}

// fakeEntry returns an entry for pid with the given parent and start order.
func fakeEntry(pid, parentPID int, order int64) entry {
	return entry{
		process:   Process{PID: pid, Started: strconv.FormatInt(order, 10), Name: "p" + strconv.Itoa(pid)},
		parentPID: parentPID,
		order:     order,
	}
}

// pids returns the PIDs of chain in order.
func pids(chain []Process) []int {
	var listed []int
	for _, process := range chain {
		listed = append(listed, process.PID)
	}
	return listed
}

func TestAnchor_Chains(t *testing.T) {
	cases := []struct {
		name      string
		names     []string
		wantName  string
		wantFound bool
	}{
		{"shells and wrappers are skipped", []string{"bash", "env", "nohup", "claude", "zsh"}, "claude", true},
		{"the first other name wins", []string{"node", "claude"}, "node", true},
		{"an all-shell chain has no anchor", []string{"sh", "bash", "sudo", "fish"}, "", false},
		{"an empty chain has no anchor", nil, "", false},
		{"case and .exe are ignored", []string{"BASH.EXE", "Pwsh.exe", "Code.exe"}, "Code.exe", true},
		{"a path is matched on its base name", []string{`C:\Windows\System32\cmd.exe`, "/usr/bin/env", "agent"}, "agent", true},
		{"a capcom shim is skipped", []string{"capcom", "claude"}, "claude", true},
		{"a capcom shim is skipped whatever its case and .exe", []string{"CAPCOM.EXE", "bash", "claude"}, "claude", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var chain []Process
			for i, name := range tt.names {
				chain = append(chain, Process{PID: 100 + i, Started: "1", Name: name})
			}

			anchor, found := Anchor(chain)

			if found != tt.wantFound || anchor.Name != tt.wantName {
				t.Errorf("Anchor = (%q, %v), want (%q, %v)", anchor.Name, found, tt.wantName, tt.wantFound)
			}
		})
	}
}

func TestAncestry_TestProcess_StartsAtTheParent(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("no process reader on " + runtime.GOOS)
	}

	chain := Ancestry()

	if len(chain) == 0 {
		t.Fatal("Ancestry is empty")
	}
	if chain[0].PID != os.Getppid() || chain[0].Started == "" {
		t.Errorf("first ancestor = %+v, want PID %d with a start stamp", chain[0], os.Getppid())
	}
}

func TestAncestry_TwoCalls_ReturnEqualChains(t *testing.T) {
	first := Ancestry()

	second := Ancestry()

	if !reflect.DeepEqual(first, second) {
		t.Errorf("Ancestry changed between calls:\n%+v\n%+v", first, second)
	}
}

func TestWalk_StopRules(t *testing.T) {
	cases := []struct {
		name    string
		entries []entry
		want    []int
	}{
		{"stops before PID 1", []entry{fakeEntry(10, 9, 5), fakeEntry(9, 1, 4), fakeEntry(1, 0, 0)}, []int{9}},
		{"stops at an unreadable parent", []entry{fakeEntry(10, 9, 5), fakeEntry(9, 8, 4)}, []int{9}},
		{"stops at a parent that started after its child", []entry{fakeEntry(10, 9, 5), fakeEntry(9, 8, 4), fakeEntry(8, 7, 6)}, []int{9}},
		{"an unreadable caller has no ancestry", nil, nil},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			chain := walk(10, fakeReader(tt.entries...))

			if got := pids(chain); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("walk PIDs = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWalk_LongChain_StopsAfterTheLimit(t *testing.T) {
	var entries []entry
	for pid := 2; pid <= maxAncestry+10; pid++ {
		entries = append(entries, fakeEntry(pid, pid+1, int64(1000-pid)))
	}

	chain := walk(2, fakeReader(entries...))

	if len(chain) != maxAncestry {
		t.Errorf("walk returned %d ancestors, want %d", len(chain), maxAncestry)
	}
}
