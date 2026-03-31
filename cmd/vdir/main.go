// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/vdir implements prd108-vdir: list directory contents in long
// format with C-style escaping of non-printable characters.
// vdir is equivalent to ls -l -b. Accepts all ls flags (R1.6).
// Exit codes: 0 success (R2.1), 1 minor (R2.2), 2 serious (R2.3).
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// outputFormat selects the output layout mode.
type outputFormat int

const (
	formatLong    outputFormat = iota // -l (default for vdir)
	formatColumns                    // -C
	formatSingle                     // -1
	formatAcross                     // -x
	formatComma                      // -m
)

// filterMode selects which entries are visible.
type filterMode int

const (
	filterNoDot     filterMode = iota // default: hide dotfiles
	filterAll                         // -a: show all including . and ..
	filterAlmostAll                   // -A: show dotfiles except . and ..
)

// sortMode selects the primary sort key.
type sortMode int

const (
	sortName      sortMode = iota // default: C locale name sort
	sortTime                      // -t: newest first
	sortSize                      // -S: largest first
	sortNone                      // -U: directory order
	sortVersion                   // -v: version sort
	sortExtension                 // -X: extension sort
)

// colorMode selects color output behavior.
type colorMode int

const (
	colorNever  colorMode = iota // default for vdir
	colorAlways                  // --color=always
	colorAuto                    // --color=auto
)

const (
	exitOK           = 0
	exitMinor        = 1
	exitSerious      = 2
	defaultTermWidth = 80
	progName         = "vdir"
)

// vdirConfig holds all parsed flag state for a single vdir invocation.
type vdirConfig struct {
	format     outputFormat
	filter     filterMode
	sortBy     sortMode
	colorOpt   colorMode
	reverse    bool // -r
	dirOnly    bool // -d
	classify   bool // -F
	recursive  bool // -R
	showInode  bool // -i
	showBlocks bool // -s
	humanSize  bool // -h
	numericIDs bool // -n
	termWidth  int
	formatSet  bool // whether format was explicitly set by a flag
}

// vdirEntry holds metadata for a single directory entry.
type vdirEntry struct {
	name  string
	path  string
	info  *sys.FileInfo
	link  string // symlink target for -l
	isDir bool
}

// R2.4: install SIGPIPE handler at startup.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses flags, executes the listing, and returns the exit code.
func run(args []string) int {
	cfg, paths, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return exitSerious
	}
	applyColorConfig(cfg)
	installResizeHandler(cfg)
	return listPaths(cfg, paths)
}

// parseFlags parses command-line arguments into a vdirConfig and paths.
// R1.5: defaults to "." when no paths are given.
func parseFlags(args []string) (*vdirConfig, []string, error) {
	cfg := &vdirConfig{}
	var paths []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || !strings.HasPrefix(arg, "-") || arg == "-" {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if err := parseLongFlag(cfg, arg[2:]); err != nil {
				return nil, nil, err
			}
			continue
		}
		if err := parseShortFlags(cfg, arg[1:]); err != nil {
			return nil, nil, err
		}
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	resolveFormat(cfg)
	return cfg, paths, nil
}

// parseShortFlags processes a string of single-character flags.
func parseShortFlags(cfg *vdirConfig, flags string) error {
	for _, ch := range flags {
		if err := applyShortFlag(cfg, ch); err != nil {
			return err
		}
	}
	return nil
}

// applyShortFlag applies a single short flag character to the config.
// R1.6: accepts all flags that ls accepts.
func applyShortFlag(cfg *vdirConfig, ch rune) error {
	switch ch {
	case '1':
		cfg.format = formatSingle
		cfg.formatSet = true
	case 'l':
		cfg.format = formatLong
		cfg.formatSet = true
	case 'C':
		cfg.format = formatColumns
		cfg.formatSet = true
	case 'x':
		cfg.format = formatAcross
		cfg.formatSet = true
	case 'm':
		cfg.format = formatComma
		cfg.formatSet = true
	case 'a':
		cfg.filter = filterAll
	case 'A':
		cfg.filter = filterAlmostAll
	case 'b':
		// no-op: escape is always on for vdir
	case 'd':
		cfg.dirOnly = true
	case 'r':
		cfg.reverse = true
	case 't':
		cfg.sortBy = sortTime
	case 'S':
		cfg.sortBy = sortSize
	case 'U':
		cfg.sortBy = sortNone
	case 'v':
		cfg.sortBy = sortVersion
	case 'X':
		cfg.sortBy = sortExtension
	case 'h':
		cfg.humanSize = true
	case 'F':
		cfg.classify = true
	case 'R':
		cfg.recursive = true
	case 'i':
		cfg.showInode = true
	case 's':
		cfg.showBlocks = true
	case 'n':
		cfg.numericIDs = true
		cfg.format = formatLong
		cfg.formatSet = true
	default:
		return fmt.Errorf("invalid option -- '%c'", ch)
	}
	return nil
}

