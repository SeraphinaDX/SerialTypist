package tui

import (
	"image"
	"unicode"

	rw "github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
)

// TypingPane draws each target rune directly into gotui's cell buffer. This
// gives the tutor precise per-character colors and an exact visual cursor.
type TypingPane struct {
	*ui.Block
	Target []rune
	Typed  []rune

	PendingStyle     ui.Style
	CurrentWordStyle ui.Style
	CorrectStyle     ui.Style
	IncorrectStyle   ui.Style
	CursorStyle      ui.Style
}

func NewTypingPane() *TypingPane {
	block := ui.NewBlock()
	block.BorderRounded = true
	block.PaddingLeft = 1
	block.PaddingRight = 1
	block.PaddingTop = 1
	block.PaddingBottom = 1
	block.BackgroundColor = ui.NewRGBColor(16, 9, 18)
	block.BorderStyle = ui.NewStyle(ui.NewRGBColor(255, 95, 183))
	block.TitleStyle = ui.NewStyle(ui.NewRGBColor(255, 158, 210), ui.ColorClear, ui.ModifierBold)

	return &TypingPane{
		Block:            block,
		PendingStyle:     ui.NewStyle(ui.NewRGBColor(181, 138, 170), ui.NewRGBColor(16, 9, 18)),
		CurrentWordStyle: ui.NewStyle(ui.NewRGBColor(255, 238, 248), ui.NewRGBColor(55, 26, 48)),
		CorrectStyle:     ui.NewStyle(ui.NewRGBColor(125, 226, 209), ui.NewRGBColor(55, 26, 48)),
		IncorrectStyle:   ui.NewStyle(ui.NewRGBColor(255, 238, 248), ui.NewRGBColor(164, 48, 84), ui.ModifierBold),
		CursorStyle:      ui.NewStyle(ui.NewRGBColor(22, 9, 20), ui.NewRGBColor(255, 121, 198), ui.ModifierBold),
	}
}

func (p *TypingPane) Draw(buf *ui.Buffer) {
	p.Block.Draw(buf)
	if p.Inner.Empty() || len(p.Target) == 0 {
		return
	}

	placed := layoutRunes(p.Target, p.Inner.Dx())
	position := len(p.Typed)
	cursorLine := 0
	if position < len(placed) {
		cursorLine = placed[position].Y
	} else if len(placed) > 0 {
		cursorLine = placed[len(placed)-1].Y
	}
	scroll := cursorLine - p.Inner.Dy()/2
	if scroll < 0 {
		scroll = 0
	}
	wordStart, wordEnd := wordBounds(p.Target, position)

	for i, cell := range placed {
		y := cell.Y - scroll
		if y < 0 || y >= p.Inner.Dy() {
			continue
		}

		style := p.PendingStyle
		if i >= wordStart && i < wordEnd {
			style = p.CurrentWordStyle
		}
		if i < len(p.Typed) {
			if p.Typed[i] == p.Target[i] {
				style = p.CorrectStyle
			} else {
				style = p.IncorrectStyle
			}
		}
		if i == position {
			style = p.CursorStyle
		}

		point := image.Pt(p.Inner.Min.X+cell.X, p.Inner.Min.Y+y)
		buf.SetCell(ui.NewCell(p.Target[i], style), point)
		for extra := 1; extra < cell.Width; extra++ {
			buf.SetCell(ui.NewCell(' ', style), point.Add(image.Pt(extra, 0)))
		}
	}
}

type placedRune struct {
	X, Y  int
	Width int
}

func layoutRunes(text []rune, width int) []placedRune {
	placed := make([]placedRune, len(text))
	if width < 1 {
		return placed
	}

	x, y := 0, 0
	for i := 0; i < len(text); {
		if unicode.IsSpace(text[i]) {
			if x >= width {
				x, y = 0, y+1
			}
			placed[i] = placedRune{X: x, Y: y, Width: 1}
			x++
			i++
			continue
		}

		end := i
		wordWidth := 0
		for end < len(text) && !unicode.IsSpace(text[end]) {
			wordWidth += runeWidth(text[end])
			end++
		}
		if x > 0 && x+wordWidth > width {
			x, y = 0, y+1
		}
		for i < end {
			cellWidth := runeWidth(text[i])
			if x > 0 && x+cellWidth > width {
				x, y = 0, y+1
			}
			placed[i] = placedRune{X: x, Y: y, Width: cellWidth}
			x += cellWidth
			i++
		}
	}
	return placed
}

func runeWidth(r rune) int {
	width := rw.RuneWidth(r)
	if width < 1 {
		return 1
	}
	return width
}

func wordBounds(target []rune, position int) (int, int) {
	if len(target) == 0 {
		return 0, 0
	}
	if position >= len(target) {
		position = len(target) - 1
	}
	if position < 0 {
		position = 0
	}
	if unicode.IsSpace(target[position]) {
		return position, position + 1
	}
	start := position
	for start > 0 && !unicode.IsSpace(target[start-1]) {
		start--
	}
	end := position
	for end < len(target) && !unicode.IsSpace(target[end]) {
		end++
	}
	return start, end
}
