package keyboard

import "testing"

func TestLookupQWERTYKeys(t *testing.T) {
	tests := []struct {
		character rune
		base      rune
		finger    Finger
		shift     bool
	}{
		{'a', 'a', LeftPinky, false},
		{'A', 'a', LeftPinky, true},
		{'!', '1', LeftPinky, true},
		{'?', '/', RightPinky, true},
		{' ', ' ', Thumbs, false},
	}
	for _, test := range tests {
		hint := Lookup(test.character)
		if !hint.Known || hint.Key.Base != test.base || hint.Key.Finger != test.finger || hint.Shift != test.shift {
			t.Fatalf("Lookup(%q) = %+v", test.character, hint)
		}
	}
	if hint := Lookup('é'); hint.Known {
		t.Fatalf("Lookup(é) = %+v, want unknown", hint)
	}
}

func TestTroubleByFinger(t *testing.T) {
	got := TroubleByFinger(map[string]int{"a": 2, "A": 3, "?": 4, "é": 99})
	if len(got) != 2 || got[0] != (FingerWeight{Finger: LeftPinky, Weight: 5}) || got[1] != (FingerWeight{Finger: RightPinky, Weight: 4}) {
		t.Fatalf("TroubleByFinger() = %#v", got)
	}
}
