# SerialTypist

SerialTypist is a colorful terminal typing trainer written in Go with
[gotui v5](https://github.com/metaspartan/gotui). It has three useful practice
styles:

- **Quick speed test:** selects random paragraphs from random `.txt` files and
  runs a timed test.
- **Lessons:** presents sorted lesson files one at a time and offers the next
  lesson after each result.
- **Dictionary drill:** builds a fresh exercise from randomized dictionary
  words.

![Serial Typist Screenshot](screenshot.avif)

The typing surface draws directly into gotui's cell buffer. Correct characters,
incorrect characters, the current word, and the exact next position are all
visually distinct. Text entry and cursor positions are Unicode-aware.

## Quick start

SerialTypist requires Go 1.24 or newer.

```sh
go build -o serialtypist ./cmd/serialtypist
./serialtypist
```

Load a directory of text files:
```
./SerialTypist -texts=vimuser/
```


It includes sample paragraphs, six lessons, and a small dictionary, so no setup
is required for the first run.

For your own material:

```sh
./serialtypist \
  -texts=./my-texts \
  -lessons=./my-lessons \
  -dictionary=/usr/share/dict/words \
  -duration=60s \
  -words=80
```

Using `-option=value` works consistently in bash, fish, and other shells.

## Options

| Option | Default | Purpose |
| --- | --- | --- |
| `-texts=DIR` | bundled text | Quick-test `.txt` directory |
| `-lessons=DIR` | bundled lessons | Lesson `.txt` directory |
| `-dictionary=FILE` | bundled words | Dictionary file |
| `-duration=TIME` | `60s` | Quick-test length (`30s`, `2m`, etc.) |
| `-words=N` | `60` | Words in each dictionary drill |
| `-version` | off | Print the program version |

Supplying a path replaces the corresponding bundled content. This makes it
easy to use only your own books, articles, source material, or course.

## Text directory format

Quick mode searches the supplied directory and all subdirectories for files
ending in `.txt`. Files must be UTF-8. A blank line separates paragraphs:

```text
This is the first paragraph. It can wrap across lines in the file.

This is the second paragraph. Quick mode may choose either one.
```

Line breaks inside a paragraph become spaces. Quick mode shuffles complete
paragraphs and adds enough material to keep a fast typist supplied for the
whole timer.

See `examples/texts/` for a ready-to-copy example.

## Lesson directory format

Each top-level `.txt` file is one lesson. SerialTypist sorts filenames, so numeric
prefixes make the intended order explicit:

```text
001-home-row.txt
002-top-row.txt
003-capitals.txt
```

The filename becomes the on-screen title (`001-home-row.txt` becomes
`Home Row`). All whitespace in the file is normalized to single spaces, and
the resulting text becomes the exercise.

See `examples/lessons/` for an example.

## Dictionary format

Both one-word-per-line dictionaries and whitespace-separated word lists work:

```text
rose
violet
lily
```

Blank lines and lines beginning with `#` are ignored. Duplicate words are
removed while loading.

## Controls

### Home

- `Q` or `1`: quick speed test
- `L` or `2`: lesson browser
- `D` or `3`: dictionary drill
- `Esc` or `Ctrl+C`: quit

### Lesson browser

- `Up`/`Down` or `k`/`j`: select a lesson
- `Enter`: begin
- `Esc`: return to the home screen

### While typing

- Type normally; the timer begins with the first printable character.
- `Backspace`: correct the previous character.
- `Esc`: abandon the exercise and return home.
- `Ctrl+C`: quit safely.

### Results

- `R`: repeat the same text
- `N` or `Enter`: get a new test/drill or continue to the next lesson
- `M` or `Esc`: return home

## Scoring

WPM uses the conventional five-correct-characters-per-word formula. Accuracy
counts every printable key attempt, so a corrected mistake still affects the
accuracy result. The live error count shows only mistakes still present in the
typed text.

## Development

```sh
go test ./...
go vet ./...
```

The content loader, session/scoring engine, Unicode behavior, and text layout
have unit tests. The TUI itself can be smoke-tested in any 56×18 or larger
terminal; an 80×24 TrueColor terminal is recommended.
A blank line starts a new paragraph. During a quick speed test, SerialTypist picks
