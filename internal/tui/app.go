package tui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"

	"serialtypist/internal/adaptive"
	"serialtypist/internal/content"
	"serialtypist/internal/progress"
	typingengine "serialtypist/internal/typing"
)

type Config struct {
	Duration             time.Duration
	WordCount            int
	CorrectionMode       typingengine.CorrectionMode
	LessonCorrectionMode typingengine.CorrectionMode
	MinimumAccuracy      float64
	MinimumWPM           float64
	Library              content.Library
	Progress             *progress.Store
}

type screen int

const (
	screenHome screen = iota
	screenLessons
	screenProgress
	screenTyping
	screenResults
)

type sessionKind int

const (
	kindQuick sessionKind = iota
	kindLesson
	kindDictionary
	kindAdaptive
)

type App struct {
	config Config
	rng    *rand.Rand

	screen          screen
	kind            sessionKind
	session         *typingengine.Session
	sessionTitle    string
	lessonSelection int
	lessonIndex     int
	lessonMastered  bool
	progressError   string
	homeNotice      string
	comparison      progress.Summary
	newBestWPM      bool
	width           int
	height          int
}

func New(config Config) *App {
	if config.Duration <= 0 {
		config.Duration = 60 * time.Second
	}
	if config.WordCount <= 0 {
		config.WordCount = 60
	}
	if config.CorrectionMode == "" {
		config.CorrectionMode = typingengine.CorrectionFree
	}
	if config.LessonCorrectionMode == "" {
		config.LessonCorrectionMode = typingengine.CorrectionStrict
	}
	app := &App{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
		screen: screenHome,
	}
	if next := app.nextIncompleteLesson(); next >= 0 {
		app.lessonSelection = next
	}
	return app
}

func (a *App) Run() error {
	if err := ui.Init(); err != nil {
		return fmt.Errorf("initialize gotui: %w", err)
	}
	defer ui.Close()

	a.width, a.height = ui.TerminalDimensions()
	a.render()
	events := ui.PollEvents()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if event.ID == "<C-c>" {
				return nil
			}
			if event.ID == "<Resize>" {
				resize := event.Payload.(ui.Resize)
				a.width, a.height = resize.Width, resize.Height
				ui.Clear()
				a.render()
				continue
			}
			if a.handle(event) {
				return nil
			}
			a.render()

		case now := <-ticker.C:
			if a.screen == screenTyping && a.session != nil && a.session.Started() {
				a.session.Tick(now)
				if a.session.Done() {
					a.finishSession(now)
				}
				a.render()
			}
		}
	}
}

// handle returns true when the application should quit.
func (a *App) handle(event ui.Event) bool {
	switch a.screen {
	case screenHome:
		switch event.ID {
		case "q", "Q", "1":
			a.startQuick()
		case "l", "L", "2":
			a.screen = screenLessons
		case "d", "D", "3":
			a.startDictionary()
		case "c", "C", "4":
			next := a.nextIncompleteLesson()
			if next < 0 {
				a.screen = screenLessons
			} else {
				a.lessonSelection = next
				a.startLesson(next)
			}
		case "a", "A", "5":
			a.startAdaptive()
		case "s", "S", "6":
			a.screen = screenProgress
		case "<Escape>":
			return true
		}

	case screenLessons:
		switch event.ID {
		case "<Up>", "k":
			if a.lessonSelection > 0 {
				a.lessonSelection--
			}
		case "<Down>", "j":
			if a.lessonSelection+1 < len(a.config.Library.Lessons) {
				a.lessonSelection++
			}
		case "<Enter>":
			a.startLesson(a.lessonSelection)
		case "a", "A":
			a.startAccuracyLesson(a.lessonSelection)
		case "<Escape>":
			a.screen = screenHome
		}

	case screenProgress:
		switch event.ID {
		case "a", "A", "<Enter>":
			a.startAdaptive()
		case "m", "M", "<Escape>":
			a.screen = screenHome
		}

	case screenTyping:
		switch event.ID {
		case "<Escape>":
			a.session = nil
			a.screen = screenHome
			return false
		case "<Backspace>":
			a.session.Backspace()
			return false
		case "<Space>":
			a.addRune(' ')
			return false
		}
		if event.Type == ui.KeyboardEvent && utf8.RuneCountInString(event.ID) == 1 {
			r, _ := utf8.DecodeRuneInString(event.ID)
			if unicode.IsPrint(r) || unicode.IsSpace(r) {
				a.addRune(r)
			}
		}

	case screenResults:
		switch event.ID {
		case "r", "R":
			a.restartSame()
		case "n", "N", "<Enter>":
			a.startNext()
		case "m", "M", "<Escape>":
			a.session = nil
			a.screen = screenHome
		case "s", "S":
			a.screen = screenProgress
		case "x", "X":
			a.restartStrict()
		}
	}
	return false
}

