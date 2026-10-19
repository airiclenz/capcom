package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/airiclenz/capcom/internal/cli/output"
)

// countFlagType is the pflag type name of a flag that counts its own repetitions (-vvv).
const countFlagType = "count"

// onceValue wraps the value of a flag that holds one value and counts how often the command line
// sets it. pflag lets the last of several values win silently; the count lets prepare refuse the
// repetition instead. String and Type are the wrapped value's own, so help, the generated reference
// and the typed getters (GetString, GetBool, ...) see no difference.
type onceValue struct {
	pflag.Value
	setCount int
}

// Set counts the call, then hands the value to the wrapped flag value.
func (v *onceValue) Set(value string) error {
	v.setCount++
	return v.Value.Set(value)
}

// guardSingleValueFlags wraps every local and persistent flag of command and of each command below
// it that holds one value, so refuseRepeatedFlags can tell when one was given more than once.
// Slice, array and count flags accumulate their repetitions and are left as they are; a flag
// already wrapped is not wrapped twice. Call it once the tree is built, before it parses anything.
func guardSingleValueFlags(command *cobra.Command) {
	wrap := func(flag *pflag.Flag) {
		if _, isWrapped := flag.Value.(*onceValue); isWrapped || isAccumulating(flag.Value) {
			return
		}
		flag.Value = &onceValue{Value: flag.Value}
	}
	command.Flags().VisitAll(wrap)
	command.PersistentFlags().VisitAll(wrap)
	for _, child := range command.Commands() {
		guardSingleValueFlags(child)
	}
}

// isAccumulating reports whether value collects every repetition of its flag rather than keeping
// only one value: a slice, an array or a count.
func isAccumulating(value pflag.Value) bool {
	valueType := value.Type()
	return strings.HasSuffix(valueType, "Slice") ||
		strings.HasSuffix(valueType, "Array") ||
		valueType == countFlagType
}

// refuseRepeatedFlags returns a usage error naming the first flag of flags, in name order, that
// guardSingleValueFlags wrapped and the command line set more than once; nil when there is none.
func refuseRepeatedFlags(flags *pflag.FlagSet) error {
	var refusal error
	flags.VisitAll(func(flag *pflag.Flag) {
		guarded, isGuarded := flag.Value.(*onceValue)
		if refusal != nil || !isGuarded || guarded.setCount < 2 {
			return
		}
		refusal = output.UsageErrorf("--%s given %d times; give it once", flag.Name, guarded.setCount)
	})
	return refusal
}
