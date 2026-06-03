// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd114-vidir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2.
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

var verbose bool

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	args = parseFlags(args)
	files, err := collectFiles(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vidir: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		return 0
	}
	original := buildListing(files)
	edited, err := editListing(original)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vidir: %v\n", err)
		return 1
	}
	return applyChanges(original, edited)
}

func parseFlags(args []string) []string {
	var rest []string
	for _, a := range args {
		switch a {
		case "-v", "--verbose":
			verbose = true
		default:
			rest = append(rest, a)
		}
	}
	return rest
}

func collectFiles(args []string) ([]string, error) {
	if !isTerminalStdin() {
		return readStdinFiles()
	}
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	return readDirFiles(dir)
}

func isTerminalStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return true
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func readStdinFiles() ([]string, error) {
	var files []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			files = append(files, line)
		}
	}
	return files, scanner.Err()
}

func readDirFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		files = append(files, filepath.Join(dir, e.Name()))
	}
	return files, nil
}

type entry struct {
	id   int
	name string
}

func buildListing(files []string) []entry {
	entries := make([]entry, len(files))
	for i, f := range files {
		entries[i] = entry{id: i + 1, name: f}
	}
	return entries
}

func editListing(entries []entry) ([]entry, error) {
	tmp, err := writeTempFile(entries)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)
	if err := runEditor(tmp); err != nil {
		return nil, err
	}
	return parseListing(tmp)
}

func writeTempFile(entries []entry) (string, error) {
	f, err := os.CreateTemp("", "vidir-*.txt")
	if err != nil {
		return "", err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, e := range entries {
		fmt.Fprintf(w, "%d\t%s\n", e.id, e.name)
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func runEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := makeEditorCmd(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func parseListing(path string) ([]entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var entries []entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		e, err := parseLine(scanner.Text())
		if err != nil {
			return nil, err
		}
		if e != nil {
			entries = append(entries, *e)
		}
	}
	return entries, scanner.Err()
}

func parseLine(line string) (*entry, error) {
	if strings.TrimSpace(line) == "" {
		return nil, nil
	}
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid line: %s", line)
	}
	id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid id: %s", parts[0])
	}
	return &entry{id: id, name: parts[1]}, nil
}

func applyChanges(original, edited []entry) int {
	editedMap := make(map[int]string, len(edited))
	for _, e := range edited {
		editedMap[e.id] = e.name
	}
	code := processRenames(original, editedMap)
	if c := processDeletions(original, editedMap); c != 0 {
		code = c
	}
	return code
}

func processRenames(original []entry, editedMap map[int]string) int {
	code := 0
	for _, orig := range original {
		newName, exists := editedMap[orig.id]
		if !exists || newName == orig.name {
			continue
		}
		if err := renameFile(orig.name, newName); err != nil {
			fmt.Fprintf(os.Stderr, "vidir: %v\n", err)
			code = 1
		}
	}
	return code
}

func renameFile(oldName, newName string) error {
	dir := filepath.Dir(newName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "%s -> %s\n", oldName, newName)
	}
	return os.Rename(oldName, newName)
}

func makeEditorCmd(editor, path string) *exec.Cmd {
	return exec.Command(editor, path)
}

func processDeletions(original []entry, editedMap map[int]string) int {
	code := 0
	for _, orig := range original {
		if _, exists := editedMap[orig.id]; exists {
			continue
		}
		name := orig.name
		if verbose {
			fmt.Fprintf(os.Stderr, "delete %s\n", name)
		}
		if err := os.Remove(name); err != nil {
			fmt.Fprintf(os.Stderr, "vidir: %v\n", err)
			code = 1
		}
	}
	return code
}