func (a *App) addRune(r rune) {
	if a.session == nil {
		return
	}
	now := time.Now()
	a.session.Add(r, now)
	if a.session.Done() {
		a.finishSession(now)
	}
}

func (a *App) finishSession(now time.Time) {
	if a.session == nil || a.screen == screenResults {
		return
	}
	a.screen = screenResults
	wpm := a.session.WPM(now)
	accuracy := a.session.Accuracy()
	if a.kind == kindLesson {
		a.lessonMastered = accuracy >= a.config.MinimumAccuracy && wpm >= a.config.MinimumWPM
	}
	if a.config.Progress == nil {
		return
	}
	mode := a.progressMode()
	a.comparison = a.config.Progress.Summary(mode, 10)
	allTime := a.config.Progress.Summary(mode, 0)
	a.newBestWPM = allTime.Sessions == 0 || wpm > allTime.BestWPM
	practicedAt := a.session.FinishedAt
	if practicedAt.IsZero() {
		practicedAt = now
	}
	update := progress.SessionUpdate{
		Mode:              mode,
		WPM:               wpm,
		Accuracy:          accuracy,
		CorrectCharacters: a.session.CorrectCharacters(),
		Attempts:          a.session.Attempts,
		Elapsed:           a.session.Elapsed(now),
		PracticedAt:       practicedAt,
		CharacterMistakes: a.session.Mistakes,
		WordMistakes:      a.session.WordMistakes,
		DecayMistakes:     a.kind == kindAdaptive,
	}
	if a.kind == kindLesson {
		lesson := a.config.Library.Lessons[a.lessonIndex]
		update.LessonID = progress.LessonID(lesson.Source, lesson.Text)
		update.CompletedLesson = a.lessonMastered
	}
	if err := a.config.Progress.RecordSession(update); err != nil {
		a.progressError = err.Error()
	}
}

func (a *App) startQuick() {
	minimum := int(a.config.Duration.Seconds()*15) + 500
	target := a.config.Library.QuickText(a.rng, minimum)
	a.resetResultState()
	a.homeNotice = ""
	a.kind = kindQuick
	a.sessionTitle = fmt.Sprintf("Quick Speed Test — %s", formatDuration(a.config.Duration))
	a.session = typingengine.NewWithMode(target, a.config.Duration, a.config.CorrectionMode)
	a.screen = screenTyping
}

func (a *App) startDictionary() {
	target := a.config.Library.DictionaryText(a.rng, a.config.WordCount)
	a.resetResultState()
	a.homeNotice = ""
	a.kind = kindDictionary
	a.sessionTitle = fmt.Sprintf("Dictionary Drill — %d words", a.config.WordCount)
	a.session = typingengine.NewWithMode(target, 0, a.config.CorrectionMode)
	a.screen = screenTyping
}

func (a *App) startAdaptive() {
	if a.config.Progress == nil {
		a.session = nil
		a.screen = screenHome
		a.homeNotice = "Adaptive practice is unavailable because progress storage is disabled."
		return
	}
	characters, words := a.config.Progress.Mistakes()
	target := adaptive.Build(a.rng, a.config.Library.Words, characters, words, a.config.WordCount)
	if target == "" {
		a.session = nil
		a.screen = screenHome
		a.homeNotice = "Complete another exercise with a few corrected mistakes to unlock adaptive practice."
		return
	}
	a.resetResultState()
	a.homeNotice = ""
	a.kind = kindAdaptive
	a.sessionTitle = fmt.Sprintf("Adaptive Practice — %d focused words", a.config.WordCount)
	a.session = typingengine.NewWithMode(target, 0, a.config.CorrectionMode)
	a.screen = screenTyping
}

