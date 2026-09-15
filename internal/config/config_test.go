package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUserConfigPath(t *testing.T) {
	root := filepath.Join("home", "config")
	tests := []struct {
		name string
		goos string
		dir  string
	}{
		{name: "Windows", goos: "windows", dir: "SerialTypist"},
		{name: "Linux", goos: "linux", dir: "serialtypist"},
		{name: "macOS", goos: "darwin", dir: "serialtypist"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := filepath.Join(root, test.dir, FileName)
			if got := userConfigPath(root, test.goos); got != want {
				t.Fatalf("userConfigPath() = %q, want %q", got, want)
			}
		})
	}
}

func TestSystemConfigPath(t *testing.T) {
	root := filepath.Join("shared", "config")
	want := filepath.Join(root, "SerialTypist", FileName)
	if got := systemConfigPath(root, "windows"); got != want {
		t.Fatalf("systemConfigPath() = %q, want %q", got, want)
	}
	if got := systemConfigPath(root, "linux"); got != "" {
		t.Fatalf("non-Windows systemConfigPath() = %q, want empty", got)
	}
}

func TestLoadFileResolvesRelativeContentPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	contents := `[content]
texts = "texts"
lessons = "course/lessons"
dictionary = "words.txt"

[practice]
duration = "45s"
dictionary_words = 75
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	settings, loadedPath, err := loadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loadedPath != path {
		t.Fatalf("loaded path = %q, want %q", loadedPath, path)
	}
	if settings.TextDir != filepath.Join(dir, "texts") {
		t.Fatalf("text directory = %q", settings.TextDir)
	}
	if settings.LessonDir != filepath.Join(dir, "course", "lessons") {
		t.Fatalf("lesson directory = %q", settings.LessonDir)
	}
	if settings.Dictionary != filepath.Join(dir, "words.txt") {
		t.Fatalf("dictionary = %q", settings.Dictionary)
	}
	if settings.Duration != 45*time.Second {
		t.Fatalf("duration = %s", settings.Duration)
	}
	if settings.DictionaryWords != 75 {
		t.Fatalf("dictionary words = %d", settings.DictionaryWords)
	}
}

func TestLoadFileRejectsUnknownSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("[practice]\nduraton = \"30s\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := loadFile(path)
	if err == nil || !strings.Contains(err.Error(), "practice.duraton") {
		t.Fatalf("expected unknown-setting error, got %v", err)
	}
}

func TestLoadFileRejectsInvalidPracticeValues(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "duration", content: "[practice]\nduration = \"forever\"\n"},
		{name: "word count", content: "[practice]\ndictionary_words = 0\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), FileName)
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := loadFile(path); err == nil {
				t.Fatal("expected invalid practice setting to fail")
			}
		})
	}
}
