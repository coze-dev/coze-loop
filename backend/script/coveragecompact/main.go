// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// coveragecompact coalesces duplicate blocks emitted by go test -coverpkg=./...
// before upload. It preserves every file, block, statement count and hit count,
// using the same merging semantics as go tool cover.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/tools/cover"
)

func compact(input io.Reader, output io.Writer) error {
	profiles, err := cover.ParseProfilesFromReader(input)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		return fmt.Errorf("coverage report contains no blocks")
	}
	mode := profiles[0].Mode
	if mode != "set" && mode != "count" && mode != "atomic" {
		return fmt.Errorf("unsupported coverage mode %q", mode)
	}
	w := bufio.NewWriter(output)
	if _, err := fmt.Fprintf(w, "mode: %s\n", mode); err != nil {
		return err
	}
	for _, profile := range profiles {
		for _, block := range profile.Blocks {
			if _, err := fmt.Fprintf(w, "%s:%d.%d,%d.%d %d %d\n", profile.FileName,
				block.StartLine, block.StartCol, block.EndLine, block.EndCol, block.NumStmt, block.Count); err != nil {
				return err
			}
		}
	}
	return w.Flush()
}

func run(inputPath, outputPath string) error {
	// Keep the original profile available for diagnostics and equivalence checks.
	if inputPath == outputPath {
		return fmt.Errorf("input and output paths must differ")
	}
	input, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	output, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	err = compact(input, output)
	closeErr := output.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func main() {
	input := flag.String("input", "coverage.out", "original Go coverage profile")
	output := flag.String("output", "coverage.compact.out", "new compact profile (must not exist)")
	flag.Parse()
	if err := run(*input, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
