// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd008-ls R1.1-R1.12, R2.1-R2.12: directory listing with output
// modes, sorting flags (-t, -S, -r, -U), filtering (-a, -A, -d), metadata
// display (-s, -i), human-readable sizes (-h), and recursive listing (-R).
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// outputMode selects the listing format.
type outputMode int

const (
	modeDefault outputMode = iota
	modeSingle             // -1: one entry per line
	modeLong               // -l: long format
	modeColumns            // -C: forced multi-column (vertical)
	modeHorizontal         // -x: forced multi-column (horizontal)
)

// filterMode selects which entries to show.
type filterMode int

const (
	filterDefault   filterMode = iota // hide dot-entries
	filterAlmostAll                   // -A: show dot-entries except . and ..
	filterAll                         // -a: show all including . and ..
)

// sortMode selects the sort order.
type sortMode int

const (
	sortName sortMode = iota // default: alphabetical C locale
	sortTime                 // -t: by modification time, newest first
	sortSize                 // -S: by file size, largest first
	sortNone                 // -U: directory order (no sort)
)

// lsConfig holds parsed command-line options.
type lsConfig struct {
	output        outputMode
	filter        filterMode
	sorting       sortMode
	reverse       bool // -r: reverse sort order
	humanReadable bool // -h: human-readable sizes
	showBlocks    bool // -s: show allocated block count
	showInode     bool // -i: show inode number
	dirOnly       bool // -d: list directories themselves
	recursive     bool // -R: recurse into subdirectories
	args          []string
}

// entry holds a directory entry with cached metadata.
type entry struct {
	name string
	path string
	info *sys.FileInfo // nil if stat failed
}

func main() {
	sys.InstallSIGPIPEHandler()
	// R1.3: C locale sort order.
	os.Setenv("LC_ALL", "C")

	cfg := parseArgs(os.Args[1:])
	os.Exit(run(cfg))
}

// parseArgs extracts flags and positional arguments.
func parseArgs(args []string) lsConfig {
	var cfg lsConfig
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			break
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println("ls (go-unix-utils) dev")
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "-") && arg != "-" && !strings.HasPrefix(arg, "--") {
			parseShortFlags(arg[1:], &cfg)
			continue
		}
		if handleLongFlag(arg, &cfg) {
			continue
		}
		cfg.args = append(cfg.args, arg)
	}
	if len(cfg.args) == 0 {
		cfg.args = []string{"."}
	}
	return cfg
}

// parseShortFlags processes a cluster of short flags (e.g., "-laRtSr").
func parseShortFlags(flags string, cfg *lsConfig) {
	for _, ch := range flags {
		switch ch {
		case 'a':
			cfg.filter = filterAll
		case 'A':
			cfg.filter = filterAlmostAll
		case 'l':
			cfg.output = modeLong
		case '1':
			cfg.output = modeSingle
		case 'C':
			cfg.output = modeColumns
		case 'x':
			cfg.output = modeHorizontal
		case 'R':
			cfg.recursive = true
		case 't':
			cfg.sorting = sortTime
		case 'S':
			cfg.sorting = sortSize
		case 'r':
			cfg.reverse = true
		case 'U':
			cfg.sorting = sortNone
		case 'h':
			cfg.humanReadable = true
		case 's':
			cfg.showBlocks = true
		case 'i':
			cfg.showInode = true
		case 'd':
			cfg.dirOnly = true
		}
	}
}

