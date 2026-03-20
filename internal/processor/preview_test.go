package processor

import (
	"fmt"
	"os"
	"strings"
)

// ExampleNormalizeText_preview processes the sample fixture and prints the
// normalized result for manual inspection during local development.
//
//	go test -run ExampleNormalizeText_preview -v ./internal/processor
func ExampleNormalizeText_preview() {
	fixture, err := os.ReadFile("../../test/fixtures/sample.md")
	if err != nil {
		fmt.Printf("cannot read fixture: %v\n", err)
		return
	}

	input := string(fixture)
	output := NormalizeText(input)

	fmt.Println("=== INPUT ===")
	for _, line := range strings.Split(input, "\n") {
		fmt.Println(line)
	}

	fmt.Println()
	fmt.Println("=== OUTPUT ===")
	for _, line := range strings.Split(output, "\n") {
		fmt.Println(line)
	}
}
