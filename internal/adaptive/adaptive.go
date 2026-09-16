// Package adaptive builds drills from a student's accumulated mistakes.
package adaptive

import (
	"math/rand"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type candidate struct {
	text   string
	weight int64
}

// Build returns a weighted drill. Trouble words receive the strongest weight;
// dictionary words containing troublesome characters fill out the exercise.
func Build(rng *rand.Rand, dictionary []string, characterWeights, wordWeights map[string]int, wordCount int) string {
	if wordCount <= 0 {
		return ""
	}
	weights := make(map[string]int64)
	for word, count := range wordWeights {
		word = strings.TrimSpace(word)
		if word != "" && count > 0 {
			weights[word] += boundedWeight(count, 12)
		}
	}
	for _, word := range dictionary {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		var score int64
		for _, character := range word {
			if count := characterWeights[string(character)]; count > 0 {
				score += boundedWeight(count, 1)
			}
		}
		if score > 0 {
			weights[word] += score
		}
	}

	// A punctuation-only lesson may not have matching dictionary entries. Keep
	// those characters useful by producing short, typeable character groups.
	if len(weights) == 0 {
		for value, count := range characterWeights {
			r, size := utf8.DecodeRuneInString(value)
			if count <= 0 || r == utf8.RuneError || size != len(value) || !unicode.IsPrint(r) || unicode.IsSpace(r) {
				continue
			}
			weights[strings.Repeat(value, 4)] = boundedWeight(count, 1)
		}
	}
	if len(weights) == 0 {
		return ""
	}

	keys := make([]string, 0, len(weights))
	for text := range weights {
		keys = append(keys, text)
	}
	sort.Strings(keys)
	candidates := make([]candidate, 0, len(keys))
	for _, text := range keys {
		candidates = append(candidates, candidate{text: text, weight: weights[text]})
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}

	words := make([]string, 0, wordCount)
	last := -1
	for len(words) < wordCount {
		selected := weightedIndex(rng, candidates)
		if len(candidates) > 1 && selected == last {
			for attempts := 0; attempts < 3 && selected == last; attempts++ {
				selected = weightedIndex(rng, candidates)
			}
		}
		words = append(words, candidates[selected].text)
		last = selected
	}
	return strings.Join(words, " ")
}

func weightedIndex(rng *rand.Rand, candidates []candidate) int {
	var total int64
	for _, item := range candidates {
		total += item.weight
	}
	choice := rng.Int63n(total)
	for index, item := range candidates {
		if choice < item.weight {
			return index
		}
		choice -= item.weight
	}
	return len(candidates) - 1
}

func boundedWeight(count, multiplier int) int64 {
	const maximum = int64(1_000_000_000)
	weight := int64(count)
	if weight > maximum/int64(multiplier) {
		return maximum
	}
	return weight * int64(multiplier)
}