func (a *App) startLesson(index int) {
	a.startLessonWithMode(index, a.config.LessonCorrectionMode)
}

func (a *App) startAccuracyLesson(index int) {
	a.startLessonWithMode(index, typingengine.CorrectionStrict)
	if a.session != nil {
		a.sessionTitle = "Accuracy " + a.sessionTitle
	}
}

func (a *App) startLessonWithMode(index int, correction typingengine.CorrectionMode) {
	if index < 0 || index >= len(a.config.Library.Lessons) {
		return
	}
	a.resetResultState()
	a.homeNotice = ""
	a.kind = kindLesson
	a.lessonIndex = index
	lesson := a.config.Library.Lessons[index]
	a.sessionTitle = fmt.Sprintf("Lesson %d/%d — %s", index+1, len(a.config.Library.Lessons), lesson.Name)
	a.session = typingengine.NewWithMode(lesson.Text, 0, correction)
	a.screen = screenTyping
}

func (a *App) restartSame() {
	if a.session == nil {
		return
	}
	a.resetResultState()
	a.session = typingengine.NewWithMode(string(a.session.Target), a.session.Limit, a.session.Correction)
	a.screen = screenTyping
}

func (a *App) restartStrict() {
	if a.session == nil {
		return
	}
	a.resetResultState()
	if !strings.HasPrefix(a.sessionTitle, "Accuracy Retry — ") {
		a.sessionTitle = "Accuracy Retry — " + a.sessionTitle
	}
	a.session = typingengine.NewWithMode(string(a.session.Target), 0, typingengine.CorrectionStrict)
	a.screen = screenTyping
}

func (a *App) startNext() {
	switch a.kind {
	case kindQuick:
		a.startQuick()
	case kindDictionary:
		a.startDictionary()
	case kindAdaptive:
		a.startAdaptive()
	case kindLesson:
		next := a.lessonIndex + 1
		if next >= len(a.config.Library.Lessons) {
			a.session = nil
			a.screen = screenHome
			return
		}
		a.lessonSelection = next
		a.startLesson(next)
	}
}

func (a *App) resetResultState() {
	a.lessonMastered = false
	a.progressError = ""
	a.comparison = progress.Summary{}
	a.newBestWPM = false
}

func (a *App) progressMode() string {
	switch a.kind {
	case kindQuick:
		return progress.ModeQuick
	case kindLesson:
		return progress.ModeLesson
	case kindDictionary:
		return progress.ModeDictionary
	case kindAdaptive:
		return progress.ModeAdaptive
	default:
		return ""
	}
}

func (a *App) lessonIDs() []string {
	ids := make([]string, len(a.config.Library.Lessons))
	for index, lesson := range a.config.Library.Lessons {
		ids[index] = progress.LessonID(lesson.Source, lesson.Text)
	}
	return ids
}

func (a *App) nextIncompleteLesson() int {
	if len(a.config.Library.Lessons) == 0 {
		return -1
	}
	if a.config.Progress == nil {
		return 0
	}
	return a.config.Progress.NextIncomplete(a.lessonIDs())
}

func (a *App) completedLessonCount() int {
	if a.config.Progress == nil {
		return 0
	}
	return a.config.Progress.CompletedCount(a.lessonIDs())
}

func (a *App) render() {
	if a.width < 56 || a.height < 18 {
		a.renderSmallTerminal()
		return
	}

	switch a.screen {
	case screenHome:
		a.renderHome()
	case screenLessons:
		a.renderLessons()
	case screenProgress:
		a.renderProgress()
	case screenTyping:
		a.renderTyping()
	case screenResults:
		a.renderResults()
	}
}

