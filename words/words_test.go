package words

import (
	"strings"
	"testing"
)

func TestWordList_LengthAndContent(t *testing.T) {
	if len(WordList) < 50 {
		t.Errorf("expected WordList to have at least 50 words, got %d", len(WordList))
	}
}

func TestWordList_NoEmptyOrWhitespace(t *testing.T) {
	for i, word := range WordList {
		if strings.TrimSpace(word) == "" {
			t.Errorf("found empty or whitespace word at index %d", i)
		}
	}
}
