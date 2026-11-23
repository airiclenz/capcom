package index

import (
	"errors"
	"slices"
	"testing"
)

func TestSearchEveryWordFirst_EveryWordHits_ReturnsThemWithoutTheAnyWordSearch(t *testing.T) {
	t.Parallel()
	isAnyWordRun := false

	entries, isEveryWord, err := SearchEveryWordFirst(
		func() ([]string, error) { return []string{"both"}, nil },
		func() ([]string, error) { isAnyWordRun = true; return []string{"either"}, nil },
	)

	if err != nil || !isEveryWord || !slices.Equal(entries, []string{"both"}) || isAnyWordRun {
		t.Errorf("SearchEveryWordFirst = %v, %v, %v (any-word run: %v); want [both], true, nil, no any-word run",
			entries, isEveryWord, err, isAnyWordRun)
	}
}

func TestSearchEveryWordFirst_NoEveryWordHit_ReturnsTheAnyWordHits(t *testing.T) {
	t.Parallel()

	entries, isEveryWord, err := SearchEveryWordFirst(
		func() ([]string, error) { return nil, nil },
		func() ([]string, error) { return []string{"either", "other"}, nil },
	)

	if err != nil || isEveryWord || !slices.Equal(entries, []string{"either", "other"}) {
		t.Errorf("SearchEveryWordFirst = %v, %v, %v; want [either other], false, nil", entries, isEveryWord, err)
	}
}

func TestSearchEveryWordFirst_SearchFails_ReturnsItsErrorAsItIs(t *testing.T) {
	t.Parallel()
	failure := errors.New("index busy")
	cases := []struct {
		name      string
		everyWord func() ([]string, error)
		anyWord   func() ([]string, error)
	}{
		{
			"every-word search fails",
			func() ([]string, error) { return nil, failure },
			func() ([]string, error) { return []string{"either"}, nil },
		},
		{
			"any-word search fails",
			func() ([]string, error) { return nil, nil },
			func() ([]string, error) { return nil, failure },
		},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()

			entries, _, err := SearchEveryWordFirst(current.everyWord, current.anyWord)

			if !errors.Is(err, failure) || entries != nil {
				t.Errorf("SearchEveryWordFirst = %v, %v; want no hits and the search's error", entries, err)
			}
		})
	}
}
