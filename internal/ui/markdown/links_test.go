package markdown

import (
	"reflect"
	"strings"
	"testing"
)

// knownIDs returns an Options whose IsID accepts exactly ids.
func knownIDs(ids ...string) Options {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return Options{IsID: func(s string) bool { return set[s] }}
}

// regionText returns the visible text under link's region of its line in rendered.
func regionText(t *testing.T, th Theme, rendered Rendered, link Link) string {
	t.Helper()
	if link.Line < 0 || link.Line >= len(rendered.Lines) {
		t.Fatalf("link %+v names line %d of %d", link, link.Line, len(rendered.Lines))
	}
	return th.Measure.Cut(strip(rendered.Lines[link.Line]), link.Col, link.Col+link.Width)
}

// assertNoMarkers fails when a link marker survived into the rendered lines.
func assertNoMarkers(t *testing.T, rendered Rendered) {
	t.Helper()
	for i, line := range rendered.Lines {
		if strings.Contains(line, linkMarkerIntro) {
			t.Errorf("line %d still carries a link marker: %q", i, line)
		}
	}
}

func TestLinkRegionIsMeasuredInCellsAfterWideCharacters(t *testing.T) {
	t.Parallel()
	th := testTheme()

	got := Render(th, "日本語 see [the epic](capcom-ui) now", 60, knownIDs("capcom-ui"))

	want := []Link{{Target: "capcom-ui", Line: 0, Col: 11, Width: 8}}
	if !reflect.DeepEqual(got.Links, want) {
		t.Fatalf("Links = %+v; want %+v", got.Links, want)
	}
	if text := regionText(t, th, got, got.Links[0]); text != "the epic" {
		t.Errorf("region text = %q; want %q", text, "the epic")
	}
	if v := strip(got.Lines[0]); v != "日本語 see the epic now" {
		t.Errorf("visible = %q; want the link text without its markup", v)
	}
	assertNoMarkers(t, got)
}

func TestLinkWrappedOverTwoLinesRecordsOneRegionPerSegment(t *testing.T) {
	t.Parallel()
	th := testTheme()

	got := Render(th, "read [the capcom ui epic](capcom-ui) first", 14, knownIDs("capcom-ui"))

	if len(got.Links) < 2 {
		t.Fatalf("Links = %+v; want one region per wrapped segment, at least two", got.Links)
	}
	var segments []string
	for i, link := range got.Links {
		if link.Target != "capcom-ui" {
			t.Errorf("region %d target = %q; want capcom-ui", i, link.Target)
		}
		if i > 0 && link.Line != got.Links[i-1].Line+1 {
			t.Errorf("region %d is on line %d; want the line after region %d's", i, link.Line, i-1)
		}
		segments = append(segments, regionText(t, th, got, link))
	}
	if joined := strings.Join(segments, " "); joined != "the capcom ui epic" {
		t.Errorf("segments %q join to %q; want the whole link text", segments, joined)
	}
	assertNoMarkers(t, got)
}

func TestInlineCodeNamingAnIDIsALink(t *testing.T) {
	t.Parallel()
	th := testTheme()

	got := Render(th, "run `capcom-ui` before `go test`", 60, knownIDs("capcom-ui"))

	want := []Link{{Target: "capcom-ui", Line: 0, Col: 4, Width: 9}}
	if !reflect.DeepEqual(got.Links, want) {
		t.Fatalf("Links = %+v; want only the id span %+v", got.Links, want)
	}
	if v := strip(got.Lines[0]); v != "run capcom-ui before go test" {
		t.Errorf("visible = %q; want both spans without backticks", v)
	}
	if colorActive(th) && !strings.Contains(got.Lines[0], th.Link.Render("capcom-ui")) {
		t.Errorf("id span not in the Link style: %q", got.Lines[0])
	}
	assertNoMarkers(t, got)
}

func TestURLsRenderStyledButUnrecorded(t *testing.T) {
	t.Parallel()
	th := testTheme()
	source := "see [the docs](https://example.com/a \"title\"), [notes](./notes.md) and <https://x.io/b>"

	got := Render(th, source, 80, knownIDs("capcom-ui"))

	if len(got.Links) != 0 {
		t.Errorf("Links = %+v; want none for non-id targets", got.Links)
	}
	if v := strip(got.Lines[0]); v != "see the docs, notes and https://x.io/b" {
		t.Errorf("visible = %q; want each link's text without its markup", v)
	}
	if colorActive(th) && !strings.Contains(got.Lines[0], th.Inert.Render("the docs")) {
		t.Errorf("URL link not in the Inert style: %q", got.Lines[0])
	}
	assertNoMarkers(t, got)
}

