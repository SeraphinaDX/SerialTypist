# SerialTypist

![Serial Typist Logo](serialtypist.avif)

SerialTypist is a colorful terminal typing tutor written in Go with
[gotui v5](https://github.com/metaspartan/gotui). It has four practice modes:

- **Quick speed test:** selects random paragraphs from random `.txt` files and
  runs a timed test.
- **Lessons:** presents sorted lesson files one at a time and offers the next
  lesson after each result, with persistent mastery and personal-best scores.
- **Dictionary drill:** builds a fresh exercise from randomized dictionary
  words.
- **Adaptive practice:** remembers corrected mistakes and builds focused drills
  from troublesome words and dictionary words containing troublesome characters.

![SerialTypist screenshot](screenshot.avif)

The typing surface draws directly into gotui's cell buffer. Correct characters,
incorrect characters, the current word, and the exact next position are all
visually distinct. Text entry and cursor positions are Unicode-aware.

## Quick start

SerialTypist requires Go 1.24 or newer.

```sh
go build -o serialtypist ./cmd/serialtypist
./serialtypist
```

On Windows PowerShell:

```powershell
go build -o serialtypist.exe ./cmd/serialtypist
.\serialtypist.exe
```

It includes sample paragraphs, six lessons, and a small dictionary, so no setup
is required for the first run.

## TOML configuration

SerialTypist looks for configuration in this order:

1. The file supplied with `-config=PATH`.
2. `serialtypist.toml` in the current working directory.
3. The operating system's per-user configuration directory.
4. On Windows, the shared machine configuration directory.

| System | Per-user location |
| --- | --- |
| Linux | `$XDG_CONFIG_HOME/serialtypist/serialtypist.toml`, normally `~/.config/serialtypist/serialtypist.toml` |
| Windows | `%AppData%\SerialTypist\serialtypist.toml` |
| macOS | `~/Library/Application Support/serialtypist/serialtypist.toml` |

Windows also supports `%ProgramData%\SerialTypist\serialtypist.toml` after the
per-user location. This is useful for a school administrator who wants one
configuration for every account on a computer while still allowing a student's
per-user configuration to take precedence.

The per-user directory and file are optional; SerialTypist continues with its
built-in content and defaults when neither exists. An explicitly supplied file
must exist and be valid.

```toml
[content]
texts = "texts"
lessons = "lessons"
dictionary = "words.txt"

[practice]
duration = "60s"
dictionary_words = 60

[mastery]
minimum_accuracy = 95.0
minimum_wpm = 20.0
```

Relative content paths are resolved from the directory containing the TOML
file—not from the directory where SerialTypist was launched. This lets a school
copy one self-contained configuration and lesson directory between machines.

Windows absolute paths may use TOML literal strings, avoiding doubled
backslashes:

```toml
[content]
texts = 'C:\Typing\texts'
lessons = 'C:\Typing\lessons'
dictionary = 'C:\Typing\words.txt'
```

Command-line content and practice options override the corresponding TOML
values. For example:

```sh
serialtypist -config=C:\Typing\serialtypist.toml -duration=30s
```

Using `-option=value` works consistently in bash, fish, PowerShell, and Command
Prompt.

## Command-line options

| Option | Default | Purpose |
| --- | --- | --- |
| `-config=FILE` | automatic lookup | TOML configuration file |
| `-texts=DIR` | configured or bundled text | Quick-test `.txt` directory |
| `-lessons=DIR` | configured or bundled lessons | Lesson `.txt` directory |
| `-dictionary=FILE` | configured or bundled words | Dictionary file |
| `-progress=FILE` | per-user state directory | Lesson progress file |
| `-duration=TIME` | configured or `60s` | Quick-test length (`30s`, `2m`, etc.) |
| `-words=N` | configured or `60` | Words in each dictionary drill |
| `-version` | off | Print the program version |

Supplying a content path replaces the corresponding bundled content. This makes
it easy to use only your own books, articles, source material, or course.

## Lesson progress and mastery

SerialTypist records lesson attempts, personal-best WPM, personal-best
accuracy, the most recent practice time, and whether each lesson is mastered.
The lesson browser uses these markers:

- `✓` mastered
- `●` attempted but not yet mastered
- `○` not attempted

A lesson is mastered when one completed attempt meets both values in the
`[mastery]` configuration section. Set a requirement to `0` to disable it. The
home screen's `C`/`4` shortcut starts the first lesson that has not been
mastered. Lessons can still be selected and practiced in any order.

Progress is deliberately stored per operating-system user, even when a school
uses a shared configuration from `%ProgramData%`:

| System | Progress location |
| --- | --- |
| Linux | `$XDG_STATE_HOME/serialtypist/progress.toml`, normally `~/.local/state/serialtypist/progress.toml` |
| Windows | `%LocalAppData%\SerialTypist\progress.toml` |
| macOS | `~/Library/Application Support/SerialTypist/progress.toml` |

Use `-progress=FILE` to override the location for a portable installation or
test profile. Progress is written through a temporary file and rename so an
interrupted save cannot leave a partially written TOML file.

## Adaptive practice

SerialTypist remembers the expected characters and words behind mistakes made
in completed quick tests, lessons, and dictionary drills. Press `A` or `5` on
the home screen to generate a drill weighted toward those trouble spots.
Recorded words get the strongest preference; the supplied dictionary adds more
words containing characters that need work. Printable symbols still produce a
useful drill even when no dictionary word contains them.

After each adaptive drill, earlier mistake weights are reduced before any new
mistakes are added. This lets the focus move on as accuracy improves instead of
repeating old trouble spots forever. Existing v0.3 progress files are upgraded
in memory automatically, preserving all lesson records.

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

Each top-level `.txt` file is one lesson. SerialTypist sorts filenames, so
numeric prefixes make the intended order explicit:

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
- `C` or `4`: continue with the first unmastered lesson
- `A` or `5`: adaptive practice based on corrected mistakes
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
typed text. Lesson mastery defaults to at least 95% accuracy and 20 WPM.

## Development

```sh
go test ./...
go vet ./...
go build ./cmd/serialtypist
```

The content loader, configuration loader, progress store, adaptive drill
generator, session/scoring engine, Unicode behavior, and text layout have unit
tests. GitHub Actions runs the complete test, vet, and build suite on Linux,
Windows, and macOS. The TUI itself can be smoke-tested in any 56×18 or larger
terminal; an 80×24 TrueColor terminal is recommended.
