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

func TestOpenMigratesVersionTwoProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	contents := "version = 2\n\n[character_mistakes]\np = 3\n\n[word_mistakes]\npeach = 2\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if store.data.Version != 3 || len(store.History()) != 0 {
		t.Fatalf("migrated version = %d, history = %#v", store.data.Version, store.History())
	}
	characters, words := store.Mistakes()
	if characters["p"] != 3 || words["peach"] != 2 {
		t.Fatalf("migration lost adaptive data: %#v %#v", characters, words)
	}
}

func TestSessionHistorySummariesAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	updates := []SessionUpdate{
		{Mode: ModeQuick, WPM: 20, Accuracy: 90, Elapsed: time.Minute, PracticedAt: base},
		{Mode: ModeDictionary, WPM: 40, Accuracy: 100, Elapsed: 2 * time.Minute, PracticedAt: base.Add(time.Hour)},
		{Mode: ModeQuick, WPM: 60, Accuracy: 100, Elapsed: time.Minute, PracticedAt: base.Add(2 * time.Hour)},
	}
	for _, update := range updates {
		if err := store.RecordSession(update); err != nil {
			t.Fatal(err)
		}
	}

	quick := store.Summary(ModeQuick, 0)
	if quick.Sessions != 2 || quick.AverageWPM != 40 || quick.AverageAccuracy != 95 || quick.BestWPM != 60 {
		t.Fatalf("quick summary = %+v", quick)
	}
	recent := store.Summary("", 2)
	if recent.Sessions != 2 || recent.AverageWPM != 50 || recent.TotalDuration != 3*time.Minute {
		t.Fatalf("recent summary = %+v", recent)
	}

	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if history := reloaded.History(); len(history) != 3 || history[2].Mode != ModeQuick || history[2].WPM != 60 {
		t.Fatalf("reloaded history = %#v", history)
	}
}

func TestSessionHistoryIsBounded(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < maxHistory; index++ {
		store.data.History = append(store.data.History, SessionRecord{Mode: ModeQuick, WPM: float64(index)})
	}
	if err := store.RecordSession(SessionUpdate{Mode: ModeAdaptive, WPM: 999, Accuracy: 100}); err != nil {
		t.Fatal(err)
	}
	history := store.History()
	if len(history) != maxHistory || history[0].WPM != 1 || history[len(history)-1].WPM != 999 {
		t.Fatalf("bounded history has %d records, first %.0f, last %.0f", len(history), history[0].WPM, history[len(history)-1].WPM)
	}
}

func TestTopTroubleSortsByWeightThenText(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSession(SessionUpdate{
		CharacterMistakes: map[rune]int{'z': 2, 'a': 2, 'q': 5},
		WordMistakes:      map[string]int{"zebra": 1, "quiet": 4},
	}); err != nil {
		t.Fatal(err)
	}
	characters := store.TopCharacters(2)
	if len(characters) != 2 || characters[0].Text != "q" || characters[1].Text != "a" {
		t.Fatalf("top characters = %#v", characters)
	}
	words := store.TopWords(1)
	if len(words) != 1 || words[0].Text != "quiet" {
		t.Fatalf("top words = %#v", words)
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
