package tui

import (
	"fmt"
	"strings"
	"unicode"

	"serialtypist/internal/keyboard"
)

const (
	fullGuideMinimumWidth  = 66
	fullGuideMinimumHeight = 25
)

func keyboardHint(expected rune) string {
	hint := keyboard.Lookup(expected)
	if !hint.Known {
		return fmt.Sprintf("Next  [%s](fg:hotpink,mod:bold) — no QWERTY finger mapping", safeCharacter(expected))
	}
	key := hint.Key.Label
	if hint.Shift {
		key = "Shift + " + key
	}
	return fmt.Sprintf("Next  [%s](fg:hotpink,mod:bold) — %s", key, hint.Key.Finger)
}

func renderKeyboardGuide(expected rune, errorState bool) string {
	hint := keyboard.Lookup(expected)
	var lines []string
	for rowIndex, row := range keyboard.Rows() {
		parts := make([]string, 0, len(row))
		for _, key := range row {
			label := fmt.Sprintf(" %s ", key.Label)
			if hint.Known && hint.Key.Base == key.Base {
				color := "hotpink"
				if errorState {
					color = "coral"
				}
				label = fmt.Sprintf("[%s](fg:%s,mod:bold)", label, color)
			}
			parts = append(parts, label)
		}
		indent := strings.Repeat(" ", rowIndex*2)
		lines = append(lines, indent+strings.Join(parts, ""))
	}
	shift := "Shift off"
	if hint.Shift {
		shift = "[Shift ON](fg:orchid,mod:bold)"
	}
	if hint.Known && hint.Key.Base == ' ' {
		shift += "    [       SPACE       ](fg:hotpink,mod:bold)"
	} else {
		shift += "           SPACE"
	}
	lines = append(lines, shift+"    "+keyboardHint(expected))
	return strings.Join(lines, "\n")
}

func nextExpected(sessionTarget []rune, position int) (rune, bool) {
	if position < 0 || position >= len(sessionTarget) {
		return 0, false
	}
	return sessionTarget[position], true
}

func safeCharacter(r rune) string {
	switch r {
	case '[':
		return "［"
	case ']':
		return "］"
	case ' ':
		return "Space"
	}
	if unicode.IsPrint(r) {
		return string(r)
	}
	return fmt.Sprintf("U+%04X", r)
}

func formatFingerTrouble(weights []keyboard.FingerWeight, limit int) string {
	if len(weights) == 0 {
		return "—"
	}
	if limit > len(weights) {
		limit = len(weights)
	}
	parts := make([]string, 0, limit)
	for _, item := range weights[:limit] {
		parts = append(parts, fmt.Sprintf("%s×%d", item.Finger, item.Weight))
	}
	return strings.Join(parts, " • ")
}