func (a *App) renderHome() {
	backdrop := a.backdrop()
	header := a.header("SERIAL TYPIST", "Accuracy first. Speed follows.")
	body := a.paragraph("Choose a mode")
	completed := a.completedLessonCount()
	next := a.nextIncompleteLesson()
	continueText := "Course mastered — review lessons"
	if next >= 0 {
		continueText = fmt.Sprintf("Lesson %d/%d — %s", next+1, len(a.config.Library.Lessons), a.config.Library.Lessons[next].Name)
	}
	adaptiveText := "No mistakes recorded yet"
	if a.config.Progress != nil {
		characters, words := a.config.Progress.Mistakes()
		if len(characters) > 0 || len(words) > 0 {
			adaptiveText = fmt.Sprintf("Focus on %d characters • %d words", len(characters), len(words))
		}
	}
	notice := ""
	if a.homeNotice != "" {
		notice = "\n[" + a.homeNotice + "](fg:coral)"
	}
	loaded := ""
	if a.height >= 21 {
		loaded = fmt.Sprintf(
			"\n\nLoaded: [ %d paragraphs • %d lessons • %d dictionary words ](fg:lightgrey)",
			len(a.config.Library.Paragraphs), len(a.config.Library.Lessons), len(a.config.Library.Words),
		)
	}
	body.Text = fmt.Sprintf(
		"[1 / Q](fg:hotpink,mod:bold)  Quick speed test     Random paragraphs for %s\n"+
			"[2 / L](fg:orchid,mod:bold)   Lessons              %d ordered • %d mastered\n"+
			"[3 / D](fg:turquoise,mod:bold)   Dictionary drill     %d randomized words\n"+
			"[4 / C](fg:turquoise,mod:bold)   Continue course      %s\n"+
			"[5 / A](fg:hotpink,mod:bold)   Adaptive practice    %s\n"+
			"[6 / S](fg:orchid,mod:bold)   Progress insights    Trends, bests, and trouble spots%s%s",
		formatDuration(a.config.Duration), len(a.config.Library.Lessons), completed,
		a.config.WordCount, continueText, adaptiveText, loaded, notice,
	)
	body.TextAlignment = ui.AlignCenter
	body.VerticalAlignment = ui.AlignMiddle
	body.SetRect(2, 4, a.width-2, a.height-3)
	footer := a.footer("1 quick • 2 lessons • 3 dictionary • 4 continue • 5 adaptive • 6 progress • Esc quit")
	ui.Render(backdrop, header, body, footer)
}

func (a *App) renderProgress() {
	backdrop := a.backdrop()
	header := a.header("PROGRESS INSIGHTS", "Practice history • personal bests • adaptive focus")
	body := a.paragraph("Your progress")
	if a.config.Progress == nil {
		body.Text = "Progress storage is unavailable for this session."
	} else {
		allTime := a.config.Progress.Summary("", 0)
		recent := a.config.Progress.Summary("", 10)
		records := a.config.Progress.Recent("", 24)
		trendWidth := a.width - 28
		if trendWidth < 8 {
			trendWidth = 8
		}
		if trendWidth > 36 {
			trendWidth = 36
		}

		var lines []string
		if allTime.Sessions == 0 {
			lines = []string{
				"[No completed sessions yet.](fg:hotpink,mod:bold)",
				"Complete an exercise to begin building your history.",
			}
		} else {
			lines = []string{
				fmt.Sprintf("All time      [%d sessions](fg:hotpink,mod:bold) • %.0f WPM avg • %.1f%% • best %.0f", allTime.Sessions, allTime.AverageWPM, allTime.AverageAccuracy, allTime.BestWPM),
				fmt.Sprintf("Recent 10    %.0f WPM avg • %.1f%% accuracy", recent.AverageWPM, recent.AverageAccuracy),
				"WPM trend    [" + sessionSparkline(records, trendWidth, func(record progress.SessionRecord) float64 { return record.WPM }) + "](fg:hotpink)",
				"Accuracy     [" + sessionSparkline(records, trendWidth, func(record progress.SessionRecord) float64 { return record.Accuracy }) + "](fg:turquoise)",
			}
			if a.height >= 23 {
				lines = append(lines, "")
				for _, mode := range []struct {
					name string
					id   string
				}{
					{name: "Quick", id: progress.ModeQuick},
					{name: "Lessons", id: progress.ModeLesson},
					{name: "Dictionary", id: progress.ModeDictionary},
					{name: "Adaptive", id: progress.ModeAdaptive},
				} {
					summary := a.config.Progress.Summary(mode.id, 0)
					lines = append(lines, fmt.Sprintf("%-12s %3d× • %3.0f WPM • %5.1f%%", mode.name, summary.Sessions, summary.AverageWPM, summary.AverageAccuracy))
				}
			}
		}
		lines = append(lines,
			"",
			"Trouble keys  "+formatTrouble(a.config.Progress.TopCharacters(5)),
			"Trouble words "+formatTrouble(a.config.Progress.TopWords(4)),
		)
		body.Text = strings.Join(lines, "\n")
	}
	body.SetRect(2, 4, a.width-2, a.height-3)
	footer := a.footer("A/Enter adaptive practice  •  M/Esc menu  •  Ctrl+C quit")
	ui.Render(backdrop, header, body, footer)
}

