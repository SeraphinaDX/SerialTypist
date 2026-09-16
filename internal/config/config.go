package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const FileName = "serialtypist.toml"

// Settings contains the resolved runtime configuration.
type Settings struct {
	TextDir         string
	LessonDir       string
	Dictionary      string
	Duration        time.Duration
	DictionaryWords int
	MinimumAccuracy float64
	MinimumWPM      float64
}

type diskConfig struct {
	Content struct {
		Texts      string `toml:"texts"`
		Lessons    string `toml:"lessons"`
		Dictionary string `toml:"dictionary"`
	} `toml:"content"`
	Practice struct {
		Duration        string `toml:"duration"`
		DictionaryWords int    `toml:"dictionary_words"`
	} `toml:"practice"`
	Mastery struct {
		MinimumAccuracy float64 `toml:"minimum_accuracy"`
		MinimumWPM      float64 `toml:"minimum_wpm"`
	} `toml:"mastery"`
}

// Defaults returns settings that use bundled content.
func Defaults() Settings {
	return Settings{
		Duration:        60 * time.Second,
		DictionaryWords: 60,
		MinimumAccuracy: 95,
		MinimumWPM:      20,
	}
}

// Load loads an explicit configuration file, or searches the standard
// locations when explicitPath is empty. It returns the path that was loaded,
// or an empty string when defaults are in use.
func Load(explicitPath string) (Settings, string, error) {
	if explicitPath != "" {
		settings, path, err := loadFile(explicitPath)
		return settings, path, err
	}

	candidates := []string{FileName}
	if root, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, userConfigPath(root, runtime.GOOS))
	}
	if shared := systemConfigPath(os.Getenv("ProgramData"), runtime.GOOS); shared != "" {
		candidates = append(candidates, shared)
	}

	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		switch {
		case err == nil:
			settings, path, loadErr := loadFile(candidate)
			return settings, path, loadErr
		case os.IsNotExist(err):
			continue
		default:
			return Settings{}, "", fmt.Errorf("inspect config %q: %w", candidate, err)
		}
	}

	return Defaults(), "", nil
}

func loadFile(path string) (Settings, string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return Settings{}, "", fmt.Errorf("resolve config path %q: %w", path, err)
	}

	var stored diskConfig
	metadata, err := toml.DecodeFile(absolutePath, &stored)
	if err != nil {
		return Settings{}, "", fmt.Errorf("load config %q: %w", absolutePath, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		return Settings{}, "", fmt.Errorf(
			"load config %q: unknown setting(s): %s",
			absolutePath,
			strings.Join(keys, ", "),
		)
	}

	settings := Defaults()
	baseDir := filepath.Dir(absolutePath)
	settings.TextDir = resolveContentPath(baseDir, stored.Content.Texts)
	settings.LessonDir = resolveContentPath(baseDir, stored.Content.Lessons)
	settings.Dictionary = resolveContentPath(baseDir, stored.Content.Dictionary)

	if metadata.IsDefined("practice", "duration") {
		parsed, parseErr := time.ParseDuration(stored.Practice.Duration)
		if parseErr != nil {
			return Settings{}, "", fmt.Errorf(
				"load config %q: invalid practice.duration %q: %w",
				absolutePath,
				stored.Practice.Duration,
				parseErr,
			)
		}
		if parsed <= 0 {
			return Settings{}, "", fmt.Errorf("load config %q: practice.duration must be greater than zero", absolutePath)
		}
		settings.Duration = parsed
	}
	if metadata.IsDefined("practice", "dictionary_words") {
		if stored.Practice.DictionaryWords <= 0 {
			return Settings{}, "", fmt.Errorf("load config %q: practice.dictionary_words must be greater than zero", absolutePath)
		}
		settings.DictionaryWords = stored.Practice.DictionaryWords
	}
	if metadata.IsDefined("mastery", "minimum_accuracy") {
		if invalidNumber(stored.Mastery.MinimumAccuracy) || stored.Mastery.MinimumAccuracy < 0 || stored.Mastery.MinimumAccuracy > 100 {
			return Settings{}, "", fmt.Errorf("load config %q: mastery.minimum_accuracy must be between 0 and 100", absolutePath)
		}
		settings.MinimumAccuracy = stored.Mastery.MinimumAccuracy
	}
	if metadata.IsDefined("mastery", "minimum_wpm") {
		if invalidNumber(stored.Mastery.MinimumWPM) || stored.Mastery.MinimumWPM < 0 {
			return Settings{}, "", fmt.Errorf("load config %q: mastery.minimum_wpm must be zero or greater", absolutePath)
		}
		settings.MinimumWPM = stored.Mastery.MinimumWPM
	}

	return settings, absolutePath, nil
}

func invalidNumber(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0)
}

func userConfigPath(root, goos string) string {
	directory := "serialtypist"
	if goos == "windows" {
		directory = "SerialTypist"
	}
	return filepath.Join(root, directory, FileName)
}

func systemConfigPath(programData, goos string) string {
	if goos != "windows" || programData == "" {
		return ""
	}
	return filepath.Join(programData, "SerialTypist", FileName)
}

func resolveContentPath(baseDir, configured string) string {
	if configured == "" || filepath.IsAbs(configured) {
		return configured
	}
	return filepath.Clean(filepath.Join(baseDir, configured))
}
