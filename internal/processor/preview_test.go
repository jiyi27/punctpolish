package processor

import (
	"os"
	"strings"
	"testing"
)

// TestPreview processes the sample fixture and prints the result.
// Not an automated assertion — run with -v to inspect the output visually.
//
//	go test -v -run TestPreview ./internal/processor/
func TestPreview(t *testing.T) {
	fixture, err := os.ReadFile("../../test/fixtures/sample.md")
	if err != nil {
		t.Fatalf("cannot read fixture: %v", err)
	}

	input := string(fixture)
	output := NormalizeText(input)

	t.Log("=== INPUT ===")
	for _, line := range strings.Split(input, "\n") {
		t.Log(line)
	}

	t.Log("\n=== OUTPUT ===")
	for _, line := range strings.Split(output, "\n") {
		t.Log(line)
	}
}
