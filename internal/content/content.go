package content

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:embed builtin/texts/*.txt builtin/lessons/*.txt builtin/dictionary.txt
var builtins embed.FS

// Lesson is one ordered typing lesson. The file name determines its order.
type Lesson struct {
	Name   string
	Text   string
	Source string
}

// Library contains all text that can be used by the tutor.
type Library struct {
	Paragraphs []string
	Lessons    []Lesson
	Words      []string
}

// Load loads user-provided content. An empty path selects the bundled content
// for that category.
func Load(textDir, lessonDir, dictionaryFile string) (Library, error) {
	var lib Library
	var err error

	if textDir == "" {
		lib.Paragraphs, err = paragraphsFromFS(builtins, "builtin/texts")
	} else {
		lib.Paragraphs, err = paragraphsFromDir(textDir)
	}
	if err != nil {
		return Library{}, fmt.Errorf("load speed-test text: %w", err)
	}
	if len(lib.Paragraphs) == 0 {
		return Library{}, fmt.Errorf("no non-empty paragraphs found in speed-test text")
	}

	if lessonDir == "" {
		lib.Lessons, err = lessonsFromFS(builtins, "builtin/lessons")
	} else {
		lib.Lessons, err = lessonsFromDir(lessonDir)
	}
	if err != nil {
		return Library{}, fmt.Errorf("load lessons: %w", err)
	}
	if len(lib.Lessons) == 0 {
		return Library{}, fmt.Errorf("no non-empty lesson files found")
	}

	if dictionaryFile == "" {
		f, openErr := builtins.Open("builtin/dictionary.txt")
		if openErr != nil {
			return Library{}, openErr
		}
		defer f.Close()
		lib.Words, err = readWords(f)
	} else {
		f, openErr := os.Open(dictionaryFile)
		if openErr != nil {
			return Library{}, fmt.Errorf("open dictionary %q: %w", dictionaryFile, openErr)
		}
		defer f.Close()
		lib.Words, err = readWords(f)
	}
	if err != nil {
		return Library{}, fmt.Errorf("load dictionary: %w", err)
	}
	if len(lib.Words) == 0 {
		return Library{}, fmt.Errorf("dictionary contains no words")
	}

	return lib, nil
}

// QuickText draws whole paragraphs in a random order until at least minRunes
// are available. Paragraphs may repeat when the source collection is small.
func (l Library) QuickText(rng *rand.Rand, minRunes int) string {
	if len(l.Paragraphs) == 0 {
		return ""
	}
	if minRunes < 1 {
		minRunes = 1
	}

	var out strings.Builder
	runeCount := 0
	order := rng.Perm(len(l.Paragraphs))
	for runeCount < minRunes {
		for _, i := range order {
			if out.Len() > 0 {
				out.WriteString("  ")
				runeCount += 2
			}
			out.WriteString(l.Paragraphs[i])
			runeCount += utf8.RuneCountInString(l.Paragraphs[i])
			if runeCount >= minRunes {
				break
			}
		}
		order = rng.Perm(len(l.Paragraphs))
	}
	return out.String()
}

// DictionaryText creates a randomized word drill.
func (l Library) DictionaryText(rng *rand.Rand, count int) string {
	if len(l.Words) == 0 || count <= 0 {
		return ""
	}
	words := make([]string, 0, count)
	for len(words) < count {
		for _, i := range rng.Perm(len(l.Words)) {
			words = append(words, l.Words[i])
			if len(words) == count {
				break
			}
		}
	}
	return strings.Join(words, " ")
}

func paragraphsFromDir(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", root)
	}

	var paths []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".txt") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	var paragraphs []string
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("%q is not valid UTF-8", path)
		}
		paragraphs = append(paragraphs, splitParagraphs(string(data))...)
	}
	return paragraphs, nil
}

func paragraphsFromFS(source fs.FS, root string) ([]string, error) {
	var paths []string
	err := fs.WalkDir(source, root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".txt") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	var paragraphs []string
	for _, path := range paths {
		data, readErr := fs.ReadFile(source, path)
		if readErr != nil {
			return nil, readErr
		}
		paragraphs = append(paragraphs, splitParagraphs(string(data))...)
	}
	return paragraphs, nil
}

func lessonsFromDir(root string) ([]Lesson, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var lessons []Lesson
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".txt") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("%q is not valid UTF-8", path)
		}
		if text := normalize(string(data)); text != "" {
			lessons = append(lessons, Lesson{Name: titleFromFilename(entry.Name()), Text: text, Source: path})
		}
	}
	sort.Slice(lessons, func(i, j int) bool { return lessons[i].Source < lessons[j].Source })
	return lessons, nil
}

func lessonsFromFS(source fs.FS, root string) ([]Lesson, error) {
	entries, err := fs.ReadDir(source, root)
	if err != nil {
		return nil, err
	}
	var lessons []Lesson
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".txt") {
			continue
		}
		path := root + "/" + entry.Name()
		data, readErr := fs.ReadFile(source, path)
		if readErr != nil {
			return nil, readErr
		}
		if text := normalize(string(data)); text != "" {
			lessons = append(lessons, Lesson{Name: titleFromFilename(entry.Name()), Text: text, Source: path})
		}
	}
	sort.Slice(lessons, func(i, j int) bool { return lessons[i].Source < lessons[j].Source })
	return lessons, nil
}

func readWords(file fs.File) ([]string, error) {
	seen := make(map[string]bool)
	var words []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, word := range strings.Fields(line) {
			if !utf8.ValidString(word) || seen[word] {
				continue
			}
			seen[word] = true
			words = append(words, word)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return words, nil
}

func splitParagraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var result []string
	var block []string
	flush := func() {
		if p := normalize(strings.Join(block, " ")); p != "" {
			result = append(result, p)
		}
		block = block[:0]
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		block = append(block, line)
	}
	flush()
	return result
}

func normalize(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func titleFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = strings.TrimLeft(base, "0123456789-_. ")
	base = strings.NewReplacer("-", " ", "_", " ").Replace(base)
	words := strings.Fields(base)
	for i, word := range words {
		runes := []rune(word)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	if len(words) == 0 {
		return "Lesson"
	}
	return strings.Join(words, " ")
}
