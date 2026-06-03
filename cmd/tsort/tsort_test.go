// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var stderrNorm = regexp.MustCompile(`^g?tsort:`)

func normStderr(b []byte) []byte {
	var out []byte
	for line := range bytes.SplitSeq(b, []byte("\n")) {
		if len(out) > 0 {
			out = append(out, '\n')
		}
		out = append(out, stderrNorm.ReplaceAll(line, []byte("PROG:"))...)
	}
	return out
}

func normHelpLine(b []byte) []byte {
	var out []byte
	for line := range bytes.SplitSeq(b, []byte("\n")) {
		if len(out) > 0 {
			out = append(out, '\n')
		}
		if bytes.HasPrefix(line, []byte("Try '")) {
			out = append(out, []byte("Try 'tsort --help' for more information.")...)
		} else {
			out = append(out, line...)
		}
	}
	return out
}

func normErrno(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("No such file or directory"), []byte("no such file or directory"))
	return b
}

func TestDiff(t *testing.T) {
	if _, err := os.Stat(filepath.Join(".", "main.go")); os.IsNotExist(err) {
		t.Skip("cmd/tsort not yet generated")
	}
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtsort")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFile(t, dir, "simple_pairs.txt", "a b b c\n")
	writeFile(t, dir, "cycle.txt", "a b b a\n")
	writeFile(t, dir, "multiline.txt", "a b\nb c\nc d\n")
	writeFile(t, dir, "diamond.txt", "a b a c b d c d\n")
	writeFile(t, dir, "lone_nodes.txt", "a a\nb b\n")
	writeFile(t, dir, "empty.txt", "")
	writeFile(t, dir, "odd_tokens.txt", "a b c\n")
	writeFile(t, dir, "complex_cycle.txt", "a b b c c a d e\n")

	env := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		// R1.1, R2.1: basic topological sorting from stdin
		{
			Name:  "r1_1_basic_pair",
			Stdin: []byte("a b\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_chain",
			Stdin: []byte("a b b c\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_diamond",
			Stdin: []byte("a b a c b d c d\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_multiline_pairs",
			Stdin: []byte("a b\nb c\nc d\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_lone_nodes",
			Stdin: []byte("a a\nb b\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_self_pair",
			Stdin: []byte("x x\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_empty_input",
			Stdin: []byte(""),
			Env:   env,
		},
		{
			Name:  "r1_1_multiple_spaces",
			Stdin: []byte("a  b  b  c\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_tabs_between_tokens",
			Stdin: []byte("a\tb\tb\tc\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_blank_lines",
			Stdin: []byte("a b\n\nb c\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_many_nodes",
			Stdin: []byte("a b b c c d d e e f f g\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_duplicate_pairs",
			Stdin: []byte("a b a b b c\n"),
			Env:   env,
		},
		// R1.2, R2.2: cycle detection
		{
			Name:      "r1_2_simple_cycle",
			Stdin:     []byte("a b b a\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r1_2_three_node_cycle",
			Stdin:     []byte("a b b c c a\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r1_2_cycle_with_other_nodes",
			Stdin:     []byte("a b b c c a d e\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r1_2_self_cycle",
			Stdin:     []byte("a b b a a a\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r1_2_cycle_from_file",
			Args:      []string{"cycle.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r1_2_complex_cycle_from_file",
			Args:      []string{"complex_cycle.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		// R1.3, R2.2: odd number of tokens
		{
			Name:      "r1_3_single_token",
			Stdin:     []byte("a\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r1_3_three_tokens",
			Stdin:     []byte("a b c\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r1_3_five_tokens",
			Stdin:     []byte("a b c d e\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		// R1.4: file argument, stdin, and "-"
		{
			Name:    "r1_4_file_argument",
			Args:    []string{"simple_pairs.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_4_multiline_file",
			Args:    []string{"multiline.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_4_diamond_file",
			Args:    []string{"diamond.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_4_empty_file",
			Args:    []string{"empty.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_4_lone_nodes_file",
			Args:    []string{"lone_nodes.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:  "r1_4_dash_stdin",
			Args:  []string{"-"},
			Stdin: []byte("a b b c\n"),
			Env:   env,
		},
		{
			Name:  "r1_4_no_args_stdin",
			Stdin: []byte("a b b c\n"),
			Env:   env,
		},
		{
			Name:      "r1_4_odd_tokens_file",
			Args:      []string{"odd_tokens.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		// R2.2: exit 1 on malformed input / error conditions
		{
			Name:      "r2_2_extra_operand",
			Args:      []string{"simple_pairs.txt", "cycle.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr, normHelpLine},
		},
		{
			Name:      "r2_2_nonexistent_file",
			Args:      []string{"no_such_file.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr, normErrno},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	if _, err := os.Stat(filepath.Join(".", "main.go")); os.IsNotExist(err) {
		t.Skip("cmd/tsort not yet generated")
	}
	goBin := testutils.BuildBinary(t, ".")

	var sb strings.Builder
	for i := range 100000 {
		fmt.Fprintf(&sb, "n%d n%d\n", i, i+1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin)
	cmd.Stdin = strings.NewReader(sb.String())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("tsort timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