// parseLongFlag handles a single --flag argument (without the -- prefix).
// R1.6: accepts all long flags that ls accepts.
func parseLongFlag(cfg *vdirConfig, name string) error {
	if name == "color" || strings.HasPrefix(name, "color=") {
		return parseColorFlag(cfg, name)
	}
	if name == "reverse" {
		cfg.reverse = true
		return nil
	}
	if strings.HasPrefix(name, "sort=") {
		return parseSortFlag(cfg, name[5:])
	}
	return fmt.Errorf("unrecognized option '--%s'", name)
}

// parseSortFlag parses the value of --sort=WORD.
func parseSortFlag(cfg *vdirConfig, val string) error {
	switch val {
	case "none":
		cfg.sortBy = sortNone
	case "time":
		cfg.sortBy = sortTime
	case "size":
		cfg.sortBy = sortSize
	case "version":
		cfg.sortBy = sortVersion
	case "extension":
		cfg.sortBy = sortExtension
	default:
		return fmt.Errorf("invalid argument '%s' for '--sort'", val)
	}
	return nil
}

// parseColorFlag parses --color[=VALUE].
func parseColorFlag(cfg *vdirConfig, name string) error {
	if name == "color" {
		cfg.colorOpt = colorAlways
		return nil
	}
	val := name[6:] // after "color="
	switch val {
	case "always", "yes", "force":
		cfg.colorOpt = colorAlways
	case "never", "no", "none":
		cfg.colorOpt = colorNever
	case "auto":
		cfg.colorOpt = colorAuto
	default:
		return fmt.Errorf("invalid argument '%s' for '--color'", val)
	}
	return nil
}

// resolveFormat determines terminal width and default format.
// R1.1: vdir defaults to long format regardless of whether stdout is a TTY.
func resolveFormat(cfg *vdirConfig) {
	if sys.IsTerminal(os.Stdout.Fd()) {
		w, err := sys.TerminalWidth()
		if err == nil {
			cfg.termWidth = w
		} else {
			cfg.termWidth = defaultTermWidth
		}
	} else {
		cfg.termWidth = defaultTermWidth
	}
	if !cfg.formatSet {
		cfg.format = formatLong
	}
}

// applyColorConfig sets the process-global color state based on cfg.
func applyColorConfig(cfg *vdirConfig) {
	switch cfg.colorOpt {
	case colorAlways:
		format.SetColorEnabled(true)
	case colorNever:
		format.SetColorEnabled(false)
	case colorAuto:
		format.SetColorEnabled(sys.IsTerminal(os.Stdout.Fd()))
	}
}

// installResizeHandler registers a SIGWINCH callback.
func installResizeHandler(cfg *vdirConfig) {
	sys.OnTerminalResize(func(width int) {
		cfg.termWidth = width
	})
}

// listPaths lists each path argument and returns the exit code.
// R2.1: exit 0 on success. R2.2: accumulates exit status.
func listPaths(cfg *vdirConfig, paths []string) int {
	exitCode := exitOK
	var files []vdirEntry
	var dirs []string
	for _, p := range paths {
		code := classifyArg(cfg, p, &files, &dirs)
		if code > exitCode {
			exitCode = code
		}
	}
	if len(files) > 0 {
		sortEntries(files, cfg)
		formatOutput(cfg, files)
	}
	showHeader := len(paths) > 1 || cfg.recursive
	code := listDirs(cfg, dirs, showHeader, len(files) > 0)
	if code > exitCode {
		exitCode = code
	}
	return exitCode
}

