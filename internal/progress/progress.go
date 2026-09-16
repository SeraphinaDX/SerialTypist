// Package progress stores per-user lesson results.
package progress

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	FileName       = "progress.toml"
	currentVersion = 1
)

// LessonRecord is the best result and attempt count for one lesson.
type LessonRecord struct {
	Completed     bool      `toml:"completed"`
	Attempts      int       `toml:"attempts"`
	BestWPM       float64   `toml:"best_wpm"`
	BestAccuracy  float64   `toml:"best_accuracy"`
	LastPracticed time.Time `toml:"last_practiced"`
}

type fileData struct {
	Version int                     `toml:"version"`
	Lessons map[string]LessonRecord `toml:"lessons"`
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
		data: fileData{Version: currentVersion, Lessons: make(map[string]LessonRecord)},
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
	if loaded.Lessons == nil {
		loaded.Lessons = make(map[string]LessonRecord)
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
	if invalidNumber(wpm) || wpm < 0 {
		return fmt.Errorf("invalid WPM %.2f", wpm)
	}
	if invalidNumber(accuracy) || accuracy < 0 || accuracy > 100 {
		return fmt.Errorf("invalid accuracy %.2f", accuracy)
	}

	previous, existed := s.data.Lessons[id]
	updated := previous
	updated.Attempts++
	if wpm > updated.BestWPM {
		updated.BestWPM = wpm
	}
	if accuracy > updated.BestAccuracy {
		updated.BestAccuracy = accuracy
	}
	updated.Completed = updated.Completed || completed
	updated.LastPracticed = practicedAt.UTC()
	s.data.Lessons[id] = updated

	if err := s.save(); err != nil {
		if existed {
			s.data.Lessons[id] = previous
		} else {
			delete(s.data.Lessons, id)
		}
		return err
	}
	return nil
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
