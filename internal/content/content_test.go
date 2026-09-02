package content

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBundledContentLoads(t *testing.T) {
	lib, err := Load("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Paragraphs) < 2 || len(lib.Lessons) < 2 || len(lib.Words) < 20 {
		t.Fatalf("unexpected built-in sizes: paragraphs=%d lessons=%d words=%d", len(lib.Paragraphs), len(lib.Lessons), len(lib.Words))
	}
	if lib.Lessons[0].Name != "Home Row" {
		t.Fatalf("first lesson = %q, want Home Row", lib.Lessons[0].Name)
	}
}

func TestLoadUserContent(t *testing.T) {
	textDir := t.TempDir()
	lessonDir := t.TempDir()
	dictionary := filepath.Join(t.TempDir(), "words.txt")
	if err := os.WriteFile(filepath.Join(textDir, "practice.txt"), []byte("First paragraph.\n\nSecond paragraph."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lessonDir, "001-custom.txt"), []byte("custom lesson text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dictionary, []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	lib, err := Load(textDir, lessonDir, dictionary)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Paragraphs) != 2 || len(lib.Lessons) != 1 || len(lib.Words) != 2 {
		t.Fatalf("unexpected user content sizes: paragraphs=%d lessons=%d words=%d", len(lib.Paragraphs), len(lib.Lessons), len(lib.Words))
	}
	if lib.Lessons[0].Name != "Custom" {
		t.Fatalf("lesson name = %q, want Custom", lib.Lessons[0].Name)
	}
}

func TestQuickTextReachesMinimum(t *testing.T) {
	lib := Library{Paragraphs: []string{"one short paragraph", "another paragraph"}}
	text := lib.QuickText(rand.New(rand.NewSource(1)), 100)
	if utf8.RuneCountInString(text) < 100 {
		t.Fatalf("quick text has %d runes, want at least 100", utf8.RuneCountInString(text))
	}
	if !strings.Contains(text, "paragraph") {
		t.Fatal("quick text did not use source paragraphs")
	}
}

func TestDictionaryTextUsesRequestedCount(t *testing.T) {
	lib := Library{Words: []string{"rose", "violet", "lily"}}
	text := lib.DictionaryText(rand.New(rand.NewSource(2)), 8)
	if got := len(strings.Fields(text)); got != 8 {
		t.Fatalf("word count = %d, want 8", got)
	}
}
