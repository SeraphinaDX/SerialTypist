// Package progress stores per-user lesson results and practice history.
package progress

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	FileName       = "progress.toml"
	currentVersion = 3
	maxHistory     = 200

	ModeQuick      = "quick"
	ModeLesson     = "lesson"
	ModeDictionary = "dictionary"
	ModeAdaptive   = "adaptive"
)

// LessonRecord is the best result and attempt count for one lesson.
type LessonRecord struct {
	Completed     bool      `toml:"completed"`
	Attempts      int       `toml:"attempts"`
	BestWPM       float64   `toml:"best_wpm"`
	BestAccuracy  float64   `toml:"best_accuracy"`
	LastPracticed time.Time `toml:"last_practiced"`
}

// SessionRecord is one completed exercise retained for progress insights.
type SessionRecord struct {
	Mode              string    `toml:"mode"`
	WPM               float64   `toml:"wpm"`
	Accuracy          float64   `toml:"accuracy"`
	CorrectCharacters int       `toml:"correct_characters"`
	Attempts          int       `toml:"attempts"`
	DurationSeconds   float64   `toml:"duration_seconds"`
	PracticedAt       time.Time `toml:"practiced_at"`
}

// Summary aggregates a selection of completed sessions.
type Summary struct {
	Sessions        int
	AverageWPM      float64
	AverageAccuracy float64
	BestWPM         float64
	TotalDuration   time.Duration
}

// TroubleItem is one weighted character or word used by adaptive practice.
type TroubleItem struct {
	Text   string
	Weight int
}

type fileData struct {
	Version           int                     `toml:"version"`
	Lessons           map[string]LessonRecord `toml:"lessons"`
	CharacterMistakes map[string]int          `toml:"character_mistakes"`
	WordMistakes      map[string]int          `toml:"word_mistakes"`
	History           []SessionRecord         `toml:"history"`
}

// SessionUpdate contains the learning data produced by one completed session.
// LessonID is optional because quick, dictionary, and adaptive sessions do not
// update lesson mastery.
type SessionUpdate struct {
	Mode               string
	LessonID           string
	WPM                float64
	Accuracy           float64
	CorrectCharacters int
	Attempts           int
	Elapsed            time.Duration
	CompletedLesson    bool
	PracticedAt        time.Time
	CharacterMistakes map[rune]int
	WordMistakes      map[string]int
	DecayMistakes     bool
}

// Store is an in-memory view of a progress file.
type Store struct {
	path string
	data fileData
}

// Open loads path. A missing file starts a new progress record.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("progress path is empty")
	}

	store := &Store{
		path: path,
		data: newFileData(),
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return store, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect progress %q: %w", path, err)
	}

	var loaded fileData
	if _, err := toml.DecodeFile(path, &loaded); err != nil {
		return nil, fmt.Errorf("load progress %q: %w", path, err)
	}
	if loaded.Version > currentVersion {
		return nil, fmt.Errorf("load progress %q: file version %d is newer than supported version %d", path, loaded.Version, currentVersion)
	}
	if loaded.Version == 0 {
		loaded.Version = currentVersion
	}
	loaded.Version = currentVersion
	if loaded.Lessons == nil {
		loaded.Lessons = make(map[string]LessonRecord)
	}
	if loaded.CharacterMistakes == nil {
		loaded.CharacterMistakes = make(map[string]int)
	}
	if loaded.WordMistakes == nil {
		loaded.WordMistakes = make(map[string]int)
	}
	if len(loaded.History) > maxHistory {
		loaded.History = append([]SessionRecord(nil), loaded.History[len(loaded.History)-maxHistory:]...)
	}
	store.data = loaded
	return store, nil
}

// DefaultPath returns the per-user progress file for the current platform.
func DefaultPath() (string, error) {
	home, _ := os.UserHomeDir()
	configRoot, _ := os.UserConfigDir()
	return defaultPath(
		runtime.GOOS,
		home,
		configRoot,
		os.Getenv("LOCALAPPDATA"),
		os.Getenv("XDG_STATE_HOME"),
	)
}

func defaultPath(goos, home, configRoot, localAppData, stateHome string) (string, error) {
	var root string
	switch goos {
	case "windows":
		root = localAppData
		if root == "" {
			root = configRoot
		}
		if root == "" && home != "" {
			root = filepath.Join(home, "AppData", "Local")
		}
		if root != "" {
			return filepath.Join(root, "SerialTypist", FileName), nil
		}
	case "darwin":
		root = configRoot
		if root == "" && home != "" {
			root = filepath.Join(home, "Library", "Application Support")
		}
		if root != "" {
			return filepath.Join(root, "SerialTypist", FileName), nil
		}
	default:
		root = stateHome
		if root == "" && home != "" {
			root = filepath.Join(home, ".local", "state")
		}
		if root != "" {
			return filepath.Join(root, "serialtypist", FileName), nil
		}
	}
	return "", fmt.Errorf("cannot determine per-user progress directory")
}

