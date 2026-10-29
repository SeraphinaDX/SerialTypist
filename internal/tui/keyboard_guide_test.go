package tui

import (
	"strings"
	"testing"

	"serialtypist/internal/keyboard"
)

func TestKeyboardHintIncludesShiftAndFinger(t *testing.T) {
	got := keyboardHint('A')
	if !strings.Contains(got, "Shift + A") || !strings.Contains(got, "left pinky") {
		t.Fatalf("keyboardHint(A) = %q", got)
	}
}

func TestKeyboardGuideUsesErrorHighlight(t *testing.T) {
	got := renderKeyboardGuide('?', true)
	if !strings.Contains(got, "Shift ON") || !strings.Contains(got, "fg:coral") || !strings.Contains(got, "right pinky") {
		t.Fatalf("renderKeyboardGuide(?, true) = %q", got)
	}
}

func TestFormatFingerTrouble(t *testing.T) {
	got := formatFingerTrouble([]keyboard.FingerWeight{{Finger: keyboard.LeftIndex, Weight: 7}, {Finger: keyboard.RightPinky, Weight: 4}}, 2)
	if got != "left index×7 • right pinky×4" {
		t.Fatalf("formatFingerTrouble() = %q", got)
	}
}
