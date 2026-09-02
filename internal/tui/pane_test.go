package tui

import "testing"

func TestLayoutWrapsAtWordBoundary(t *testing.T) {
	placed := layoutRunes([]rune("one two"), 5)
	if placed[4].X != 0 || placed[4].Y != 1 {
		t.Fatalf("second word starts at (%d,%d), want (0,1)", placed[4].X, placed[4].Y)
	}
}

func TestWordBounds(t *testing.T) {
	text := []rune("pink rose garden")
	start, end := wordBounds(text, 7)
	if string(text[start:end]) != "rose" {
		t.Fatalf("word = %q, want rose", string(text[start:end]))
	}
}
