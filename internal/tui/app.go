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

	"serialtypist/internal/content"
	typingengine "serialtypist/internal/typing"
)

type Config struct {
	Duration  time.Duration
	WordCount int
	Library   content.Library
}

type screen int

const (
	screenHome screen = iota
	screenLessons
	screenTyping
	screenResults
)

type sessionKind int

const (
	kindQuick sessionKind = iota
	kindLesson
	kindDictionary
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
	return &App{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
		screen: screenHome,
	}
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
					a.screen = screenResults
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
		case "<Escape>":
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
		}
	}
	return false
}

func (a *App) addRune(r rune) {
	if a.session == nil {
		return
	}
	a.session.Add(r, time.Now())
	if a.session.Done() {
		a.screen = screenResults
	}
}

func (a *App) startQuick() {
	minimum := int(a.config.Duration.Seconds()*15) + 500
	target := a.config.Library.QuickText(a.rng, minimum)
	a.kind = kindQuick
	a.sessionTitle = fmt.Sprintf("Quick Speed Test — %s", formatDuration(a.config.Duration))
	a.session = typingengine.New(target, a.config.Duration)
	a.screen = screenTyping
}

func (a *App) startDictionary() {
	target := a.config.Library.DictionaryText(a.rng, a.config.WordCount)
	a.kind = kindDictionary
	a.sessionTitle = fmt.Sprintf("Dictionary Drill — %d words", a.config.WordCount)
	a.session = typingengine.New(target, 0)
	a.screen = screenTyping
}

func (a *App) startLesson(index int) {
	if index < 0 || index >= len(a.config.Library.Lessons) {
		return
	}
	a.kind = kindLesson
	a.lessonIndex = index
	lesson := a.config.Library.Lessons[index]
	a.sessionTitle = fmt.Sprintf("Lesson %d/%d — %s", index+1, len(a.config.Library.Lessons), lesson.Name)
	a.session = typingengine.New(lesson.Text, 0)
	a.screen = screenTyping
}

func (a *App) restartSame() {
	if a.session == nil {
		return
	}
	a.session = typingengine.New(string(a.session.Target), a.session.Limit)
	a.screen = screenTyping
}

func (a *App) startNext() {
	switch a.kind {
	case kindQuick:
		a.startQuick()
	case kindDictionary:
		a.startDictionary()
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
	body.Text = fmt.Sprintf(
		"[1 / Q](fg:hotpink,mod:bold)  Quick speed test     Random paragraphs for %s\n\n"+
			"[2 / L](fg:orchid,mod:bold)   Lessons              %d ordered lessons\n\n"+
			"[3 / D](fg:turquoise,mod:bold)   Dictionary drill     %d randomized words\n\n\n"+
			"Loaded: [ %d paragraphs • %d lessons • %d dictionary words ](fg:lightgrey)",
		formatDuration(a.config.Duration), len(a.config.Library.Lessons), a.config.WordCount,
		len(a.config.Library.Paragraphs), len(a.config.Library.Lessons), len(a.config.Library.Words),
	)
	body.TextAlignment = ui.AlignCenter
	body.VerticalAlignment = ui.AlignMiddle
	body.SetRect(2, 4, a.width-2, a.height-3)
	footer := a.footer("Q/1 quick test  •  L/2 lessons  •  D/3 dictionary  •  Esc/Ctrl+C quit")
	ui.Render(backdrop, header, body, footer)
}

func (a *App) renderLessons() {
	backdrop := a.backdrop()
	header := a.header("LESSONS", "Up/Down selects • Enter begins")
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
	for i := start; i < end; i++ {
		lesson := a.config.Library.Lessons[i]
		marker := "  "
		if i == a.lessonSelection {
			marker = "❯ "
		}
		lines = append(lines, fmt.Sprintf("%s%2d. %s", marker, i+1, lesson.Name))
	}
	body.Text = strings.Join(lines, "\n")
	body.TitleRight = fmt.Sprintf(" %d of %d ", a.lessonSelection+1, len(a.config.Library.Lessons))
	body.SetRect(2, 4, a.width-2, a.height-3)
	footer := a.footer("↑/↓ or j/k select  •  Enter start  •  Esc menu  •  Ctrl+C quit")
	ui.Render(backdrop, header, body, footer)
}

func (a *App) renderTyping() {
	now := time.Now()
	backdrop := a.backdrop()
	header := a.header(a.sessionTitle, "Type the highlighted text • timer starts with your first key")
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
	pane.SetRect(1, 7, a.width-1, a.height-3)
	footer := a.footer("Backspace corrects  •  Esc abandons session  •  Ctrl+C quits")
	ui.Render(backdrop, header, stats, pane, footer)
}

func (a *App) renderResults() {
	now := time.Now()
	backdrop := a.backdrop()
	header := a.header("RESULTS", a.sessionTitle)
	body := a.paragraph("Session complete")
	next := "new exercise"
	if a.kind == kindLesson {
		if a.lessonIndex+1 < len(a.config.Library.Lessons) {
			next = "next lesson"
		} else {
			next = "finish course"
		}
	}
	body.Text = fmt.Sprintf(
		"[%.0f WPM](fg:hotpink,mod:bold)\n\n"+
			"Accuracy     [%.1f%%](fg:turquoise)\n"+
			"Correct      %d characters\n"+
			"Attempts     %d\n"+
			"Elapsed      %s\n"+
			"Finished by  %s\n\n"+
			"[R](fg:orchid,mod:bold) repeat   [N / Enter](fg:hotpink,mod:bold) %s   [M](fg:turquoise,mod:bold) menu",
		a.session.WPM(now), a.session.Accuracy(), a.session.CorrectCharacters(),
		a.session.Attempts, formatDuration(a.session.Elapsed(now)), a.session.Reason, next,
	)
	body.TextAlignment = ui.AlignCenter
	body.VerticalAlignment = ui.AlignMiddle
	body.SetRect(2, 4, a.width-2, a.height-3)
	footer := a.footer("R repeat  •  N/Enter continue  •  M/Esc menu  •  Ctrl+C quit")
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
		"[WPM %.0f](fg:hotpink,mod:bold)    [Accuracy %.1f%%](fg:turquoise)    [Errors %d](fg:coral)    %s",
		a.session.WPM(now), a.session.Accuracy(), a.session.CurrentErrors(), timing,
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
