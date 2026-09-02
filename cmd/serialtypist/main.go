package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"serialtypist/internal/content"
	"serialtypist/internal/tui"
)

const version = "0.1.0"

func main() {
	var (
		textDir     string
		lessonDir   string
		dictionary  string
		duration    time.Duration
		wordCount   int
		showVersion bool
	)

	flag.StringVar(&textDir, "texts", "", "directory of UTF-8 .txt files for quick tests")
	flag.StringVar(&lessonDir, "lessons", "", "directory of ordered UTF-8 .txt lesson files")
	flag.StringVar(&dictionary, "dictionary", "", "dictionary file (one word per line or whitespace separated)")
	flag.DurationVar(&duration, "duration", 60*time.Second, "quick-test duration, such as 30s or 2m")
	flag.IntVar(&wordCount, "words", 60, "number of words in a dictionary drill")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("SerialTypist %s\n", version)
		return
	}
	if duration <= 0 {
		exitError("duration must be greater than zero")
	}
	if wordCount <= 0 {
		exitError("words must be greater than zero")
	}

	library, err := content.Load(textDir, lessonDir, dictionary)
	if err != nil {
		exitError(err.Error())
	}

	app := tui.New(tui.Config{Duration: duration, WordCount: wordCount, Library: library})
	if err := app.Run(); err != nil {
		exitError(err.Error())
	}
}

func exitError(message string) {
	fmt.Fprintf(os.Stderr, "serialtypist: %s\n", message)
	os.Exit(1)
}
