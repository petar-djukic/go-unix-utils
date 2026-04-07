// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/wc: count lines, words, and bytes.
// Implements srd005-wc: R1.1 (line count), R1.2 (word count),
// R1.3 (byte count), R1.4 (character count).
package main

import (
	"bufio"
	"io"
	"os"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// bufSize is the read buffer size for counting operations.
const bufSize = 32 * 1024

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
	showLines   bool   // -l
	showWords   bool   // -w
	showBytes   bool   // -c
	showChars   bool   // -m
	showMaxLine bool   // -L
	files0From  string // --files0-from=FILE
}

// parseArgs parses command-line arguments and returns a config and file list.
// R2: maps flag package results to config struct.
func parseArgs() (config, []string) {
	return config{}, nil
}

// countReader counts lines, words, bytes, and chars from r in a single pass.
// R1.1: lines are newline characters (\n).
// R1.2: words are maximal sequences of non-whitespace characters.
// R1.3: bytes are total bytes read.
// R1.4: chars are UTF-8 code points; invalid bytes each count as one character.
func countReader(r io.Reader) (countResult, error) {
	reader := bufio.NewReaderSize(r, bufSize)
	var result countResult
	inWord := false

	for {
		buf, err := reader.Peek(bufSize)
		if len(buf) == 0 && err == io.EOF {
			break
		}
		if len(buf) == 0 && err != nil {
			return result, err
		}
		n := len(buf)
		// Discard what we peeked so the next Peek advances.
		_, _ = reader.Discard(n) // always succeeds after Peek

		result.bytes += int64(n)
		countChunk(&result, buf, &inWord)

		if err == io.EOF {
			break
		}
	}
	return result, nil
}

// countChunk updates result with counts from a single buffer chunk.
// Tracks word boundary state via inWord across chunk boundaries.
func countChunk(result *countResult, buf []byte, inWord *bool) {
	i := 0
	for i < len(buf) {
		b := buf[i]
		if b == '\n' {
			result.lines++
			result.chars++
			if *inWord {
				*inWord = false
			}
			i++
			continue
		}
		if isWSByte(b) {
			if *inWord {
				*inWord = false
			}
			result.chars++
			i++
			continue
		}
		// Non-whitespace byte: may start or continue a word.
		if !*inWord {
			*inWord = true
			result.words++
		}
		// R1.4: count characters as UTF-8 code points.
		if b < utf8.RuneSelf {
			// ASCII: one byte, one character.
			result.chars++
			i++
		} else {
			_, size := utf8.DecodeRune(buf[i:])
			result.chars++
			i += size
		}
	}
}

// isWSByte returns true if b is a C isspace() whitespace character
// (space, tab, newline, carriage return, form feed, vertical tab).
// Newline is handled separately by the caller, so this covers the rest.
func isWSByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\f' || b == '\v'
}

// countFile counts lines, words, bytes, chars, and max line length for a file.
// R1, R2: delegates to countReader for counting logic.
func countFile(name string, _ config) (countResult, error) {
	f, err := os.Open(name)
	if err != nil {
		return countResult{}, err
	}
	defer f.Close()
	return countReader(f)
}

// countStdin counts lines, words, bytes, chars, and max line length from stdin.
// R4: handles stdin as an unnamed input source.
func countStdin(_ config) (countResult, error) {
	return countReader(os.Stdin)
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
