package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"serialtypist/internal/content"
	"serialtypist/internal/progress"
	typingengine "serialtypist/internal/typing"
)

func TestNewSelectsFirstIncompleteLesson(t *testing.T) {
	lessons := []content.Lesson{
		{Name: "Home Row", Source: "001-home-row.txt", Text: "asdf"},
		{Name: "Top Row", Source: "002-top-row.txt", Text: "qwer"},
	}
	store, err := progress.Open(filepath.Join(t.TempDir(), progress.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordLesson(
		progress.LessonID(lessons[0].Source, lessons[0].Text),
		25,
		98,
		true,
		time.Now(),
	); err != nil {
		t.Fatal(err)
	}

	app := New(Config{Library: content.Library{Lessons: lessons}, Progress: store})
	if app.lessonSelection != 1 {
		t.Fatalf("lesson selection = %d, want 1", app.lessonSelection)
	}
}

func TestStartAdaptiveBuildsFocusedDrill(t *testing.T) {
	store, err := progress.Open(filepath.Join(t.TempDir(), progress.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSession(progress.SessionUpdate{
		CharacterMistakes: map[rune]int{'p': 2},
	}); err != nil {
		t.Fatal(err)
	}

	app := New(Config{
		WordCount: 5,
		Library:   content.Library{Words: []string{"apple", "berry"}},
		Progress:  store,
	})
	app.startAdaptive()

	if app.kind != kindAdaptive || app.screen != screenTyping || app.session == nil {
		t.Fatalf("adaptive state: kind=%d screen=%d session=%v", app.kind, app.screen, app.session != nil)
	}
	words := strings.Fields(string(app.session.Target))
	if len(words) != 5 {
		t.Fatalf("adaptive word count = %d, want 5: %q", len(words), app.session.Target)
	}
	for _, word := range words {
		if word != "apple" {
			t.Fatalf("adaptive target contains %q, want only apple: %q", word, app.session.Target)
		}
	}
}

func TestStartAdaptiveWithoutMistakesReturnsHome(t *testing.T) {
	store, err := progress.Open(filepath.Join(t.TempDir(), progress.FileName))
	if err != nil {
		t.Fatal(err)
	}
	app := New(Config{Progress: store})
	app.screen = screenResults
	app.session = typingengine.New("test", 0)

	app.startAdaptive()

	if app.screen != screenHome || app.session != nil || app.homeNotice == "" {
		t.Fatalf("empty adaptive state: screen=%d session=%v notice=%q", app.screen, app.session != nil, app.homeNotice)
	}
}
