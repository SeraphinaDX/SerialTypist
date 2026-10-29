// Package keyboard describes the physical keys and standard touch-typing
// fingers for a US QWERTY keyboard. It deliberately has no TUI dependency so
// additional layouts can be added without changing the renderer.
package keyboard

import (
	"sort"
	"unicode"
)

type Finger string

const (
	LeftPinky   Finger = "left pinky"
	LeftRing    Finger = "left ring"
	LeftMiddle  Finger = "left middle"
	LeftIndex   Finger = "left index"
	RightIndex  Finger = "right index"
	RightMiddle Finger = "right middle"
	RightRing   Finger = "right ring"
	RightPinky  Finger = "right pinky"
	Thumbs      Finger = "thumbs"
)

// Key is one physical key in the displayed keyboard layout.
type Key struct {
	Base    rune
	Label   string
	Finger  Finger
	Shifted rune
}

// Hint describes how to type a target character.
type Hint struct {
	Expected rune
	Key      Key
	Shift    bool
	Known    bool
}

// FingerWeight combines recorded character mistakes by touch-typing finger.
type FingerWeight struct {
	Finger Finger
	Weight int
}

var rows = [][]Key{
	{
		key('`', "`", LeftPinky, '~'), key('1', "1", LeftPinky, '!'), key('2', "2", LeftRing, '@'),
		key('3', "3", LeftMiddle, '#'), key('4', "4", LeftIndex, '$'), key('5', "5", LeftIndex, '%'),
		key('6', "6", RightIndex, '^'), key('7', "7", RightIndex, '&'), key('8', "8", RightMiddle, '*'),
		key('9', "9", RightRing, '('), key('0', "0", RightPinky, ')'), key('-', "-", RightPinky, '_'),
		key('=', "=", RightPinky, '+'),
	},
	{
		letter('q', LeftPinky), letter('w', LeftRing), letter('e', LeftMiddle), letter('r', LeftIndex),
		letter('t', LeftIndex), letter('y', RightIndex), letter('u', RightIndex), letter('i', RightMiddle),
		letter('o', RightRing), letter('p', RightPinky), key('[', "［", RightPinky, '{'),
		key(']', "］", RightPinky, '}'), key('\\', "\\", RightPinky, '|'),
	},
	{
		letter('a', LeftPinky), letter('s', LeftRing), letter('d', LeftMiddle), letter('f', LeftIndex),
		letter('g', LeftIndex), letter('h', RightIndex), letter('j', RightIndex), letter('k', RightMiddle),
		letter('l', RightRing), key(';', ";", RightPinky, ':'), key('\'', "'", RightPinky, '"'),
	},
	{
		letter('z', LeftPinky), letter('x', LeftRing), letter('c', LeftMiddle), letter('v', LeftIndex),
		letter('b', LeftIndex), letter('n', RightIndex), letter('m', RightIndex), key(',', ",", RightMiddle, '<'),
		key('.', ".", RightRing, '>'), key('/', "/", RightPinky, '?'),
	},
}

func key(base rune, label string, finger Finger, shifted rune) Key {
	return Key{Base: base, Label: label, Finger: finger, Shifted: shifted}
}

func letter(base rune, finger Finger) Key {
	return key(base, string(unicode.ToUpper(base)), finger, unicode.ToUpper(base))
}

// Rows returns a copy of the QWERTY layout used by the guide.
func Rows() [][]Key {
	result := make([][]Key, len(rows))
	for index := range rows {
		result[index] = append([]Key(nil), rows[index]...)
	}
	return result
}

// Lookup finds the physical key, Shift state, and finger for a character.
func Lookup(expected rune) Hint {
	if expected == ' ' {
		return Hint{Expected: expected, Key: Key{Base: ' ', Label: "Space", Finger: Thumbs}, Known: true}
	}
	for _, row := range rows {
		for _, candidate := range row {
			if expected == candidate.Base {
				return Hint{Expected: expected, Key: candidate, Known: true}
			}
			if expected == candidate.Shifted {
				return Hint{Expected: expected, Key: candidate, Shift: true, Known: true}
			}
		}
	}
	return Hint{Expected: expected}
}

// TroubleByFinger aggregates character mistake weights for known keys.
func TroubleByFinger(characters map[string]int) []FingerWeight {
	weights := make(map[Finger]int)
	for character, weight := range characters {
		runes := []rune(character)
		if len(runes) != 1 || weight <= 0 {
			continue
		}
		hint := Lookup(runes[0])
		if hint.Known {
			weights[hint.Key.Finger] += weight
		}
	}
	result := make([]FingerWeight, 0, len(weights))
	for finger, weight := range weights {
		result = append(result, FingerWeight{Finger: finger, Weight: weight})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Weight == result[j].Weight {
			return result[i].Finger < result[j].Finger
		}
		return result[i].Weight > result[j].Weight
	})
	return result
}
