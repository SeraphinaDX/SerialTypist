package typing

import (
	"math"
	"testing"
	"time"
)

func TestSessionTracksTypingAndCorrections(t *testing.T) {
	start := time.Unix(100, 0)
	s := New("cat", 0)
	s.Add('c', start)
	s.Add('x', start.Add(time.Second))
	if s.CurrentErrors() != 1 {
		t.Fatalf("CurrentErrors = %d, want 1", s.CurrentErrors())
	}
	s.Backspace()
	s.Add('a', start.Add(2*time.Second))
	s.Add('t', start.Add(3*time.Second))

	if !s.Done() || s.Reason != FinishedTarget {
		t.Fatalf("session did not finish the target: done=%v reason=%q", s.Done(), s.Reason)
	}
	if s.CorrectCharacters() != 3 {
		t.Fatalf("CorrectCharacters = %d, want 3", s.CorrectCharacters())
	}
	if math.Abs(s.Accuracy()-75) > 0.001 {
		t.Fatalf("Accuracy = %.2f, want 75", s.Accuracy())
	}
}

func TestTimedSessionStartsOnFirstCharacter(t *testing.T) {
	base := time.Unix(200, 0)
	s := New("a very long target", 30*time.Second)
	s.Tick(base.Add(time.Hour))
	if s.Done() {
		t.Fatal("idle session should not expire")
	}
	s.Add('a', base)
	s.Tick(base.Add(29 * time.Second))
	if s.Done() {
		t.Fatal("session expired early")
	}
	s.Tick(base.Add(31 * time.Second))
	if !s.Done() || s.Reason != FinishedTime {
		t.Fatalf("timed session not finished correctly: done=%v reason=%q", s.Done(), s.Reason)
	}
	if got := s.Elapsed(base.Add(time.Minute)); got != 30*time.Second {
		t.Fatalf("Elapsed = %v, want 30s", got)
	}
}

func TestUnicodePositionsUseRunes(t *testing.T) {
	s := New("café", 0)
	now := time.Now()
	for _, r := range "café" {
		s.Add(r, now)
	}
	if s.Position() != 4 || !s.Done() {
		t.Fatalf("Position = %d, done=%v; want 4, true", s.Position(), s.Done())
	}
}
