// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/vidir implements moreutils vidir: edit a directory in a text editor.
//
// Implements prd114-vidir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "vidir"

// defaultEditor is the fallback editor when $EDITOR is not set.
const defaultEditor = "vi"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stderr))
}

// config holds parsed flag state.
type config struct {
	verbose bool
	args    []string
}

// parseArgs extracts flags and positional arguments.
func parseArgs(args []string) config {
	c := config{}
	for _, arg := range args {
		switch arg {
		case "-v", "--verbose":
			c.verbose = true
		default:
			c.args = append(c.args, arg)
		}
	}
	return c
}

// entry represents one file in the numbered listing.
type entry struct {
	id   int
	name string
}

// run executes the vidir logic. Returns exit code.
func run(args []string, stdin io.Reader, stderr io.Writer) int {
	cfg := parseArgs(args)
	original, err := buildListing(cfg.args, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1
	}
	edited, err := editListing(original)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1
	}
	if err := applyChanges(original, edited, cfg.verbose, stderr); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1
	}
	return 0
}

// buildListing produces the initial file listing.
// R1.1: lists files from directory args or reads from stdin if piped.
func buildListing(args []string, stdin io.Reader) ([]entry, error) {
	if isStdinPipe() {
		return readListingFromStdin(stdin)
	}
	dirs := args
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	return listDirectories(dirs)
}

// isStdinPipe reports whether stdin is a pipe (FIFO), matching Perl's -p check.
func isStdinPipe() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeNamedPipe != 0
}

// listDirectories lists all entries from the given directories.
func listDirectories(dirs []string) ([]entry, error) {
	var entries []entry
	id := 1
	for _, dir := range dirs {
		dirEntries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("reading directory %s: %w", dir, err)
		}
		for _, de := range dirEntries {
			name := filepath.Join(dir, de.Name())
			entries = append(entries, entry{id: id, name: name})
			id++
		}
	}
	return entries, nil
}

// readListingFromStdin reads filenames from stdin, one per line.
func readListingFromStdin(r io.Reader) ([]entry, error) {
	var entries []entry
	scanner := bufio.NewScanner(r)
	id := 1
	for scanner.Scan() {
		name := scanner.Text()
		if name != "" {
			entries = append(entries, entry{id: id, name: name})
			id++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return entries, nil
}

// formatListing writes the numbered listing to w.
// R1.1: each line formatted as "NUMBER\tFILENAME".
func formatListing(w io.Writer, entries []entry) error {
	for _, e := range entries {
		if _, err := fmt.Fprintf(w, "%d\t%s\n", e.id, e.name); err != nil {
			return err
		}
	}
	return nil
}

// editListing writes the listing to a temp file, opens the editor, and parses
// the result.
// R1.1: opens listing in $EDITOR (default vi).
func editListing(original []entry) ([]entry, error) {
	tmpFile, err := writeTempListing(original)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile) // best-effort cleanup
	if err := runEditor(tmpFile); err != nil {
		return nil, err
	}
	return parseTempListing(tmpFile)
}

// writeTempListing creates a temp file with the numbered listing.
func writeTempListing(entries []entry) (string, error) {
	f, err := os.CreateTemp("", "vidir-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	if err := formatListing(f, entries); err != nil {
		f.Close()            // best-effort close
		os.Remove(f.Name()) // best-effort cleanup
		return "", fmt.Errorf("writing listing: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	return name, nil
}

// runEditor launches $EDITOR on the given file.
// R2.1: returns error if editor exits non-zero.
func runEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = defaultEditor
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}
	return nil
}

// parseTempListing reads the edited temp file and returns the parsed entries.
func parseTempListing(path string) ([]entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading edited listing: %w", err)
	}
	defer f.Close() // best-effort close
	return parseListing(f)
}

// parseListing parses a numbered listing from r.
func parseListing(r io.Reader) ([]entry, error) {
	var entries []entry
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		e, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading listing: %w", err)
	}
	return entries, nil
}

// parseLine parses a single "NUMBER\tFILENAME" line.
func parseLine(line string) (entry, error) {
	numStr, name, ok := strings.Cut(line, "\t")
	if !ok {
		return entry{}, fmt.Errorf("malformed line: %q", line)
	}
	id, err := strconv.Atoi(numStr)
	if err != nil {
		return entry{}, fmt.Errorf("invalid id in line: %q", line)
	}
	return entry{id: id, name: name}, nil
}

// applyChanges compares original and edited listings and applies renames and
// deletions.
// R1.2: deleted lines → remove file; changed names → rename file.
// R1.3: process renames before deletions.
func applyChanges(original, edited []entry, verbose bool, stderr io.Writer) error {
	origMap := buildIDMap(original)
	editedMap := buildIDMap(edited)

	if err := processRenames(origMap, editedMap, verbose, stderr); err != nil {
		return err
	}
	return processDeletions(origMap, editedMap, verbose, stderr)
}

// buildIDMap creates a map from entry id to filename.
func buildIDMap(entries []entry) map[int]string {
	m := make(map[int]string, len(entries))
	for _, e := range entries {
		m[e.id] = e.name
	}
	return m
}

// processRenames handles entries whose names changed.
// R1.3: creates parent directories for renames that move files.
// R1.4: prints rename actions to stderr when verbose.
func processRenames(orig, edited map[int]string, verbose bool, stderr io.Writer) error {
	for id, newName := range edited {
		oldName, ok := orig[id]
		if !ok || oldName == newName {
			continue
		}
		if err := renameFile(oldName, newName, verbose, stderr); err != nil {
			return err
		}
	}
	return nil
}

// renameFile renames a file, creating parent directories as needed.
func renameFile(oldName, newName string, verbose bool, stderr io.Writer) error {
	dir := filepath.Dir(newName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	if err := os.Rename(oldName, newName); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", oldName, newName, err)
	}
	if verbose {
		fmt.Fprintf(stderr, "%s -> %s\n", oldName, newName)
	}
	return nil
}

// processDeletions removes files that were deleted from the listing.
// R1.4: prints delete actions to stderr when verbose.
func processDeletions(orig, edited map[int]string, verbose bool, stderr io.Writer) error {
	for id, name := range orig {
		if _, ok := edited[id]; ok {
			continue
		}
		if err := os.Remove(name); err != nil {
			return fmt.Errorf("removing %s: %w", name, err)
		}
		if verbose {
			fmt.Fprintf(stderr, "removed %s\n", name)
		}
	}
	return nil
}
