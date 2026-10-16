package lock

import (
	"fmt"
	"time"
)

// minutesPerHour splits a lock age into hours and minutes.
const minutesPerHour = 60

// AgeText renders a lock age in whole minutes: `1h05m`, `12m`. A negative age, from a clock that
// moved back, renders as `0m`.
func AgeText(age time.Duration) string {
	minutes := int(max(age, 0) / time.Minute)
	if minutes < minutesPerHour {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh%02dm", minutes/minutesPerHour, minutes%minutesPerHour)
}
