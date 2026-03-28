// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/vidir implements the moreutils vidir command: edit a directory in a
// text editor. Implements prd114-vidir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// editorFallback is the default editor when $EDITOR and $VISUAL are unset.
const editorFallback = "vi"

// exitEditorFailed is the exit code when the editor exits non-zero.
// R2.1: matches the reference vidir behavior (exit 2 on editor failure).
const exitEditorFailed = 2

// errEditorFailed is a sentinel error for editor non-zero exit.
var errEditorFailed = fmt.Errorf("editor failed")

// entry represents a numbered file in the vidir listing.
type entry struct {
	num  int
	path string
}

func main() {
	sys.InstallSIGPIPEHandler()

	verbose, args := parseArgs(os.Args[1:])

	entries, err := readEntries(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vidir: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		os.Exit(0)
	}

	edited, err := editEntries(entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vidir: %v\n", err)
		os.Exit(exitCodeForError(err))
	}

	if err := checkDuplicatePaths(edited); err != nil {
		fmt.Fprintf(os.Stderr, "vidir: %v\n", err)
		os.Exit(1)
	}

	os.Exit(applyChanges(entries, edited, verbose))
}

// exitCodeForError returns exitEditorFailed if the error wraps errEditorFailed,
// otherwise returns 1.
func exitCodeForError(err error) int {
	if errors.Is(err, errEditorFailed) {
		return exitEditorFailed
	}
	return 1
}

// parseArgs extracts -v/--verbose and returns remaining positional args.
func parseArgs(args []string) (bool, []string) {
	verbose := false
	var positional []string
	for _, a := range args {
		switch a {
		case "-v", "--verbose":
			verbose = true
		default:
			positional = append(positional, a)
		}
	}
	return verbose, positional
}

// readEntries builds the numbered entry list from a directory or stdin.
// R1.1: reads from directory argument, cwd if none given, or stdin if piped.
func readEntries(args []string) ([]entry, error) {
	if !isStdinTerminal() {
		return readEntriesFromStdin()
	}
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	return readEntriesFromDir(dir)
}

// isStdinTerminal reports whether stdin is connected to a terminal.
func isStdinTerminal() bool {
	return sys.IsTerminal(os.Stdin.Fd())
}

// readEntriesFromDir lists files in dir and returns numbered entries.
func readEntriesFromDir(dir string) ([]entry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	entries := make([]entry, 0, len(dirEntries))
	for i, de := range dirEntries {
		path := filepath.Join(dir, de.Name())
		entries = append(entries, entry{num: i + 1, path: path})
	}
	return entries, nil
}

// readEntriesFromStdin reads one path per line from stdin.
func readEntriesFromStdin() ([]entry, error) {
	var entries []entry
	scanner := bufio.NewScanner(os.Stdin)
	num := 1
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		entries = append(entries, entry{num: num, path: line})
		num++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return entries, nil
}

// editEntries writes entries to a temp file, launches the editor, and
// parses the result. Returns the edited entries.
// R1.1: format is "NUMBER\tFILENAME" per line.
// R1.2: launches $EDITOR ($VISUAL, vi fallback) on the temp file.
func editEntries(entries []entry) ([]entry, error) {
	tmpFile, err := os.CreateTemp("", "vidir-*.txt")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // best-effort cleanup

	if err := writeEntriesToFile(tmpFile, entries); err != nil {
		tmpFile.Close() // best-effort close
		return nil, err
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("closing temp file: %w", err)
	}

	if err := launchEditor(tmpPath); err != nil {
		return nil, err
	}

	return parseEditedFile(tmpPath)
}

// writeEntriesToFile writes numbered entries to w.
func writeEntriesToFile(f *os.File, entries []entry) error {
	w := bufio.NewWriter(f)
	for _, e := range entries {
		if _, err := fmt.Fprintf(w, "%d\t%s\n", e.num, e.path); err != nil {
			return fmt.Errorf("writing entry: %w", err)
		}
	}
	return w.Flush()
}

// launchEditor runs the editor on the given file path.
// R2.1: exits 1 when the editor exits non-zero.
func launchEditor(path string) error {
	editor := selectEditor()
	cmd := editorCommand(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", errEditorFailed)
	}
	return nil
}

// selectEditor returns the editor binary to use.
// R1.1: $EDITOR, then $VISUAL, then vi.
func selectEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return editorFallback
}

