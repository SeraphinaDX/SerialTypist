package main

import (
	"flag"
	"fmt"
	"os"

	"serialtypist/internal/config"
	"serialtypist/internal/content"
	"serialtypist/internal/progress"
	"serialtypist/internal/tui"
	typingengine "serialtypist/internal/typing"
)

const version = "0.6.0"

func main() {
	defaults := config.Defaults()
	var (
		configPath          string
		textDir             string
		lessonDir           string
		dictionary          string
		progressPath        string
		duration            = defaults.Duration
		wordCount           = defaults.DictionaryWords
		correctionMode      = defaults.CorrectionMode
		lessonCorrectionMode = defaults.LessonCorrectionMode
		showVersion         bool
	)

	flag.StringVar(&configPath, "config", "", "TOML configuration file")
	flag.StringVar(&textDir, "texts", "", "directory of UTF-8 .txt files for quick tests")
	flag.StringVar(&lessonDir, "lessons", "", "directory of ordered UTF-8 .txt lesson files")
	flag.StringVar(&dictionary, "dictionary", "", "dictionary file (one word per line or whitespace separated)")
	flag.StringVar(&progressPath, "progress", "", "lesson progress file (defaults to the per-user state directory)")
	flag.DurationVar(&duration, "duration", defaults.Duration, "quick-test duration, such as 30s or 2m")
	flag.IntVar(&wordCount, "words", defaults.DictionaryWords, "number of words in a dictionary drill")
	flag.StringVar(&correctionMode, "correction", defaults.CorrectionMode, "correction mode for tests and drills: free or strict")
	flag.StringVar(&lessonCorrectionMode, "lesson-correction", defaults.LessonCorrectionMode, "lesson correction mode: free or strict")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("SerialTypist %s\n", version)
		return
	}

	settings, _, err := config.Load(configPath)
	if err != nil {
		exitError(err.Error())
	}

	setFlags := make(map[string]bool)
	flag.Visit(func(current *flag.Flag) {
		setFlags[current.Name] = true
	})
	if setFlags["texts"] {
		settings.TextDir = textDir
	}
	if setFlags["lessons"] {
		settings.LessonDir = lessonDir
	}
	if setFlags["dictionary"] {
		settings.Dictionary = dictionary
	}
	if setFlags["duration"] {
		settings.Duration = duration
	}
	if setFlags["words"] {
		settings.DictionaryWords = wordCount
	}
	if setFlags["correction"] {
		settings.CorrectionMode = correctionMode
	}
	if setFlags["lesson-correction"] {
		settings.LessonCorrectionMode = lessonCorrectionMode
	}

	if settings.Duration <= 0 {
		exitError("duration must be greater than zero")
	}
	if settings.DictionaryWords <= 0 {
		exitError("words must be greater than zero")
	}
	if !config.ValidCorrectionMode(settings.CorrectionMode) {
		exitError("correction must be free or strict")
	}
	if !config.ValidCorrectionMode(settings.LessonCorrectionMode) {
		exitError("lesson-correction must be free or strict")
	}

	library, err := content.Load(settings.TextDir, settings.LessonDir, settings.Dictionary)
	if err != nil {
		exitError(err.Error())
	}
	if progressPath == "" {
		progressPath, err = progress.DefaultPath()
		if err != nil {
			exitError(err.Error())
		}
	}
	progressStore, err := progress.Open(progressPath)
	if err != nil {
		exitError(err.Error())
	}

	app := tui.New(tui.Config{
		Duration:             settings.Duration,
		WordCount:            settings.DictionaryWords,
		CorrectionMode:       typingengine.CorrectionMode(settings.CorrectionMode),
		LessonCorrectionMode: typingengine.CorrectionMode(settings.LessonCorrectionMode),
		MinimumAccuracy:      settings.MinimumAccuracy,
		MinimumWPM:           settings.MinimumWPM,
		Library:              library,
		Progress:             progressStore,
	})
	if err := app.Run(); err != nil {
		exitError(err.Error())
	}
}

func exitError(message string) {
	fmt.Fprintf(os.Stderr, "serialtypist: %s\n", message)
	os.Exit(1)
}