// listDirs iterates over directories and lists each one.
func listDirs(cfg *vdirConfig, dirs []string, hdr, blank bool) int {
	exitCode := exitOK
	for i, d := range dirs {
		if blank || i > 0 {
			fmt.Println()
		}
		code := listDir(cfg, d, hdr)
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

// classifyArg stats a path and classifies it as a file or directory.
// R2.2: stat failure for command-line args is serious (exit 2).
func classifyArg(
	cfg *vdirConfig, path string,
	files *[]vdirEntry, dirs *[]string,
) int {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %s\n",
			progName, path, osErrMsg(err))
		return exitSerious
	}
	if fi.Mode.IsDir() && !cfg.dirOnly {
		*dirs = append(*dirs, path)
		return exitOK
	}
	entry := vdirEntry{
		name: path, path: path, info: fi, isDir: fi.Mode.IsDir(),
	}
	if fi.Mode&os.ModeSymlink != 0 {
		entry.link, _ = os.Readlink(path) // best-effort
	}
	*files = append(*files, entry)
	return exitOK
}

// listDir reads and lists the contents of a single directory.
func listDir(cfg *vdirConfig, dirPath string, showHeader bool) int {
	if showHeader {
		fmt.Printf("%s:\n", dirPath)
	}
	entries, exitCode := readEntries(dirPath)
	if cfg.filter == filterAll {
		entries = addDotEntries(dirPath, entries)
	}
	entries = filterEntries(entries, cfg.filter)
	sortEntries(entries, cfg)
	if cfg.format == formatLong || cfg.showBlocks {
		printTotalLine(cfg, entries)
	}
	formatOutput(cfg, entries)
	if cfg.recursive {
		code := recurseSubdirs(cfg, entries)
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

// readEntries reads directory entries and stats each one.
func readEntries(dirPath string) ([]vdirEntry, int) {
	des, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot open directory '%s': %s\n",
			progName, dirPath, osErrMsg(err))
		return nil, exitSerious
	}
	return statDirEntries(des, dirPath)
}

// statDirEntries stats each directory entry and builds vdirEntry values.
// R2.2: per-entry stat failure is minor (exit 1).
func statDirEntries(des []os.DirEntry, dir string) ([]vdirEntry, int) {
	exitCode := exitOK
	entries := make([]vdirEntry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		path := dir + "/" + name
		fi, err := sys.Lstat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %s\n",
				progName, path, osErrMsg(err))
			exitCode = exitMinor
			continue
		}
		entry := vdirEntry{
			name: name, path: path, info: fi, isDir: fi.Mode.IsDir(),
		}
		if fi.Mode&os.ModeSymlink != 0 {
			entry.link, _ = os.Readlink(path) // best-effort
		}
		entries = append(entries, entry)
	}
	return entries, exitCode
}

// addDotEntries prepends "." and ".." entries for -a mode.
func addDotEntries(dirPath string, entries []vdirEntry) []vdirEntry {
	dots := make([]vdirEntry, 0, 2+len(entries))
	for _, name := range []string{".", ".."} {
		path := dirPath + "/" + name
		fi, err := sys.Lstat(path)
		if err != nil {
			continue
		}
		dots = append(dots, vdirEntry{
			name: name, path: path, info: fi, isDir: fi.Mode.IsDir(),
		})
	}
	return append(dots, entries...)
}

// filterEntries applies the filter mode to remove hidden entries.
// R1.4: hide dotfiles by default.
func filterEntries(entries []vdirEntry, fm filterMode) []vdirEntry {
	if fm == filterAll {
		return entries
	}
	result := make([]vdirEntry, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.name, ".") {
			if fm != filterAlmostAll {
				continue
			}
			if e.name == "." || e.name == ".." {
				continue
			}
		}
		result = append(result, e)
	}
	return result
}

// recurseSubdirs recursively lists subdirectories.
func recurseSubdirs(cfg *vdirConfig, entries []vdirEntry) int {
	exitCode := exitOK
	for _, e := range entries {
		if !e.isDir || e.name == "." || e.name == ".." {
			continue
		}
		if e.info != nil && e.info.Mode&os.ModeSymlink != 0 {
			continue
		}
		fmt.Println()
		code := listDir(cfg, e.path, true)
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

// osErrMsg extracts the OS-level error message from a path error.
func osErrMsg(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return capitalizeFirst(pe.Err.Error())
	}
	return capitalizeFirst(err.Error())
}

// capitalizeFirst uppercases the first ASCII letter of s.
func capitalizeFirst(s string) string {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return s
	}
	return string(s[0]-32) + s[1:]
}
