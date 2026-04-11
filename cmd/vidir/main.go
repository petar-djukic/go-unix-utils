// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/vidir: edit a directory in a text editor.
// Implements srd114-vidir R1.1-R1.4, R2.1-R2.2.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "vidir"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// options holds parsed command-line flags.
type options struct {
	verbose bool // R1.4: -v/--verbose prints actions to stderr
}

// entry represents a numbered file in the directory listing.
type entry struct {
	num  int
	path string
}

// run parses flags, collects files, and orchestrates editing.
// R1.1-R1.4: core vidir behavior.
func run(args []string) int {
	opts, paths, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	files, err := collectFiles(paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	return editAndApply(files, opts)
}

// parseFlags extracts -v/--verbose from args.
// Returns parsed options, remaining positional args, and error.
func parseFlags(args []string) (options, []string, error) {
	var opts options
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if !strings.HasPrefix(arg, "-") {
			break
		}
		switch arg {
		case "-v", "--verbose":
			opts.verbose = true
		default:
			return opts, nil, fmt.Errorf("unknown option: %s", arg)
		}
		i++
	}
	return opts, args[i:], nil
}

// collectFiles resolves paths from args, stdin, or current directory.
// R1.1: list from args, stdin (if piped), or current directory.
func collectFiles(paths []string) ([]string, error) {
	if len(paths) > 0 {
		return expandPaths(paths)
	}
	if !sys.IsTerminal(os.Stdin.Fd()) {
		return readStdinPaths()
	}
	return listDir(".")
}

// readStdinPaths reads file paths from stdin, one per line.
// R1.1: stdin mode when input is piped.
func readStdinPaths() ([]string, error) {
	var paths []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths, scanner.Err()
}

// listDir returns sorted entries of a directory as prefixed paths.
func listDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, dir+"/"+e.Name())
	}
	return names, nil
}

// expandPaths expands each path: directories become their contents,
// files pass through as-is.
func expandPaths(paths []string) ([]string, error) {
	var result []string
	for _, p := range paths {
		expanded, err := expandOnePath(p)
		if err != nil {
			return nil, err
		}
		result = append(result, expanded...)
	}
	return result, nil
}

// expandOnePath returns directory contents if p is a directory,
// or p itself otherwise.
func expandOnePath(p string) ([]string, error) {
	info, err := os.Stat(p)
	if err != nil {
		// File may not exist yet; include as-is (matches Perl behavior).
		return []string{p}, nil
	}
	if !info.IsDir() {
		return []string{p}, nil
	}
	return listDir(p)
}

// editAndApply writes the listing, opens the editor, and applies changes.
func editAndApply(files []string, opts options) int {
	original := makeEntries(files)
	tmpPath, err := writeTempFile(original)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	defer os.Remove(tmpPath)
	if err := openEditor(tmpPath); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	return processEdits(original, tmpPath, opts)
}

// makeEntries assigns sequential numbers starting at 1 to each file.
func makeEntries(files []string) []entry {
	entries := make([]entry, len(files))
	for i, f := range files {
		entries[i] = entry{num: i + 1, path: f}
	}
	return entries
}

// writeTempFile writes the numbered listing to a temporary file.
// R1.1: format is "NUMBER\tFILENAME\n".
func writeTempFile(entries []entry) (string, error) {
	f, err := os.CreateTemp("", "vidir.")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	name := f.Name()
	for _, e := range entries {
		if _, err := fmt.Fprintf(f, "%d\t%s\n", e.num, e.path); err != nil {
			f.Close()
			os.Remove(name)
			return "", fmt.Errorf("writing temp file: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	return name, nil
}

// resolveEditor returns the editor command from EDITOR, VISUAL, or "vi".
// R1.2: EDITOR takes precedence, then VISUAL, then vi.
func resolveEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	return "vi"
}

// openEditor launches the editor on the given file and waits for exit.
// R1.2: open temp file in user's editor.
// R2.1: returns error if editor exits non-zero.
func openEditor(path string) error {
	editor := resolveEditor()
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}
	return nil
}

// processEdits parses the edited file and applies changes.
func processEdits(original []entry, tmpPath string, opts options) int {
	edited, err := parseEditedFile(tmpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	return applyChanges(original, edited, opts)
}

// parseEditedFile reads the edited temp file and returns num → path map.
// R1.2/R1.3: parses numbered lines to detect renames and deletions.
func parseEditedFile(path string) (map[int]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading edited file: %w", err)
	}
	defer f.Close()
	result := make(map[int]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		num, name, ok := parseLine(scanner.Text())
		if ok {
			result[num] = name
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning edited file: %w", err)
	}
	return result, nil
}

// parseLine extracts the number and filename from a "NUMBER\tFILENAME" line.
func parseLine(line string) (int, string, bool) {
	numStr, name, ok := strings.Cut(line, "\t")
	if !ok {
		return 0, "", false
	}
	num, err := strconv.Atoi(strings.TrimSpace(numStr))
	if err != nil {
		return 0, "", false
	}
	return num, name, true
}

// applyChanges processes renames then deletions.
// R1.3: renames before deletions to avoid conflicts.
func applyChanges(original []entry, edited map[int]string, opts options) int {
	exitCode := 0
	if code := applyRenames(original, edited, opts); code != 0 {
		exitCode = code
	}
	if code := applyDeletions(original, edited, opts); code != 0 {
		exitCode = code
	}
	return exitCode
}

// applyRenames renames files whose paths changed in the editor.
// R1.3: create parent directories for renames to new paths.
// R1.4: verbose mode prints each rename to stderr.
func applyRenames(original []entry, edited map[int]string, opts options) int {
	exitCode := 0
	for _, e := range original {
		newPath, ok := edited[e.num]
		if !ok || newPath == e.path {
			continue
		}
		if opts.verbose {
			fmt.Fprintf(os.Stderr, "'%s' -> '%s'\n", e.path, newPath)
		}
		if err := doRename(e.path, newPath); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// doRename creates parent directories and renames src to dst.
// R1.3: create parent directories for renames.
func doRename(src, dst string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	return os.Rename(src, dst)
}

// applyDeletions removes files whose lines were deleted from the editor.
// R1.4: verbose mode prints each deletion to stderr.
func applyDeletions(original []entry, edited map[int]string, opts options) int {
	exitCode := 0
	for _, e := range original {
		if _, ok := edited[e.num]; ok {
			continue
		}
		if opts.verbose {
			fmt.Fprintf(os.Stderr, "removed '%s'\n", e.path)
		}
		if err := os.Remove(e.path); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}
