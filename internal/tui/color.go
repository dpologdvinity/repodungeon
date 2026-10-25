package tui

import (
	"fmt"
	"strings"

	"github.com/dpologdvinity/repodungeon/internal/game"
)

// Colors are plain ANSI SGR codes. Bubble Tea reads them from the view and
// lowers them to what the terminal supports, and removes them when NO_COLOR is
// set or output is not a terminal, so no color library is needed. game.Render
// stays plain text: this file only wraps parts of it in escape codes and never
// adds, removes or moves a visible character (a test checks this).
const (
	sgrReset  = "\x1b[0m"
	sgrWall   = "37"   // light gray
	sgrDim    = "90"   // dark gray: floor, corridors, footer
	sgrDoor   = "33"   // yellow
	sgrStairs = "1;93" // bold bright yellow
	sgrPlayer = "1;92" // bold bright green
	sgrBoss   = "1;91" // bold bright red
	sgrMonst  = "31"   // red
	sgrPotion = "95"   // bright magenta
	sgrScroll = "96"   // bright cyan
	sgrTitle  = "1;93" // bold bright yellow: headings
	sgrGood   = "32"   // green
	sgrWarn   = "33"   // yellow
	sgrBad    = "1;91" // bold bright red
)

// Screen rows (SPEC 7.7): 0 messages, 1-21 body, 22 status, 23 footer.
const (
	rowStatus = 22
	rowFooter = 23
)

// paint wraps text in the given SGR code and a reset.
func paint(text, sgr string) string {
	if sgr == "" || text == "" {
		return text
	}
	return "\x1b[" + sgr + "m" + text + sgrReset
}

// glyphStyle is the color of one map character. Any other character is a
// monster letter.
func glyphStyle(r rune) string {
	switch r {
	case ' ':
		return ""
	case '@':
		return sgrPlayer
	case '&':
		return sgrBoss
	case '!':
		return sgrPotion
	case '?':
		return sgrScroll
	case '+':
		return sgrDoor
	case '>':
		return sgrStairs
	case '-', '|':
		return sgrWall
	case '.', '#':
		return sgrDim
	}
	return sgrMonst
}

// colorMapRow colors a map row, grouping equal neighbors into one escape code.
func colorMapRow(row string) string {
	var b strings.Builder
	var run []rune
	cur := ""
	flush := func() {
		b.WriteString(paint(string(run), cur))
		run = run[:0]
	}
	for _, r := range row {
		if st := glyphStyle(r); st != cur {
			flush()
			cur = st
		}
		run = append(run, r)
	}
	flush()
	return b.String()
}

// colorize returns the screen for s with colors added. Without its escape
// codes the result equals s.Render().
func colorize(s *game.State) string {
	rows := strings.Split(strings.TrimSuffix(s.Render(), "\n"), "\n")
	for i, row := range rows {
		switch {
		case i == 0:
			rows[i] = colorMessage(row)
		case i == rowStatus:
			rows[i] = colorStatus(s, row)
		case i == rowFooter:
			rows[i] = paint(row, sgrDim)
		case s.Mode == game.Help:
			rows[i] = colorHelpRow(row)
		case s.Mode == game.Inventory:
			rows[i] = colorInventoryRow(row)
		default:
			rows[i] = colorMapRow(row)
		}
	}
	return strings.Join(rows, "\n") + "\n"
}

// colorMessage colors the message row by what happened: bad news red, a win
// or a kill green, loot cyan.
func colorMessage(row string) string {
	switch {
	case strings.Contains(row, "hits you") || strings.Contains(row, "You die"):
		return paint(row, sgrBad)
	case strings.Contains(row, "You hit") || strings.Contains(row, "slay") ||
		strings.Contains(row, "stronger") || strings.Contains(row, "You win"):
		return paint(row, sgrGood)
	case strings.Contains(row, "You pick up") || strings.Contains(row, "You drink") ||
		strings.Contains(row, "You read"):
		return paint(row, sgrScroll)
	}
	return row
}

// colorStatus colors the "HP a/b" part by how much health is left.
func colorStatus(s *game.State, row string) string {
	hp := fmt.Sprintf("HP %d/%d", s.P.HP, s.P.MaxHP)
	sgr := sgrGood
	switch pct := s.P.HP * 100 / max(s.P.MaxHP, 1); {
	case pct <= 25:
		sgr = sgrBad
	case pct <= 50:
		sgr = sgrWarn
	}
	return strings.Replace(row, hp, paint(hp, sgr), 1)
}

// helpLegend maps the start of a legend line on the help screen to the text
// that is colored like on the map.
var helpLegend = []struct{ prefix, token string }{
	{"  @ you", "@"},
	{"  & the boss", "&"},
	{"  ! potion", "!"},
	{"  ? scroll", "?"},
	{"  a-z a monster", "a-z"},
	{"  . floor", "."},
	{"  # corridor", "#"},
	{"  + door", "+"},
	{"  - | walls", "- |"},
	{"  > stairs", ">"},
}

// colorHelpRow colors headings and the glyph at the start of legend lines.
func colorHelpRow(row string) string {
	switch row {
	case "HOW TO PLAY", "KEYS", "THE MAP":
		return paint(row, sgrTitle)
	}
	for _, l := range helpLegend {
		if strings.HasPrefix(row, l.prefix) {
			sgr := sgrMonst // "a-z" is the only multi-letter token that is not a symbol
			if l.token != "a-z" {
				sgr = glyphStyle([]rune(l.token)[0])
			}
			return "  " + paint(l.token, sgr) + row[2+len(l.token):]
		}
	}
	return row
}

// colorInventoryRow colors the inventory header and each item by its kind.
func colorInventoryRow(row string) string {
	switch {
	case strings.HasPrefix(row, "Inventory"):
		return paint(row, sgrTitle)
	case strings.Contains(row, "TODO potion"):
		return paint(row, sgrPotion)
	case strings.Contains(row, "FIXME scroll"):
		return paint(row, sgrScroll)
	}
	return row
}
