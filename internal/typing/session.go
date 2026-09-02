package typing

import "time"

// FinishReason identifies why a typing session stopped.
type FinishReason string

const (
	FinishedTarget FinishReason = "target completed"
	FinishedTime   FinishReason = "time expired"
)

// Session holds the state and statistics for one typing exercise.
type Session struct {
	Target          []rune
	Typed           []rune
	Limit           time.Duration
	StartedAt       time.Time
	FinishedAt      time.Time
	Attempts        int
	CorrectAttempts int
	Reason          FinishReason
}

func New(target string, limit time.Duration) *Session {
	return &Session{Target: []rune(target), Limit: limit}
}

func (s *Session) Position() int { return len(s.Typed) }

func (s *Session) Started() bool { return !s.StartedAt.IsZero() }

func (s *Session) Done() bool { return !s.FinishedAt.IsZero() }

// Add records one printable rune at the current position.
func (s *Session) Add(r rune, now time.Time) {
	if s.Done() || len(s.Typed) >= len(s.Target) {
		return
	}
	if !s.Started() {
		s.StartedAt = now
	}
	s.Attempts++
	if r == s.Target[len(s.Typed)] {
		s.CorrectAttempts++
	}
	s.Typed = append(s.Typed, r)
	if len(s.Typed) == len(s.Target) {
		s.finish(now, FinishedTarget)
	}
}

// Backspace removes the most recently typed rune. Attempt history is retained
// so corrected mistakes still affect accuracy.
func (s *Session) Backspace() {
	if s.Done() || len(s.Typed) == 0 {
		return
	}
	s.Typed = s.Typed[:len(s.Typed)-1]
}

// Tick updates a timed session. The timer begins with the first typed rune.
func (s *Session) Tick(now time.Time) {
	if s.Done() || !s.Started() || s.Limit <= 0 {
		return
	}
	if now.Sub(s.StartedAt) >= s.Limit {
		s.finish(s.StartedAt.Add(s.Limit), FinishedTime)
	}
}

func (s *Session) finish(now time.Time, reason FinishReason) {
	if s.Done() {
		return
	}
	s.FinishedAt = now
	s.Reason = reason
}

func (s *Session) Elapsed(now time.Time) time.Duration {
	if !s.Started() {
		return 0
	}
	end := now
	if s.Done() {
		end = s.FinishedAt
	}
	elapsed := end.Sub(s.StartedAt)
	if elapsed < 0 {
		return 0
	}
	if s.Limit > 0 && elapsed > s.Limit {
		return s.Limit
	}
	return elapsed
}

func (s *Session) Remaining(now time.Time) time.Duration {
	if s.Limit <= 0 {
		return 0
	}
	remaining := s.Limit - s.Elapsed(now)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CorrectCharacters counts currently typed runes that match their target.
func (s *Session) CorrectCharacters() int {
	correct := 0
	for i, r := range s.Typed {
		if i < len(s.Target) && r == s.Target[i] {
			correct++
		}
	}
	return correct
}

func (s *Session) CurrentErrors() int { return len(s.Typed) - s.CorrectCharacters() }

func (s *Session) Accuracy() float64 {
	if s.Attempts == 0 {
		return 100
	}
	return float64(s.CorrectAttempts) / float64(s.Attempts) * 100
}

// WPM uses the standard definition of five correct characters per word.
func (s *Session) WPM(now time.Time) float64 {
	elapsed := s.Elapsed(now)
	if elapsed < time.Second {
		return 0
	}
	minutes := elapsed.Minutes()
	if minutes <= 0 {
		return 0
	}
	return float64(s.CorrectCharacters()) / 5 / minutes
}

func (s *Session) Progress() float64 {
	if len(s.Target) == 0 {
		return 1
	}
	return float64(len(s.Typed)) / float64(len(s.Target))
}
