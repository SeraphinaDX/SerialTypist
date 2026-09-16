package adaptive

import (
	"math/rand"
	"strings"
	"testing"
)

func TestBuildUsesDictionaryWordsWithTroubleCharacters(t *testing.T) {
	target := Build(
		rand.New(rand.NewSource(1)),
		[]string{"apple", "berry"},
		map[string]int{"p": 3},
		nil,
		8,
	)
	words := strings.Fields(target)
	if len(words) != 8 {
		t.Fatalf("word count = %d, want 8", len(words))
	}
	for _, word := range words {
		if word != "apple" {
			t.Fatalf("unexpected word %q in %q", word, target)
		}
	}
}

func TestBuildPrioritizesRecordedWordsOutsideDictionary(t *testing.T) {
	target := Build(
		rand.New(rand.NewSource(2)),
		[]string{"alpha"},
		nil,
		map[string]int{"semicolon;": 2},
		4,
	)
	if target != "semicolon; semicolon; semicolon; semicolon;" {
		t.Fatalf("target = %q", target)
	}
}

func TestBuildFallsBackToPrintableCharacterGroups(t *testing.T) {
	target := Build(
		rand.New(rand.NewSource(3)),
		nil,
		map[string]int{";": 2},
		nil,
		3,
	)
	if target != ";;;; ;;;; ;;;;" {
		t.Fatalf("target = %q", target)
	}
}

func TestBuildWithoutMistakesIsEmpty(t *testing.T) {
	if target := Build(rand.New(rand.NewSource(1)), []string{"alpha"}, nil, nil, 10); target != "" {
		t.Fatalf("target = %q, want empty", target)
	}
}
