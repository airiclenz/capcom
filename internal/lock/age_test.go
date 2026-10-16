package lock

import (
	"testing"
	"time"
)

func TestAgeText_Durations(t *testing.T) {
	cases := []struct {
		name string
		age  time.Duration
		want string
	}{
		{name: "negative", age: -time.Minute, want: "0m"},
		{name: "under a minute", age: 59 * time.Second, want: "0m"},
		{name: "minutes", age: 12*time.Minute + 30*time.Second, want: "12m"},
		{name: "one hour", age: time.Hour, want: "1h00m"},
		{name: "hours and minutes", age: 3*time.Hour + 5*time.Minute, want: "3h05m"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := AgeText(testCase.age)

			if got != testCase.want {
				t.Errorf("AgeText(%v) = %q, want %q", testCase.age, got, testCase.want)
			}
		})
	}
}