// handleLongFlag processes long-form flags. Returns true if recognized.
func handleLongFlag(arg string, cfg *lsConfig) bool {
	switch arg {
	case "--all":
		cfg.filter = filterAll
	case "--almost-all":
		cfg.filter = filterAlmostAll
	case "--recursive":
		cfg.recursive = true
	case "--reverse":
		cfg.reverse = true
	case "--human-readable":
		cfg.humanReadable = true
	default:
		return false
	}
	return true
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: ls [OPTION]... [FILE]...
List information about the FILEs (the current directory by default).

  -a, --all            do not ignore entries starting with .
  -A, --almost-all     do not list implied . and ..
  -C                   list entries by columns
  -d                   list directories themselves, not their contents
  -h, --human-readable print sizes in human-readable format
  -i                   print the index number of each file
  -l                   use a long listing format
  -r, --reverse        reverse order while sorting
  -R, --recursive      list subdirectories recursively
  -s                   print the allocated size of each file
  -S                   sort by file size, largest first
  -t                   sort by time, newest first
  -U                   do not sort; list entries in directory order
  -x                   list entries by lines instead of by columns
  -1                   list one file per line
      --help           display this help and exit
      --version        output version information and exit
`)
}

// run processes all arguments and returns the exit code.
func run(cfg lsConfig) int {
	if cfg.dirOnly {
		return runDirOnly(cfg)
	}
	return runNormal(cfg)
}

// runDirOnly handles -d: list entries themselves without descending.
// R2.3: directories are listed as entries, not their contents.
func runDirOnly(cfg lsConfig) int {
	entries := make([]entry, 0, len(cfg.args))
	exitCode := 0
	for _, arg := range cfg.args {
		fi, err := sys.Lstat(arg)
		if err != nil {
			reportError("cannot access", arg, err)
			exitCode = 1
			continue
		}
		entries = append(entries, entry{name: arg, path: arg, info: fi})
	}
	sortEntries(entries, cfg)
	if code := displayEntries(entries, cfg); code != 0 {
		exitCode = 1
	}
	return exitCode
}

// runNormal processes arguments separating files and directories.
func runNormal(cfg lsConfig) int {
	exitCode := 0
	var fileEntries, dirEntries []entry
	for _, arg := range cfg.args {
		fi, err := sys.Stat(arg)
		if err != nil {
			reportError("cannot access", arg, err)
			exitCode = 1
			continue
		}
		e := entry{name: arg, path: arg, info: fi}
		if fi.Mode.IsDir() {
			dirEntries = append(dirEntries, e)
		} else {
			fileEntries = append(fileEntries, e)
		}
	}
	sortEntries(fileEntries, cfg)
	sortEntries(dirEntries, cfg)

	if len(fileEntries) > 0 {
		if code := displayEntries(fileEntries, cfg); code != 0 {
			exitCode = 1
		}
	}

	needBlank := len(fileEntries) > 0
	showHeader := len(fileEntries) > 0 || len(dirEntries) > 1
	exitCode |= listDirs(dirEntries, cfg, needBlank, showHeader)
	return exitCode
}

// listDirs lists multiple directory arguments in order.
func listDirs(dirs []entry, cfg lsConfig, needBlank, showHeader bool) int {
	exitCode := 0
	for _, de := range dirs {
		if needBlank {
			fmt.Println()
		}
		if showHeader || cfg.recursive {
			fmt.Printf("%s:\n", de.name)
		}
		if code := listDir(de.path, cfg); code != 0 {
			exitCode = 1
		}
		needBlank = true
	}
	return exitCode
}

// listDir lists the contents of a single directory.
func listDir(dir string, cfg lsConfig) int {
	rawEntries, err := os.ReadDir(dir)
	if err != nil {
		reportError("cannot open directory", dir, err)
		return 1
	}

	entries := buildDirEntries(dir, rawEntries, cfg.filter)
	if needsStat(cfg) {
		statEntriesInPlace(entries)
	}
	sortEntries(entries, cfg)

	exitCode := displayDirEntries(entries, cfg)

	if cfg.recursive {
		exitCode |= recurseSubdirs(dir, entries, cfg)
	}
	return exitCode
}

// buildDirEntries creates entry structs from directory entries with filtering.
// R1.4: default hides dot-entries.
// R2.1: -a shows all including . and ..
// R2.2: -A shows dot-entries except . and ..
func buildDirEntries(dir string, raw []os.DirEntry, filter filterMode) []entry {
	var entries []entry
	if filter == filterAll {
		entries = append(entries,
			entry{name: ".", path: joinPath(dir, ".")},
			entry{name: "..", path: joinPath(dir, "..")},
		)
	}
	for _, e := range raw {
		name := e.Name()
		if filter == filterDefault && strings.HasPrefix(name, ".") {
			continue
		}
		entries = append(entries, entry{name: name, path: joinPath(dir, name)})
	}
	return entries
}

// needsStat returns true when entry metadata is required for sorting or display.
func needsStat(cfg lsConfig) bool {
	return cfg.output == modeLong ||
		cfg.sorting == sortTime ||
		cfg.sorting == sortSize ||
		cfg.showBlocks ||
		cfg.showInode ||
		cfg.recursive
}

// statEntriesInPlace populates the info field for each entry via Lstat.
func statEntriesInPlace(entries []entry) {
	for i := range entries {
		fi, err := sys.Lstat(entries[i].path)
		if err == nil {
			entries[i].info = fi
		}
	}
}

// sortEntries sorts entries according to the configured sort mode.
// R2.5: -t sorts by mtime newest first.
// R2.6: -S sorts by size largest first.
// R2.7: -r reverses the sort order.
// R2.8: -U disables sorting.
func sortEntries(entries []entry, cfg lsConfig) {
	if cfg.sorting == sortNone || len(entries) <= 1 {
		return
	}
	cmp := selectComparator(cfg.sorting)
	if cfg.reverse {
		orig := cmp
		cmp = func(a, b entry) bool { return orig(b, a) }
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return cmp(entries[i], entries[j])
	})
}

// selectComparator returns the less function for the given sort mode.
func selectComparator(mode sortMode) func(a, b entry) bool {
	switch mode {
	case sortTime:
		return compareByTime
	case sortSize:
		return compareBySize
	default:
		return compareByName
	}
}

// compareByName sorts alphabetically by name in C locale order.
func compareByName(a, b entry) bool {
	return a.name < b.name
}

// compareByTime sorts by modification time, newest first.
// R2.5: ties broken by name in C locale order.
func compareByTime(a, b entry) bool {
	if a.info == nil || b.info == nil {
		return compareByName(a, b)
	}
	if !a.info.ModTime.Equal(b.info.ModTime) {
		return a.info.ModTime.After(b.info.ModTime)
	}
	return a.name < b.name
}

// compareBySize sorts by file size, largest first.
// R2.6: ties broken by name in C locale order.
func compareBySize(a, b entry) bool {
	if a.info == nil || b.info == nil {
		return compareByName(a, b)
	}
	if a.info.Size != b.info.Size {
		return a.info.Size > b.info.Size
	}
	return a.name < b.name
}

// recurseSubdirs handles -R recursive listing for subdirectories.
// R3.13: does not follow symbolic links to directories.
func recurseSubdirs(dir string, entries []entry, cfg lsConfig) int {
	exitCode := 0
	for _, e := range entries {
		if e.name == "." || e.name == ".." {
			continue
		}
		if e.info == nil || !e.info.Mode.IsDir() {
			continue
		}
		fmt.Println()
		fmt.Printf("%s:\n", e.path)
		if code := listDir(e.path, cfg); code != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// joinPath concatenates parent and child, avoiding double slashes.
func joinPath(parent, child string) string {
	if strings.HasSuffix(parent, "/") {
		return parent + child
	}
	return parent + "/" + child
}

// reportError prints a diagnostic to stderr matching GNU ls format.
func reportError(action, path string, err error) {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "ls: %s '%s': %s\n", action, path, msg)
}