// editorCommand builds the exec.Cmd for the editor invocation.
func editorCommand(editor, path string) *exec.Cmd {
	return exec.Command(editor, path)
}

// parseEditedFile reads the edited temp file and returns entries.
// R1.2: lines that were removed are detected as deletions.
func parseEditedFile(path string) ([]entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading edited file: %w", err)
	}
	defer f.Close() // best-effort close on read-only file

	var entries []entry
	scanner := bufio.NewScanner(f)
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
		return nil, fmt.Errorf("reading edited file: %w", err)
	}
	return entries, nil
}

// parseLine parses a single "NUMBER\tFILENAME" line.
func parseLine(line string) (entry, error) {
	numStr, path, ok := strings.Cut(line, "\t")
	if !ok {
		return entry{}, fmt.Errorf("malformed line: %q", line)
	}
	num, err := strconv.Atoi(strings.TrimSpace(numStr))
	if err != nil {
		return entry{}, fmt.Errorf("invalid number in line: %q", line)
	}
	return entry{num: num, path: path}, nil
}

// checkDuplicatePaths returns an error if two edited entries share the same path.
// R2.1: duplicate targets are an error condition.
func checkDuplicatePaths(entries []entry) error {
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if seen[e.path] {
			return fmt.Errorf("duplicate target path: %s", e.path)
		}
		seen[e.path] = true
	}
	return nil
}

// applyChanges compares original and edited entries, performs renames
// and deletions. Returns 0 on success, 1 on any failure.
// R1.3: renames before deletions; creates parent dirs for renames.
// R1.4: -v prints actions to stderr.
func applyChanges(original, edited []entry, verbose bool) int {
	origMap := buildEntryMap(original)
	editMap := buildEntryMap(edited)

	renames := collectRenames(origMap, editMap)
	deletions := collectDeletions(origMap, editMap)

	exitCode := 0
	exitCode = applyRenames(renames, verbose, exitCode)
	exitCode = applyDeletions(deletions, verbose, exitCode)
	return exitCode
}

// buildEntryMap creates a map from entry number to path.
func buildEntryMap(entries []entry) map[int]string {
	m := make(map[int]string, len(entries))
	for _, e := range entries {
		m[e.num] = e.path
	}
	return m
}

// collectRenames finds entries whose paths changed.
func collectRenames(orig, edited map[int]string) []rename {
	var renames []rename
	for num, newPath := range edited {
		oldPath, ok := orig[num]
		if !ok || oldPath == newPath {
			continue
		}
		renames = append(renames, rename{from: oldPath, to: newPath})
	}
	sort.Slice(renames, func(i, j int) bool {
		return renames[i].from < renames[j].from
	})
	return renames
}

// rename represents a file rename operation.
type rename struct {
	from string
	to   string
}

// collectDeletions finds entries removed from the edited listing.
func collectDeletions(orig, edited map[int]string) []string {
	var dels []string
	for num, path := range orig {
		if _, ok := edited[num]; !ok {
			dels = append(dels, path)
		}
	}
	sort.Strings(dels)
	return dels
}

// applyRenames performs rename operations, creating parent dirs as needed.
func applyRenames(renames []rename, verbose bool, code int) int {
	for _, r := range renames {
		if err := mkdirForRename(r.to); err != nil {
			fmt.Fprintf(os.Stderr, "vidir: mkdir %s: %v\n", filepath.Dir(r.to), err)
			code = 1
			continue
		}
		if err := os.Rename(r.from, r.to); err != nil {
			fmt.Fprintf(os.Stderr, "vidir: rename %s -> %s: %v\n", r.from, r.to, err)
			code = 1
			continue
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "renamed %s -> %s\n", r.from, r.to)
		}
	}
	return code
}

// mkdirForRename creates the parent directory of dst if it doesn't exist.
func mkdirForRename(dst string) error {
	dir := filepath.Dir(dst)
	return os.MkdirAll(dir, 0o755)
}

// applyDeletions removes files from the filesystem.
func applyDeletions(paths []string, verbose bool, code int) int {
	for _, p := range paths {
		if err := os.Remove(p); err != nil {
			fmt.Fprintf(os.Stderr, "vidir: remove %s: %v\n", p, err)
			code = 1
			continue
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "removed %s\n", p)
		}
	}
	return code
}