func (a *App) renderLessons() {
	backdrop := a.backdrop()
	header := a.header("LESSONS", "✓ mastered • ● attempted • ○ new")
	body := a.paragraph("Lesson path")
	var lines []string
	visible := a.height - 10
	if visible < 1 {
		visible = 1
	}
	start := a.lessonSelection - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(a.config.Library.Lessons) {
		end = len(a.config.Library.Lessons)
		start = end - visible
		if start < 0 {
			start = 0
		}
	}
	for index := start; index < end; index++ {
		lesson := a.config.Library.Lessons[index]
		marker := "  "
		if index == a.lessonSelection {
			marker = "❯ "
		}
		status := "○"
		details := ""
		if a.config.Progress != nil {
			id := progress.LessonID(lesson.Source, lesson.Text)
			if record, ok := a.config.Progress.Record(id); ok {
				status = "●"
				if record.Completed {
					status = "✓"
				}
				details = fmt.Sprintf(" — %.0f WPM • %.1f%% • %d×", record.BestWPM, record.BestAccuracy, record.Attempts)
			}
		}
		lines = append(lines, fmt.Sprintf("%s%s %2d. %s%s", marker, status, index+1, lesson.Name, details))
	}
	body.Text = strings.Join(lines, "\n")
	body.TitleRight = fmt.Sprintf(" %d of %d • %d mastered ", a.lessonSelection+1, len(a.config.Library.Lessons), a.completedLessonCount())
	body.SetRect(2, 4, a.width-2, a.height-3)
	footer := a.footer("↑/↓ or j/k select  •  Enter start  •  A strict accuracy lesson  •  Esc menu")
	ui.Render(backdrop, header, body, footer)
}

func (a *App) renderTyping() {
	now := time.Now()
	backdrop := a.backdrop()
	subtitle := "Free typing • correct errors with Backspace • timer starts with your first key"
	footerText := "Backspace corrects  •  Esc abandons session  •  Ctrl+C quits"
	if a.session.Correction == typingengine.CorrectionStrict {
		subtitle = "Strict correction • wrong keys do not advance • timer starts with your first key"
		footerText = "Type the highlighted key to continue  •  Esc abandons  •  Ctrl+C quits"
	}
	header := a.header(a.sessionTitle, subtitle)
	stats := widgets.NewParagraph()
	stats.Border = false
	stats.BackgroundColor = ui.NewRGBColor(16, 9, 18)
	stats.TextStyle = ui.NewStyle(ui.NewRGBColor(255, 238, 248), ui.NewRGBColor(16, 9, 18))
	stats.TextAlignment = ui.AlignCenter
	stats.Text = a.statsLine(now)
	stats.SetRect(0, 4, a.width, 7)

	pane := NewTypingPane()
	pane.Title = " Copy this text "
	pane.TitleRight = fmt.Sprintf(" %d / %d ", a.session.Position(), len(a.session.Target))
	pane.Target = a.session.Target
	pane.Typed = a.session.Typed
	pane.CorrectionRequired = a.session.NeedsCorrection()
	pane.SetRect(1, 7, a.width-1, a.height-3)
	footer := a.footer(footerText)
	ui.Render(backdrop, header, stats, pane, footer)
}

