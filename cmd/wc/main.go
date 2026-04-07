// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/wc: count lines, words, and bytes.
// Implements srd005-wc contract: exported types and function signatures.
package main

import (
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// countResult holds per-file or aggregate counting results.
// R1, R2: fields correspond to wc output columns.
type countResult struct {
	lines         int64 // R2: -l newline count
	words         int64 // R2: -w word count
	bytes         int64 // R2: -c byte count
	chars         int64 // R2: -m character count
	maxLineLength int64 // R2: -L longest line length
}

// config captures all parsed flag states for wc invocation.
// R2, R4: maps CLI flags to runtime configuration.
type config struct {
	showLines     bool   // -l
	showWords     bool   // -w
	showBytes     bool   // -c
	showChars     bool   // -m
	showMaxLine   bool   // -L
	files0From    string // --files0-from=FILE
}

// parseArgs parses command-line arguments and returns a config and file list.
// R2: maps flag package results to config struct.
func parseArgs() (config, []string) {
	return config{}, nil
}

// countFile counts lines, words, bytes, chars, and max line length for a file.
// R1, R2: delegates to counting logic per the active config.
func countFile(_ string, _ config) (countResult, error) {
	return countResult{}, nil
}

// countStdin counts lines, words, bytes, chars, and max line length from stdin.
// R4: handles stdin as an unnamed input source.
func countStdin(_ config) (countResult, error) {
	return countResult{}, nil
}

// formatOutput formats a countResult as a display line.
// R3: right-aligns counts and appends the filename.
func formatOutput(_ countResult, _ string, _ config, _ int) string {
	return ""
}

// processFiles iterates over file arguments and accumulates results.
// R1, R3, R6: processes each file, prints output, and returns exit code.
func processFiles(_ []string, _ config) int {
	return 0
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, files := parseArgs()
	os.Exit(processFiles(files, cfg))
}
