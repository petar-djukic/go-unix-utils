// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fmt against GNU gfmt.
// Covers prd070-fmt R4.1 (-p prefix), R4.2 (-t tagged-paragraph),
// R5.1-R5.4 (exit codes and comprehensive differential testing).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeFile creates a test file in dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", p, err)
	}
	return p
}

// clearOutput normalizes output to nil for error tests where exact
// error message format differs between Go and GNU implementations.
// We rely on exit code comparison for error cases.
var clearOutput testutils.NormalizeFunc = func(b []byte) []byte { return nil }

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skipf("reference binary gfmt not in PATH: %v", err)
	}

	dir := t.TempDir()

	// Test files for multi-file input (R5.4).
	f1 := writeFile(t, dir, "f1.txt",
		"First file paragraph with enough words to test wrapping behavior.\n")
	f2 := writeFile(t, dir, "f2.txt",
		"Second file paragraph.\n")

	// File with two paragraphs for default formatting test.
	twoPara := writeFile(t, dir, "twopara.txt",
		"Short first paragraph.\n\nShort second paragraph.\n")

	// File with prefixed lines (R4.1).
	prefixed := writeFile(t, dir, "prefixed.txt",
		"> This is a prefixed line that is quite long and should be "+
			"reformatted by the fmt utility.\n"+
			"> More prefixed text here.\n"+
			"Not prefixed at all.\n"+
			"> Another prefixed block.\n")

	// File with tagged paragraphs (R4.2).
	tagged := writeFile(t, dir, "tagged.txt",
		"  * Item one: description text that goes on for a while "+
			"to test wrapping.\n"+
			"    Continuation of item one body text.\n"+
			"\n"+
			"  * Item two: another description.\n"+
			"    Body of item two.\n")

	tests := []testutils.DiffTest{
		// --- R5.1: Exit 0 on success ---

		// R5.4: default 75-char formatting, short line unchanged.
		{
			Name:  "R5.1_R5.4_default_short_line",
			Stdin: []byte("This is a short line.\n"),
		},

		// R5.4: default formatting with file input and paragraphs.
		{
			Name: "R5.4_default_file_paragraphs",
			Args: []string{twoPara},
		},

		// R5.4: blank line paragraph boundaries.
		{
			Name: "R5.4_blank_line_paragraphs",
			Stdin: []byte("First paragraph words words words words " +
				"words words words words words.\n\n" +
				"Second paragraph.\n"),
		},

		// R5.4: indentation preservation.
		{
			Name: "R5.4_indentation_preserved",
			Stdin: []byte("  Indented first line of paragraph.\n" +
				"  Indented second line.\n"),
		},

		// R5.4: -w custom width.
		{
			Name:  "R5.4_width_w30",
			Args:  []string{"-w", "30"},
			Stdin: []byte("Hello world this is a test of custom width formatting.\n"),
		},

		// R5.4: --width= long form.
		{
			Name:  "R5.4_width_long_form",
			Args:  []string{"--width=40"},
			Stdin: []byte("Testing the long form width flag with enough words to wrap.\n"),
		},

		// R5.4: -g goal width.
		{
			Name:  "R5.4_goal_g30",
			Args:  []string{"-w", "50", "-g", "30"},
			Stdin: []byte("Testing goal width with enough words to see effect.\n"),
		},

		// R5.4: -s split-only mode.
		{
			Name: "R5.4_split_only",
			Args: []string{"-s", "-w", "25"},
			Stdin: []byte("Short.\n" +
				"This is a longer line that needs splitting.\n"),
		},

		// R5.4: -u uniform spacing.
		{
			Name:  "R5.4_uniform_spacing",
			Args:  []string{"-u"},
			Stdin: []byte("Hello   world.  This   has   irregular   spacing.\n"),
		},

		// R5.4: multi-file input.
		{
			Name: "R5.4_multi_file",
			Args: []string{f1, f2},
		},

		// R5.4: stdin via -.
		{
			Name:  "R5.4_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("Stdin text here.\n"),
		},

		// R5.4: empty input.
		{
			Name:  "R5.4_empty_input",
			Stdin: []byte(""),
		},

		// --- R4.1: Prefix mode ---

		// R4.1: -p prefix, basic filtering.
		{
			Name: "R4.1_prefix_basic",
			Args: []string{"-p", ">"},
			Stdin: []byte("> Line one of prefixed text that is " +
				"rather long.\n" +
				"> Line two.\n" +
				"Normal line.\n"),
		},

		// R4.1: -p prefix from file.
		{
			Name: "R4.1_prefix_file",
			Args: []string{"-p", ">", prefixed},
		},

		// R4.1: --prefix= long form.
		{
			Name:  "R4.1_prefix_long_form",
			Args:  []string{"--prefix=>"},
			Stdin: []byte("> One.\n> Two.\n"),
		},

		// R4.1: prefix with hash comment style, single long line.
		{
			Name: "R4.1_prefix_hash",
			Args: []string{"-p", "# ", "-w", "40"},
			Stdin: []byte("# This is a very long comment line that definitely exceeds the width and must be wrapped.\n" +
				"Code line.\n"),
		},

		// R4.1: prefix with non-prefixed passthrough.
		{
			Name: "R4.1_prefix_passthrough",
			Args: []string{"-p", "//"},
			Stdin: []byte("plain text\n" +
				"// Prefixed line.\n" +
				"more plain\n"),
		},

		// --- R4.2: Tagged paragraph mode ---

		// R4.2: -t tagged paragraph from file.
		{
			Name: "R4.2_tagged_paragraph_file",
			Args: []string{"-t", tagged},
		},

		// R4.2: -t with stdin.
		{
			Name: "R4.2_tagged_paragraph_stdin",
			Args: []string{"-t"},
			Stdin: []byte("  Tag line here.\n" +
				"    Body text words words words words " +
				"words words words.\n"),
		},

		// R4.2: --tagged-paragraph long form.
		{
			Name:  "R4.2_tagged_paragraph_long",
			Args:  []string{"--tagged-paragraph"},
			Stdin: []byte("Tag.\n  Body.\n"),
		},

		// R4.2: -t with width, simple tagged paragraph.
		{
			Name: "R4.2_tagged_with_width",
			Args: []string{"-t", "-w", "50"},
			Stdin: []byte("  Tag line.\n" +
				"    Body text.\n"),
		},

		// --- R5.2: Exit 1 on error ---

		// R5.2: invalid option.
		{
			Name:      "R5.2_invalid_option",
			Args:      []string{"--badoption"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},

		// R5.2: missing file.
		{
			Name:      "R5.2_missing_file",
			Args:      []string{"/no/such/file.txt"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},

		// R5.2: invalid width value (non-numeric).
		{
			Name:      "R5.2_invalid_width",
			Args:      []string{"--width=abc"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
