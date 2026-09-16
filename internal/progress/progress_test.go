package progress

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultPathByPlatform(t *testing.T) {
	tests := []struct {
		name      string
		goos      string
		home      string
		config    string
		localData string
		state     string
		want      string
	}{
		{
			name: "Windows LocalAppData", goos: "windows", home: `C:\Users\Student`,
			config: `C:\Users\Student\AppData\Roaming`, localData: `C:\Users\Student\AppData\Local`,
			want: filepath.Join(`C:\Users\Student\AppData\Local`, "SerialTypist", FileName),
		},
		{
			name: "Linux XDG state", goos: "linux", home: "/home/student", state: "/state",
			want: filepath.Join("/state", "serialtypist", FileName),
		},
		{
			name: "Linux fallback", goos: "linux", home: "/home/student",
			want: filepath.Join("/home/student", ".local", "state", "serialtypist", FileName),
		},
		{
			name: "macOS", goos: "darwin", home: "/Users/student", config: "/Users/student/Library/Application Support",
			want: filepath.Join("/Users/student/Library/Application Support", "SerialTypist", FileName),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := defaultPath(test.goos, test.home, test.config, test.localData, test.state)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("defaultPath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRecordSessionTracksAndDecaysMistakes(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSession(SessionUpdate{
		CharacterMistakes: map[rune]int{'a': 4, 'é': 1},
		WordMistakes:      map[string]int{"café": 3},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSession(SessionUpdate{DecayMistakes: true}); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	characters, words := reloaded.Mistakes()
	if characters["a"] != 2 || characters["é"] != 0 || words["café"] != 1 {
		t.Fatalf("decayed characters = %#v, words = %#v", characters, words)
	}
}

func TestOpenMigratesVersionOneProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	contents := "version = 1\n\n[lessons.example]\ncompleted = true\nattempts = 1\nbest_wpm = 20.0\nbest_accuracy = 98.0\nlast_practiced = 2026-09-16T12:00:00Z\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if record, ok := store.Record("example"); !ok || !record.Completed {
		t.Fatalf("migrated record = %+v, present=%v", record, ok)
	}
	characters, words := store.Mistakes()
	if len(characters) != 0 || len(words) != 0 {
		t.Fatalf("new mistake maps should be empty: %#v %#v", characters, words)
	}
}

func TestRecordLessonPersistsBestResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	id := LessonID("001-home-row.txt", "asdf jkl;")
	now := time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC)

	if err := store.RecordLesson(id, 18, 96, false, now); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordLesson(id, 24, 94, true, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	record, ok := reloaded.Record(id)
	if !ok {
		t.Fatal("saved lesson record is missing")
	}
	if record.Attempts != 2 || !record.Completed {
		t.Fatalf("record = %+v", record)
	}
	if record.BestWPM != 24 || record.BestAccuracy != 96 {
		t.Fatalf("personal bests = %.1f WPM, %.1f%%", record.BestWPM, record.BestAccuracy)
	}
}

func TestRecordLessonRejectsEmptyID(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordLesson("", 20, 95, true, time.Now()); err == nil {
		t.Fatal("RecordLesson() accepted an empty lesson ID")
	}
}

func TestNextIncompleteAndCompletedCount(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"one", "two", "three"}
	if err := store.RecordLesson(ids[0], 25, 98, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := store.CompletedCount(ids); got != 1 {
		t.Fatalf("CompletedCount() = %d, want 1", got)
	}
	if got := store.NextIncomplete(ids); got != 1 {
		t.Fatalf("NextIncomplete() = %d, want 1", got)
	}
}

func TestLessonIDChangesWithFilenameOrText(t *testing.T) {
	base := LessonID("001-home-row.txt", "asdf")
	if base != LessonID("/another/directory/001-home-row.txt", "asdf") {
		t.Fatal("moving a lesson should preserve its ID")
	}
	if base == LessonID("002-home-row.txt", "asdf") {
		t.Fatal("renaming a lesson should change its ID")
	}
	if base == LessonID("001-home-row.txt", "jkl;") {
		t.Fatal("changing lesson text should change its ID")
	}
}
