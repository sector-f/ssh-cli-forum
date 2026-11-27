package tui

import (
	"reflect"
	"testing"
)

func TestCompletionContext(t *testing.T) {
	tests := []struct {
		line   string
		before []string
		prefix string
		start  int
	}{
		{line: "cd /boards/gen", before: []string{"cd"}, prefix: "/boards/gen", start: 3},
		{line: "board ", before: []string{"board"}, prefix: "", start: 6},
		{line: "user role ali", before: []string{"user", "role"}, prefix: "ali", start: 10},
	}
	for _, test := range tests {
		before, prefix, start, ok := completionContext(test.line, len([]rune(test.line)))
		if !ok || !reflect.DeepEqual(before, test.before) || prefix != test.prefix || start != test.start {
			t.Errorf("completionContext(%q) = (%v, %q, %d, %t)", test.line, before, prefix, start, ok)
		}
	}
}

func TestCompletionContextRequiresCursorAtEnd(t *testing.T) {
	if _, _, _, ok := completionContext("chat now", 4); ok {
		t.Fatal("expected completion to be disabled when the cursor is not at the end")
	}
}

func TestMatchingCandidatesAndLongestCommonPrefix(t *testing.T) {
	got := matchingCandidates([]string{"restore", "reply", "recent", "reply"}, "re")
	want := []string{"recent", "reply", "restore"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matchingCandidates() = %v, want %v", got, want)
	}
	if got := longestCommonPrefix([]string{"delete-topic", "delete-reply"}); got != "delete-" {
		t.Fatalf("longestCommonPrefix() = %q, want %q", got, "delete-")
	}
}
