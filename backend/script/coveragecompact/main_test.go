// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/cover"
)

func TestCompactPreservesCoverage(t *testing.T) {
	for _, mode := range []string{"set", "count", "atomic"} {
		t.Run(mode, func(t *testing.T) {
			input := fmt.Sprintf("mode: %s\n", mode) +
				"example/b.go:8.2,10.3 2 0\n" +
				"example/a.go:1.1,3.2 3 1\n" +
				"example/a.go:1.1,3.2 3 1\n" +
				"example/a.go:5.1,6.2 1 0\n" +
				"example/b.go:8.2,10.3 2 1\n" +
				"example/a.go:1.1,3.2 3 0\n"
			var output bytes.Buffer
			require.NoError(t, compact(strings.NewReader(input), &output))
			original, err := cover.ParseProfilesFromReader(strings.NewReader(input))
			require.NoError(t, err)
			result, err := cover.ParseProfilesFromReader(bytes.NewReader(output.Bytes()))
			require.NoError(t, err)
			require.Equal(t, original, result, "all covered and uncovered blocks and hit counts must be unchanged")
			require.Len(t, result, 2)
			require.Len(t, result[0].Blocks, 2)
			require.Equal(t, 0, result[0].Blocks[1].Count, "never drop uncovered blocks")
			require.Equal(t, 1, result[1].Blocks[0].Count, "coverage from another test package must survive")
			wantCount := 2
			if mode == "set" {
				wantCount = 1
			}
			require.Equal(t, wantCount, result[0].Blocks[0].Count)
			require.Less(t, output.Len(), len(input))
			var second bytes.Buffer
			require.NoError(t, compact(bytes.NewReader(output.Bytes()), &second))
			require.Equal(t, output.String(), second.String(), "compaction must be deterministic and idempotent")
		})
	}
}

func TestCompactRejectsInvalidReports(t *testing.T) {
	for _, input := range []string{"", "mode: atomic\n", "bad header\n", "mode: invalid\na.go:1.1,2.2 1 1\n", "mode: count\nbad block\n", "mode: count\na.go:1.1,2.2 1 0\na.go:1.1,2.2 2 1\n"} {
		t.Run(input, func(t *testing.T) {
			var output bytes.Buffer
			require.Error(t, compact(strings.NewReader(input), &output))
			require.Empty(t, output.String(), "invalid input must not produce a successful-looking report")
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestCompactWriteFailure(t *testing.T) {
	input := "mode: atomic\na.go:1.1,2.2 1 1\n"
	require.ErrorContains(t, compact(strings.NewReader(input), failingWriter{}), "write failed")
	input += strings.Repeat("long/path/to/another_file.go:3.1,4.2 1 0\n", 10)
	// Distinct files force buffered writes as well as the final flush.
	for i := 0; i < 300; i++ {
		input += fmt.Sprintf("path/file%d.go:1.1,2.2 1 0\n", i)
	}
	require.ErrorContains(t, compact(strings.NewReader(input), failingWriter{}), "write failed")
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "in.out"), filepath.Join(dir, "out.out")
	raw := []byte("mode: count\na.go:1.1,2.2 1 0\na.go:1.1,2.2 1 2\n")
	require.NoError(t, os.WriteFile(input, raw, 0o600))
	require.NoError(t, run(input, output))
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, "mode: count\na.go:1.1,2.2 1 2\n", string(data))
	unchanged, err := os.ReadFile(input)
	require.NoError(t, err)
	require.Equal(t, raw, unchanged)
	require.Error(t, run(input, input))
	require.Error(t, run(input, output), "never overwrite an existing report")
	require.Error(t, run(filepath.Join(dir, "missing"), output))
	require.Error(t, run(input, filepath.Join(dir, "missing", "out")))
	require.NoError(t, os.WriteFile(input, []byte("invalid"), 0o600))
	require.Error(t, run(input, filepath.Join(dir, "invalid.out")))
}
