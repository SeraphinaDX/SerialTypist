package tui

import (
	"path/filepath"
	"testing"
	"time"

	"serialtypist/internal/content"
	"serialtypist/internal/progress"
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
