package main

import (
	"fmt"
	"os"
	"path/filepath"

	"punctpolish/internal/processor"
	"punctpolish/internal/scanner"
)

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}

	target, err := filepath.Abs(os.Args[1])
	if err != nil {
		fatal(err)
	}

	info, err := os.Stat(target)
	if err != nil {
		fatal(err)
	}

	proc := processor.New(processor.DefaultMaxFileSize)
	if info.IsDir() {
		scanner.Walk(target, proc)
		return
	}
	if !info.Mode().IsRegular() {
		fatal(fmt.Errorf("%q is not a regular file or directory", target))
	}
	if _, err := proc.Process(target); err != nil {
		fatal(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s <file-or-directory>\n", filepath.Base(os.Args[0]))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