func (a *App) renderResults() {
	now := time.Now()
	backdrop := a.backdrop()
	header := a.header("RESULTS", a.sessionTitle)
	body := a.paragraph("Session complete")
	next := "new exercise"
	mastery := ""
	if a.kind == kindLesson {
		if a.lessonIndex+1 < len(a.config.Library.Lessons) {
			next = "next lesson"
		} else {
			next = "finish course"
		}
		if a.lessonMastered {
			mastery = "\nMastery       [achieved ✓](fg:turquoise,mod:bold)"
		} else {
			mastery = fmt.Sprintf(
				"\nMastery       [needs %.1f%% accuracy and %.0f WPM](fg:coral)",
				a.config.MinimumAccuracy,
				a.config.MinimumWPM,
			)
		}
		if a.progressError != "" {
			mastery += "\nProgress      [not saved: " + a.progressError + "](fg:coral)"
		}
	} else if a.kind == kindAdaptive {
		next = "new adaptive drill"
	}
	if a.progressError != "" && a.kind != kindLesson {
		mastery += "\nProgress      [not saved: " + a.progressError + "](fg:coral)"
	}
	comparison := ""
	if a.height >= 20 {
		if a.comparison.Sessions > 0 {
			comparison = fmt.Sprintf(
				"\nRecent avg    %.0f WPM • %.1f%%\nChange        [%+.0f WPM • %+.1f points](fg:orchid)",
				a.comparison.AverageWPM,
				a.comparison.AverageAccuracy,
				a.session.WPM(now)-a.comparison.AverageWPM,
				a.session.Accuracy()-a.comparison.AverageAccuracy,
			)
		}
		if a.newBestWPM {
			comparison += "\nSpeed         [new personal best ✓](fg:hotpink,mod:bold)"
		}
	}
	body.Text = fmt.Sprintf(
		"[%.0f WPM](fg:hotpink,mod:bold)\n\n"+
			"Accuracy     [%.1f%%](fg:turquoise)\n"+
			"Correct      %d characters\n"+
			"Attempts     %d\n"+
			"Errors       %d corrected • %d unresolved\n"+
			"Elapsed      %s\n"+
			"Finished by  %s%s%s\n\n"+
			"[R](fg:orchid,mod:bold) repeat   [X](fg:coral,mod:bold) strict retry   [N / Enter](fg:hotpink,mod:bold) %s   [S](fg:orchid,mod:bold) progress   [M](fg:turquoise,mod:bold) menu",
		a.session.WPM(now), a.session.Accuracy(), a.session.CorrectCharacters(),
		a.session.Attempts, a.session.CorrectedErrors, a.session.UnresolvedErrors(),
		formatDuration(a.session.Elapsed(now)), a.session.Reason, mastery, comparison, next,
	)
	body.TextAlignment = ui.AlignCenter
	body.VerticalAlignment = ui.AlignMiddle
	body.SetRect(2, 4, a.width-2, a.height-3)
	footer := a.footer("R repeat  •  X strict retry  •  N/Enter continue  •  S progress  •  M/Esc menu")
	ui.Render(backdrop, header, body, footer)
}

func (a *App) renderSmallTerminal() {
	p := a.paragraph("Terminal too small")
	p.Text = fmt.Sprintf("SerialTypist needs at least 56 columns by 18 rows.\n\nCurrent size: %d × %d\n\nResize the terminal, or press Ctrl+C to quit.", a.width, a.height)
	p.TextAlignment = ui.AlignCenter
	p.VerticalAlignment = ui.AlignMiddle
	p.SetRect(0, 0, a.width, a.height)
	ui.Render(p)
}

