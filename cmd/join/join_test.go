// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/join against gjoin (GNU coreutils).
// Implements srd069-join R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// setupDir creates a temp directory with the given files for test input.
func setupDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
		if err != nil {
			t.Fatalf("writing test file %s: %v", name, err)
		}
	}
	return dir
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gjoin")
	if err != nil {
		t.Skipf("reference binary gjoin not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: basic join on first field, all lines match
		{
			Name: "basic_all_match",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\nc 3\n",
				"f2.txt": "a X\nb Y\nc Z\n",
			}),
		},
		// R1.1, R1.3: partial match, unpaired lines suppressed
		{
			Name: "partial_match",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\nd 4\n",
				"f2.txt": "a X\nc Y\nd Z\n",
			}),
		},
		// R1.3: no matching lines, empty output
		{
			Name: "no_match",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "c X\nd Y\n",
			}),
		},
		// R1.1, R1.2: multi-field lines with whitespace separation
		{
			Name: "multi_field_lines",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1 2\nb 3 4\n",
				"f2.txt": "a X Y\nb P Q\n",
			}),
		},
		// R1.1: duplicate keys in file2, cross product
		{
			Name: "duplicate_key_file2",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "a X\na Y\nb Z\n",
			}),
		},
		// R1.1: duplicate keys in file1, cross product
		{
			Name: "duplicate_key_file1",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\na 2\nb 3\n",
				"f2.txt": "a X\nb Y\n",
			}),
		},
		// R1.1: duplicate keys in both files, full cross product
		{
			Name: "duplicate_keys_both",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\na 2\n",
				"f2.txt": "a X\na Y\n",
			}),
		},
		// R1.1: empty file1, no output
		{
			Name: "empty_file1",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "",
				"f2.txt": "a X\nb Y\n",
			}),
		},
		// R1.1: empty file2, no output
		{
			Name: "empty_file2",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "",
			}),
		},
		// R1.2: single-field lines (join field only, no remaining)
		{
			Name: "single_field_lines",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a\nb\nc\n",
				"f2.txt": "a\nb\n",
			}),
		},
		// R1.4: stdin as file1
		{
			Name: "stdin_as_file1",
			Args:  []string{"-", "f2.txt"},
			Stdin: []byte("a 1\nb 2\n"),
			WorkDir: setupDir(t, map[string]string{
				"f2.txt": "a X\nb Y\n",
			}),
		},
		// R1.4: stdin as file2
		{
			Name: "stdin_as_file2",
			Args:  []string{"f1.txt", "-"},
			Stdin: []byte("a X\nb Y\n"),
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
			}),
		},
		// R1.1: single matching line in each file
		{
			Name: "single_line_match",
			Args: []string{"f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "hello world\n",
				"f2.txt": "hello there\n",
			}),
		},
		// R2.4: -t CHAR uses comma as field separator
		{
			Name: "separator_comma",
			Args: []string{"-t", ",", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a,1,2\nb,3,4\nc,5,6\n",
				"f2.txt": "a,X,Y\nb,P,Q\n",
			}),
		},
		// R2.4: -t with colon separator
		{
			Name: "separator_colon",
			Args: []string{"-t", ":", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a:1\nb:2\n",
				"f2.txt": "a:X\nb:Y\n",
			}),
		},
		// R2.4: -t with tab separator
		{
			Name: "separator_tab",
			Args: []string{"-t", "\t", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a\t1\nb\t2\n",
				"f2.txt": "a\tX\nb\tY\n",
			}),
		},
		// R2.1: -1 2 -2 1 join on second field of file1, first of file2
		{
			Name: "field_selection_1_2",
			Args: []string{"-1", "2", "-2", "1", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "x a\ny b\nz c\n",
				"f2.txt": "a 10\nb 20\n",
			}),
		},
		// R2.1: -1 3 join on third field of file1
		{
			Name: "field_selection_1_3",
			Args: []string{"-1", "3", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "x y a\np q b\n",
				"f2.txt": "a 10\nb 20\n",
			}),
		},
		// R2.1: -2 2 join on second field of file2
		{
			Name: "field_selection_2_2",
			Args: []string{"-2", "2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "X a\nY b\n",
			}),
		},
		// R2.2: -j 2 sets both join fields to 2
		{
			Name: "j_flag_both_fields",
			Args: []string{"-j", "2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "x a\ny b\n",
				"f2.txt": "P a\nQ b\n",
			}),
		},
		// R2.3: -o FORMAT with specific field selection
		{
			Name: "output_format_basic",
			Args: []string{"-o", "0,1.2,2.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\nc 3\n",
				"f2.txt": "a X\nb Y\nc Z\n",
			}),
		},
		// R2.3: -o with reversed field order
		{
			Name: "output_format_reversed",
			Args: []string{"-o", "2.2,0,1.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "a X\nb Y\n",
			}),
		},
		// R2.3: -o with space-separated format
		{
			Name: "output_format_space_sep",
			Args: []string{"-o", "1.1 1.2 2.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "a X\nb Y\n",
			}),
		},
		// R2.3: -o 0 outputs only the join field
		{
			Name: "output_format_join_only",
			Args: []string{"-o", "0", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "a X\nb Y\n",
			}),
		},
		// R2.1 + R2.4: -t and -1/-2 combined
		{
			Name: "separator_and_field_selection",
			Args: []string{"-t", ",", "-1", "2", "-2", "1", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "x,a,1\ny,b,2\n",
				"f2.txt": "a,10\nb,20\n",
			}),
		},
		// R2.3 + R2.4: -o and -t combined
		{
			Name: "output_format_with_separator",
			Args: []string{"-t", ",", "-o", "0,1.2,2.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a,1\nb,2\n",
				"f2.txt": "a,X\nb,Y\n",
			}),
		},
		// R2.1 + R2.3: -1/-2 with -o format
		{
			Name: "field_selection_with_output_format",
			Args: []string{"-1", "2", "-o", "0,1.1,2.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "x a\ny b\n",
				"f2.txt": "a 10\nb 20\n",
			}),
		},
		// R2.2 + R2.4: -j with -t combined
		{
			Name: "j_flag_with_separator",
			Args: []string{"-j", "2", "-t", ",", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "x,a\ny,b\n",
				"f2.txt": "P,a\nQ,b\n",
			}),
		},
		// R2.3: -o with multi-field output from both files
		{
			Name: "output_format_multi_fields",
			Args: []string{"-o", "1.1,1.2,1.3,2.1,2.2,2.3", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1 2\nb 3 4\n",
				"f2.txt": "a X Y\nb P Q\n",
			}),
		},
		// R3.1: -a 1 shows unpairable lines from file1 plus paired
		{
			Name: "a1_unpairable_file1",
			Args: []string{"-a", "1", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\nc 3\n",
				"f2.txt": "a X\nc Z\n",
			}),
		},
		// R3.1: -a 2 shows unpairable lines from file2 plus paired
		{
			Name: "a2_unpairable_file2",
			Args: []string{"-a", "2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nc 3\n",
				"f2.txt": "a X\nb Y\nc Z\n",
			}),
		},
		// R3.1: -a 1 -a 2 shows unpairable from both files
		{
			Name: "a1_a2_unpairable_both",
			Args: []string{"-a", "1", "-a", "2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\nd 4\n",
				"f2.txt": "a X\nc Y\nd Z\n",
			}),
		},
		// R3.1: -a 1 with all lines matching (no effect)
		{
			Name: "a1_all_match",
			Args: []string{"-a", "1", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "a X\nb Y\n",
			}),
		},
		// R3.1: -a 1 with no lines matching
		{
			Name: "a1_no_match",
			Args: []string{"-a", "1", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "c X\nd Y\n",
			}),
		},
		// R3.2: -v 1 shows only unpairable from file1
		{
			Name: "v1_only_unpairable_file1",
			Args: []string{"-v", "1", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\nc 3\n",
				"f2.txt": "a X\nc Z\n",
			}),
		},
		// R3.2: -v 2 shows only unpairable from file2
		{
			Name: "v2_only_unpairable_file2",
			Args: []string{"-v", "2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nc 3\n",
				"f2.txt": "a X\nb Y\nc Z\n",
			}),
		},
		// R3.2: -v 1 -v 2 shows only unpairable from both
		{
			Name: "v1_v2_only_unpairable_both",
			Args: []string{"-v", "1", "-v", "2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\nd 4\n",
				"f2.txt": "a X\nc Y\nd Z\n",
			}),
		},
		// R3.2: -v 1 with no unpairable lines (all match)
		{
			Name: "v1_all_match",
			Args: []string{"-v", "1", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "a X\nb Y\n",
			}),
		},
		// R3.3: -e STRING replaces missing fields with -o format
		{
			Name: "e_replace_missing_a1",
			Args: []string{"-a", "1", "-e", "EMPTY", "-o", "0,1.2,2.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\nc 3\n",
				"f2.txt": "a X\nc Z\n",
			}),
		},
		// R3.3: -e with -a 2
		{
			Name: "e_replace_missing_a2",
			Args: []string{"-a", "2", "-e", "NA", "-o", "0,1.2,2.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nc 3\n",
				"f2.txt": "a X\nb Y\nc Z\n",
			}),
		},
		// R3.3: -e with -a 1 -a 2 both unpairable
		{
			Name: "e_replace_both_unpaired",
			Args: []string{"-a", "1", "-a", "2", "-e", "---", "-o", "0,1.2,2.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\n",
				"f2.txt": "a X\nc Y\n",
			}),
		},
		// R3.4: --header joins first lines unconditionally
		{
			Name: "header_basic",
			Args: []string{"--header", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "KEY VAL1\na 1\nb 2\n",
				"f2.txt": "KEY VAL2\na X\nb Y\n",
			}),
		},
		// R3.4: --header with unsorted header line
		{
			Name: "header_unsorted",
			Args: []string{"--header", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "ZZZ COL1\na 1\nb 2\n",
				"f2.txt": "AAA COL2\na X\nb Y\n",
			}),
		},
		// R3.4: --header with -o format
		{
			Name: "header_with_output_format",
			Args: []string{"--header", "-o", "0,1.2,2.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "KEY VAL1\na 1\nb 2\n",
				"f2.txt": "KEY VAL2\na X\nb Y\n",
			}),
		},
		// R3.4: --header with -t separator
		{
			Name: "header_with_separator",
			Args: []string{"--header", "-t", ",", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "KEY,VAL1\na,1\nb,2\n",
				"f2.txt": "KEY,VAL2\na,X\nb,Y\n",
			}),
		},
		// R3.1 + R3.4: --header combined with -a 1
		{
			Name: "header_with_a1",
			Args: []string{"--header", "-a", "1", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "KEY VAL1\na 1\nb 2\nc 3\n",
				"f2.txt": "KEY VAL2\na X\nc Z\n",
			}),
		},
		// R3.2 + R3.3: -v with -e and -o format
		{
			Name: "v1_with_e_and_o",
			Args: []string{"-v", "1", "-e", "X", "-o", "0,1.2,2.2", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a 1\nb 2\nc 3\n",
				"f2.txt": "a X\nc Z\n",
			}),
		},
		// R3.1: -a 1 with custom separator
		{
			Name: "a1_with_separator",
			Args: []string{"-a", "1", "-t", ",", "f1.txt", "f2.txt"},
			WorkDir: setupDir(t, map[string]string{
				"f1.txt": "a,1\nb,2\nc,3\n",
				"f2.txt": "a,X\nc,Z\n",
			}),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
