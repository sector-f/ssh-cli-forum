package tui

import (
	"reflect"
	"testing"
)

func TestSplitCommandQuotedArguments(t *testing.T) {
	got, err := splitCommand(`board create general "General Discussion" "All topics here"`)
	if err != nil { t.Fatal(err) }
	want := []string{"board", "create", "general", "General Discussion", "All topics here"}
	if !reflect.DeepEqual(got, want) { t.Fatalf("splitCommand() = %#v, want %#v", got, want) }
}

func TestSplitCommandRejectsUnterminatedQuote(t *testing.T) {
	if _, err := splitCommand(`board create general "General Discussion`); err == nil {
		t.Fatal("expected unterminated quote error")
	}
}