func (a *App) statsLine(now time.Time) string {
	timing := "Timer  waiting for first key"
	if a.session.Limit > 0 {
		timing = fmt.Sprintf("Time  %s", formatDuration(a.session.Remaining(now)))
	} else if a.session.Started() {
		timing = fmt.Sprintf("Time  %s", formatDuration(a.session.Elapsed(now)))
	}
	return fmt.Sprintf(
		"[WPM %.0f](fg:hotpink,mod:bold)    [Accuracy %.1f%%](fg:turquoise)    [Errors %d • fixed %d](fg:coral)    %s",
		a.session.WPM(now), a.session.Accuracy(), a.session.UnresolvedErrors(), a.session.CorrectedErrors, timing,
	)
}

func (a *App) header(title, subtitle string) *widgets.Paragraph {
	p := widgets.NewParagraph()
	p.Border = false
	p.BackgroundColor = ui.NewRGBColor(41, 16, 36)
	p.TextStyle = ui.NewStyle(ui.NewRGBColor(255, 158, 210), ui.NewRGBColor(41, 16, 36), ui.ModifierBold)
	p.TextAlignment = ui.AlignCenter
	p.Text = title + "\n" + subtitle
	p.SetRect(0, 0, a.width, 4)
	return p
}

func (a *App) backdrop() *widgets.Paragraph {
	p := widgets.NewParagraph()
	p.Border = false
	p.BackgroundColor = ui.NewRGBColor(16, 9, 18)
	p.SetRect(0, 0, a.width, a.height)
	return p
}

func (a *App) paragraph(title string) *widgets.Paragraph {
	p := widgets.NewParagraph()
	p.Title = " " + title + " "
	p.BorderRounded = true
	p.BackgroundColor = ui.NewRGBColor(16, 9, 18)
	p.BorderStyle = ui.NewStyle(ui.NewRGBColor(117, 64, 95))
	p.TitleStyle = ui.NewStyle(ui.NewRGBColor(255, 158, 210), ui.ColorClear, ui.ModifierBold)
	p.TextStyle = ui.NewStyle(ui.NewRGBColor(255, 238, 248), ui.NewRGBColor(16, 9, 18))
	p.PaddingLeft = 2
	p.PaddingRight = 2
	p.PaddingTop = 1
	p.PaddingBottom = 1
	return p
}

func (a *App) footer(text string) *widgets.Paragraph {
	p := widgets.NewParagraph()
	p.Border = false
	p.BackgroundColor = ui.NewRGBColor(41, 16, 36)
	p.TextStyle = ui.NewStyle(ui.NewRGBColor(205, 160, 255), ui.NewRGBColor(41, 16, 36))
	p.TextAlignment = ui.AlignCenter
	p.VerticalAlignment = ui.AlignMiddle
	p.Text = text
	p.SetRect(0, a.height-3, a.width, a.height)
	return p
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Round(time.Second).Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func sessionSparkline(records []progress.SessionRecord, width int, value func(progress.SessionRecord) float64) string {
	if len(records) == 0 || width <= 0 {
		return "—"
	}
	if len(records) > width {
		records = records[len(records)-width:]
	}
	minimum := value(records[0])
	maximum := minimum
	for _, record := range records[1:] {
		current := value(record)
		if current < minimum {
			minimum = current
		}
		if current > maximum {
			maximum = current
		}
	}
	levels := []rune("▁▂▃▄▅▆▇█")
	var result strings.Builder
	for _, record := range records {
		index := len(levels) / 2
		if maximum > minimum {
			ratio := (value(record) - minimum) / (maximum - minimum)
			index = int(ratio * float64(len(levels)-1))
		}
		result.WriteRune(levels[index])
	}
	return result.String()
}

func formatTrouble(items []progress.TroubleItem) string {
	if len(items) == 0 {
		return "none recorded"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.Join(strings.Fields(item.Text), " ")
		text = strings.NewReplacer("[", "(", "]", ")").Replace(text)
		parts = append(parts, fmt.Sprintf("%s×%d", text, item.Weight))
	}
	return strings.Join(parts, " • ")
}
