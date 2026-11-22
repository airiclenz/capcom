package proc

import "testing"

func TestParseStat_CommWithSpacesAndParentheses_ReadsTheFieldsAfterIt(t *testing.T) {
	stat := "4242 (my (odd) name) S 4241 4242 4242 0 -1 4194560 100 0 0 0 1 2 0 0 20 0 1 0 987654 1000 10"

	parsed, err := parseStat(4242, stat)

	if err != nil {
		t.Fatalf("parseStat: %v", err)
	}
	want := entry{
		process:   Process{PID: 4242, Started: "987654", Name: "my (odd) name"},
		parentPID: 4241,
		order:     987654,
	}
	if parsed != want {
		t.Errorf("parseStat = %+v, want %+v", parsed, want)
	}
}

func TestParseStat_Truncated_Fails(t *testing.T) {
	_, err := parseStat(7, "7 (cut) S 6 7")

	if err == nil {
		t.Error("parseStat of a truncated stat succeeded")
	}
}
