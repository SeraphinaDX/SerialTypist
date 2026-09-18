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

func TestFinishSessionRecordsHistoryAndComparison(t *testing.T) {
	store, err := progress.Open(filepath.Join(t.TempDir(), progress.FileName))
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	if err := store.RecordSession(progress.SessionUpdate{
		Mode:        progress.ModeQuick,
		WPM:         20,
		Accuracy:    90,
		Elapsed:     time.Minute,
		PracticedAt: base.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	app := New(Config{Progress: store})
	app.kind = kindQuick
	app.screen = screenTyping
	app.session = typingengine.New("aaaaa", 0)
	for index := 0; index < 5; index++ {
		app.session.Add('a', base.Add(time.Duration(index)*15*time.Second))
	}
	app.finishSession(base.Add(time.Minute))

	if app.screen != screenResults || app.comparison.Sessions != 1 || app.comparison.AverageWPM != 20 {
		t.Fatalf("result comparison = %+v, screen = %d", app.comparison, app.screen)
	}
	history := store.History()
	if len(history) != 2 || history[1].Mode != progress.ModeQuick || history[1].DurationSeconds != 60 {
		t.Fatalf("history = %#v", history)
	}
}

func TestProgressModeNames(t *testing.T) {
	app := New(Config{})
	tests := []struct {
		kind sessionKind
		want string
	}{
		{kind: kindQuick, want: progress.ModeQuick},
		{kind: kindLesson, want: progress.ModeLesson},
		{kind: kindDictionary, want: progress.ModeDictionary},
		{kind: kindAdaptive, want: progress.ModeAdaptive},
	}
	for _, test := range tests {
		app.kind = test.kind
		if got := app.progressMode(); got != test.want {
			t.Fatalf("progressMode(%d) = %q, want %q", test.kind, got, test.want)
		}
	}
}

func TestSessionSparkline(t *testing.T) {
	records := []progress.SessionRecord{{WPM: 10}, {WPM: 20}, {WPM: 30}}
	got := sessionSparkline(records, 3, func(record progress.SessionRecord) float64 { return record.WPM })
	if got != "▁▄█" {
		t.Fatalf("sessionSparkline() = %q", got)
	}
}
