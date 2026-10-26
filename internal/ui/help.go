package ui

// helpLines are the help screen ? opens over the panes: the full key map.
var helpLines = []string{
	"",
	"  Keys",
	"",
	"  move     ↑↓←→  j k h l",
	"  expand   → / l    collapse ← / h",
	"  tabs     Tab  1 Issues  2 Ready  3 Knowledge  4 Closed",
	"  open     Enter    back     Esc",
	"  scroll   PgUp PgDn  ctrl+u ctrl+d",
	"  resize   < >      drag the divider, double-click resets",
	"  copy     drag over the detail text",
	"  filter   /        deps     d",
	"  status   s        claim    a",
	"  close    c        reopen   R",
	"  log      L        editor   e",
	"  help     ?        quit     q",
	"",
	"  The scroll keys move the detail pane from anywhere; in the narrow",
	"  detail view the arrows and j k scroll it too.",
	"  Prompts take Enter to submit and Esc to cancel.",
	"  ? or Esc closes this help.",
}