// LessonID returns a stable identifier. Moving a lesson directory preserves
// progress, while changing a lesson's filename or contents creates a new ID.
func LessonID(source, text string) string {
	name := filepath.Base(filepath.ToSlash(source))
	sum := sha256.Sum256([]byte(name + "\x00" + text))
	return hex.EncodeToString(sum[:12])
}

// Record returns the stored result for id.
func (s *Store) Record(id string) (LessonRecord, bool) {
	record, ok := s.data.Lessons[id]
	return record, ok
}

// RecordLesson adds an attempt, updates personal bests, and saves immediately.
func (s *Store) RecordLesson(id string, wpm, accuracy float64, completed bool, practicedAt time.Time) error {
	if id == "" {
		return fmt.Errorf("lesson ID is empty")
	}
	return s.RecordSession(SessionUpdate{
		Mode:             ModeLesson,
		LessonID:        id,
		WPM:             wpm,
		Accuracy:        accuracy,
		CompletedLesson: completed,
		PracticedAt:     practicedAt,
	})
}

// RecordSession updates lesson results and adaptive mistake weights in one
// recoverable save. Adaptive sessions set DecayMistakes so old trouble spots
// fade as the student practices them.
func (s *Store) RecordSession(update SessionUpdate) error {
	if update.Mode != "" && !validMode(update.Mode) {
		return fmt.Errorf("invalid session mode %q", update.Mode)
	}
	if update.Mode != "" || update.LessonID != "" {
		if invalidNumber(update.WPM) || update.WPM < 0 {
			return fmt.Errorf("invalid WPM %.2f", update.WPM)
		}
		if invalidNumber(update.Accuracy) || update.Accuracy < 0 || update.Accuracy > 100 {
			return fmt.Errorf("invalid accuracy %.2f", update.Accuracy)
		}
		if update.CorrectCharacters < 0 {
			return fmt.Errorf("invalid correct character count %d", update.CorrectCharacters)
		}
		if update.Attempts < 0 {
			return fmt.Errorf("invalid attempt count %d", update.Attempts)
		}
		if update.Elapsed < 0 {
			return fmt.Errorf("invalid elapsed time %s", update.Elapsed)
		}
	}
	if update.Mode == "" && update.LessonID == "" && !update.DecayMistakes && len(update.CharacterMistakes) == 0 && len(update.WordMistakes) == 0 {
		return nil
	}

	previous := cloneFileData(s.data)
	if update.DecayMistakes {
		decay(s.data.CharacterMistakes)
		decay(s.data.WordMistakes)
	}
	for character, count := range update.CharacterMistakes {
		if count > 0 {
			s.data.CharacterMistakes[string(character)] += count
		}
	}
	for word, count := range update.WordMistakes {
		if word != "" && count > 0 {
			s.data.WordMistakes[word] += count
		}
	}
	if update.LessonID != "" {
		stored := s.data.Lessons[update.LessonID]
		stored.Attempts++
		if update.WPM > stored.BestWPM {
			stored.BestWPM = update.WPM
		}
		if update.Accuracy > stored.BestAccuracy {
			stored.BestAccuracy = update.Accuracy
		}
		stored.Completed = stored.Completed || update.CompletedLesson
		stored.LastPracticed = update.PracticedAt.UTC()
		s.data.Lessons[update.LessonID] = stored
	}
	if update.Mode != "" {
		record := SessionRecord{
			Mode:              update.Mode,
			WPM:               update.WPM,
			Accuracy:          update.Accuracy,
			CorrectCharacters: update.CorrectCharacters,
			Attempts:          update.Attempts,
			DurationSeconds:   update.Elapsed.Seconds(),
			PracticedAt:       update.PracticedAt.UTC(),
		}
		s.data.History = append(s.data.History, record)
		if len(s.data.History) > maxHistory {
			s.data.History = append([]SessionRecord(nil), s.data.History[len(s.data.History)-maxHistory:]...)
		}
	}

	if err := s.save(); err != nil {
		s.data = previous
		return err
	}
	return nil
}

// Mistakes returns copies of the current character and word weights.
func (s *Store) Mistakes() (map[string]int, map[string]int) {
	characters := make(map[string]int, len(s.data.CharacterMistakes))
	for character, count := range s.data.CharacterMistakes {
		characters[character] = count
	}
	words := make(map[string]int, len(s.data.WordMistakes))
	for word, count := range s.data.WordMistakes {
		words[word] = count
	}
	return characters, words
}