func TestNoIDTestMakesNothingFollowable(t *testing.T) {
	t.Parallel()
	th := testTheme()

	got := Render(th, "[epic](capcom-ui) and `capcom-ui`", 60, Options{})

	if len(got.Links) != 0 {
		t.Errorf("Links = %+v; want none without an IsID", got.Links)
	}
	if v := strip(got.Lines[0]); v != "epic and capcom-ui" {
		t.Errorf("visible = %q", v)
	}
}

func TestMalformedLinksStayLiteral(t *testing.T) {
	t.Parallel()
	th := testTheme()
	for _, in := range []string{"[a] b", "[a](b", "[a]()", "[a [b](c", "<not a url>", "a < b > c"} {
		got := Render(th, in, 60, knownIDs("b", "c"))
		if v := strip(got.Lines[0]); v != in {
			t.Errorf("Render(%q) visible = %q; want it literal", in, v)
		}
		if len(got.Links) != 0 {
			t.Errorf("Render(%q) Links = %+v; want none", in, got.Links)
		}
	}
}

func TestLinkRegionsInsideListsAndTablesFollowTheirPrefixes(t *testing.T) {
	t.Parallel()
	th := testTheme()
	source := "- see [x](capcom-ui)\n\n| id | what |\n|----|------|\n| `ui-markdown` | renderer |"

	got := Render(th, source, 60, knownIDs("capcom-ui", "ui-markdown"))

	if len(got.Links) != 2 {
		t.Fatalf("Links = %+v; want the list link and the table cell link", got.Links)
	}
	for i, want := range []string{"x", "ui-markdown"} {
		if text := regionText(t, th, got, got.Links[i]); text != want {
			t.Errorf("region %d text = %q; want %q", i, text, want)
		}
	}
	if got.Links[0].Col != 6 {
		t.Errorf("list link Col = %d; want 6, past the bullet and \"see \"", got.Links[0].Col)
	}
	assertNoMarkers(t, got)
}

func TestBlockquoteWrapsInsideItsBar(t *testing.T) {
	t.Parallel()
	th := testTheme()

	got := Render(th, "> "+strings.Repeat("quoted words ", 4), 16, Options{})

	if len(got.Lines) < 2 {
		t.Fatalf("Lines = %q; want the quote wrapped", visible(got.Lines))
	}
	for i, line := range got.Lines {
		v := strip(line)
		if !strings.HasPrefix(v, glyphQuoteBar+" ") {
			t.Errorf("line %d = %q; want it to start with the quote bar", i, v)
		}
		if w := th.Measure.Width(line); w > 16 {
			t.Errorf("line %d is %d cells; want at most 16", i, w)
		}
	}
}

func TestNestedBlockquoteWithAListKeepsBothBarsAndTheHang(t *testing.T) {
	t.Parallel()
	th := testTheme()
	source := "> outer\n> > - item one two three\n>> 1. [link](capcom-ui)"

	got := Render(th, source, 16, knownIDs("capcom-ui"))

	bar := glyphQuoteBar + " "
	want := []string{
		bar + "outer",
		bar + bar + glyphBullet + " item one",
		bar + bar + "  two three",
		bar + bar + "1. link",
	}
	if v := visibleTrimmed(got.Lines); !reflect.DeepEqual(v, want) {
		t.Fatalf("visible = %q; want %q", v, want)
	}
	wantLink := []Link{{Target: "capcom-ui", Line: 3, Col: 7, Width: 4}}
	if !reflect.DeepEqual(got.Links, wantLink) {
		t.Errorf("Links = %+v; want %+v", got.Links, wantLink)
	}
}

func TestBlockquoteTooNarrowForItsBarsDropsThem(t *testing.T) {
	t.Parallel()
	th := testTheme()

	got := Render(th, "> > ab", 4, Options{})

	for i, line := range got.Lines {
		if strings.Contains(strip(line), glyphQuoteBar) {
			t.Errorf("line %d = %q; want the bars dropped at width 4", i, strip(line))
		}
	}
}
