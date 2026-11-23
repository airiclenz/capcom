package index

// SearchEveryWordFirst returns the hits of everyWord with isEveryWord set when it finds any, else
// the hits of anyWord with isEveryWord unset. Either search's error is returned as it is.
func SearchEveryWordFirst[E any](everyWord, anyWord func() ([]E, error)) (entries []E, isEveryWord bool, err error) {
	entries, err = everyWord()
	if err != nil || len(entries) > 0 {
		return entries, true, err
	}
	entries, err = anyWord()
	return entries, false, err
}