// History returns a copy of completed sessions in chronological order.
func (s *Store) History() []SessionRecord {
	return append([]SessionRecord(nil), s.data.History...)
}

// Recent returns up to limit matching sessions in chronological order. An
// empty mode includes every practice mode; a non-positive limit includes all.
func (s *Store) Recent(mode string, limit int) []SessionRecord {
	matching := make([]SessionRecord, 0, len(s.data.History))
	for _, record := range s.data.History {
		if mode == "" || record.Mode == mode {
			matching = append(matching, record)
		}
	}
	if limit > 0 && len(matching) > limit {
		matching = matching[len(matching)-limit:]
	}
	return append([]SessionRecord(nil), matching...)
}

// Summary aggregates up to limit recent sessions for one mode. An empty mode
// includes every mode; a non-positive limit includes all retained history.
func (s *Store) Summary(mode string, limit int) Summary {
	records := s.Recent(mode, limit)
	var summary Summary
	for _, record := range records {
		summary.Sessions++
		summary.AverageWPM += record.WPM
		summary.AverageAccuracy += record.Accuracy
		if record.WPM > summary.BestWPM {
			summary.BestWPM = record.WPM
		}
		summary.TotalDuration += time.Duration(record.DurationSeconds * float64(time.Second))
	}
	if summary.Sessions > 0 {
		summary.AverageWPM /= float64(summary.Sessions)
		summary.AverageAccuracy /= float64(summary.Sessions)
	}
	return summary
}

// TopCharacters returns the highest current adaptive character weights.
func (s *Store) TopCharacters(limit int) []TroubleItem {
	return topTrouble(s.data.CharacterMistakes, limit)
}

// TopWords returns the highest current adaptive word weights.
func (s *Store) TopWords(limit int) []TroubleItem {
	return topTrouble(s.data.WordMistakes, limit)
}

// CompletedCount reports how many supplied lesson IDs are mastered.
func (s *Store) CompletedCount(ids []string) int {
	count := 0
	for _, id := range ids {
		if record, ok := s.data.Lessons[id]; ok && record.Completed {
			count++
		}
	}
	return count
}

// NextIncomplete returns the first unmastered lesson index, or -1.
func (s *Store) NextIncomplete(ids []string) int {
	for index, id := range ids {
		if record, ok := s.data.Lessons[id]; !ok || !record.Completed {
			return index
		}
	}
	return -1
}

func (s *Store) save() error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create progress directory %q: %w", directory, err)
	}

	temporary, err := os.CreateTemp(directory, ".serialtypist-progress-*.toml")
	if err != nil {
		return fmt.Errorf("create temporary progress file: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanup := func() {
		temporary.Close()
		os.Remove(temporaryPath)
	}

	if err := temporary.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temporary progress file: %w", err)
	}
	if err := toml.NewEncoder(temporary).Encode(s.data); err != nil {
		cleanup()
		return fmt.Errorf("encode progress: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync progress: %w", err)
	}
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryPath)
		return fmt.Errorf("close progress: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		os.Remove(temporaryPath)
		return fmt.Errorf("replace progress %q: %w", s.path, err)
	}
	return nil
}

func invalidNumber(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0)
}

func newFileData() fileData {
	return fileData{
		Version:           currentVersion,
		Lessons:           make(map[string]LessonRecord),
		CharacterMistakes: make(map[string]int),
		WordMistakes:      make(map[string]int),
		History:           make([]SessionRecord, 0),
	}
}

func cloneFileData(source fileData) fileData {
	cloned := newFileData()
	for id, record := range source.Lessons {
		cloned.Lessons[id] = record
	}
	for character, count := range source.CharacterMistakes {
		cloned.CharacterMistakes[character] = count
	}
	for word, count := range source.WordMistakes {
		cloned.WordMistakes[word] = count
	}
	cloned.History = append(cloned.History, source.History...)
	return cloned
}

func decay(weights map[string]int) {
	for value, count := range weights {
		count /= 2
		if count == 0 {
			delete(weights, value)
		} else {
			weights[value] = count
		}
	}
}

func validMode(mode string) bool {
	switch mode {
	case ModeQuick, ModeLesson, ModeDictionary, ModeAdaptive:
		return true
	default:
		return false
	}
}

func topTrouble(weights map[string]int, limit int) []TroubleItem {
	items := make([]TroubleItem, 0, len(weights))
	for text, weight := range weights {
		if text != "" && weight > 0 {
			items = append(items, TroubleItem{Text: text, Weight: weight})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Weight == items[j].Weight {
			return items[i].Text < items[j].Text
		}
		return items[i].Weight > items[j].Weight
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}
