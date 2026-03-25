// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd108-vdir R1.1-R1.6, R2.1-R2.4: verbose directory listing
// equivalent to ls -l -b. Accepts all ls flags with long format and C-style
// escaping as defaults. Exit codes: 0 success, 1 minor error, 2 serious error.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// outputMode selects the listing format.
type outputMode int

const (
	modeDefault    outputMode = iota
	modeSingle                // -1: one entry per line
	modeLong                  // -l: long format
	modeColumns               // -C: forced multi-column (vertical)
	modeHorizontal            // -x: forced multi-column (horizontal)
	modeComma                 // -m: comma-separated, width-wrapped
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
	sortName    sortMode = iota // default: alphabetical C locale
	sortTime                    // -t: by modification time, newest first
	sortSize                    // -S: by file size, largest first
	sortNone                    // -U: directory order (no sort)
	sortVersion                 // -v: natural version sort
)

// indicatorMode selects which type indicators to append.
type indicatorMode int

const (
	indicatorNone    indicatorMode = iota // no indicator
	indicatorClassify                     // -F: full type classification
	indicatorSlash                        // -p: / for directories only
)

// colorMode selects the --color behavior.
type colorMode int

const (
	colorAuto   colorMode = iota // --color=auto or default
	colorAlways                  // --color=always
	colorNever                   // --color=never
)

// timeField selects which timestamp to display and sort by.
type timeField int

const (
	timeMod    timeField = iota // default: modification time
	timeAccess                  // --time=atime/access/use
	timeChange                  // --time=ctime/status
)

// vdirConfig holds parsed command-line options.
// R1.6: accepts all flags that ls accepts.
type vdirConfig struct {
	output        outputMode
	filter        filterMode
	sorting       sortMode
	reverse       bool          // -r: reverse sort order
	humanReadable bool          // -h: human-readable sizes
	showBlocks    bool          // -s: show allocated block count
	showInode     bool          // -i: show inode number
	dirOnly       bool          // -d: list directories themselves
	recursive     bool          // -R: recurse into subdirectories
	indicator     indicatorMode // -F or -p
	color         colorMode     // --color
	colorActive   bool          // resolved: true if color output is active
	derefAll      bool          // -L: dereference all symlinks
	derefCmd      bool          // -H: dereference command-line symlinks
	numericIDs    bool          // -n: numeric UID/GID in long format
	showContext   bool          // -Z: show SELinux security context
	timeSelect    timeField     // --time
	timeStyle     string        // --time-style
	args          []string
}

// entry holds a directory entry with cached metadata.
type entry struct {
	name string
	path string
	info *sys.FileInfo // nil if stat failed
}

// binName is the program name used in error messages and help text.
const binName = "vdir"

func main() {
	// R2.4: install SIGPIPE handler.
	sys.InstallSIGPIPEHandler()
	sys.OnTerminalResize(func(_ int) {})
	// R1.4: C locale sort order.
	os.Setenv("LC_ALL", "C")
	cfg := parseArgs(os.Args[1:])
	applyColorMode(&cfg)
	os.Exit(run(cfg))
}

// applyColorMode resolves the --color flag into the effective color state.
func applyColorMode(cfg *vdirConfig) {
	switch cfg.color {
	case colorAlways:
		cfg.colorActive = true
	case colorNever:
		cfg.colorActive = false
	default:
		cfg.colorActive = sys.IsTerminal(os.Stdout.Fd())
	}
	format.SetColorEnabled(cfg.colorActive)
}

// parseArgs extracts flags and positional arguments.
// R1.5: defaults to current directory when no args given.
// R2.3: exits 2 for unrecognized options.
func parseArgs(args []string) vdirConfig {
	var cfg vdirConfig
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
			fmt.Printf("%s (go-unix-utils) dev\n", binName)
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "-") && arg != "-" && !strings.HasPrefix(arg, "--") {
			parseShortFlags(arg[1:], &cfg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			parseLongFlag(arg, &cfg)
			continue
		}
		cfg.args = append(cfg.args, arg)
	}
	if len(cfg.args) == 0 {
		cfg.args = []string{"."}
	}
	return cfg
}

// parseLongFlag processes a long flag, exiting 2 if unrecognized.
// R2.3: unrecognized --flags produce exit code 2.
func parseLongFlag(arg string, cfg *vdirConfig) {
	if handleLongFlag(arg, cfg) {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", binName, arg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", binName)
	os.Exit(2)
}

// parseShortFlags processes a cluster of short flags.
func parseShortFlags(flags string, cfg *vdirConfig) {
	for _, ch := range flags {
		applyShortFlag(ch, cfg)
	}
}

// applyShortFlag applies a single short flag character to the config.
// R1.6: accepts all ls short flags.
// R2.3: exits 2 for unrecognized options.
func applyShortFlag(ch rune, cfg *vdirConfig) {
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
	case 'm':
		cfg.output = modeComma
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
	case 'v':
		cfg.sorting = sortVersion
	case 'h':
		cfg.humanReadable = true
	case 's':
		cfg.showBlocks = true
	case 'i':
		cfg.showInode = true
	case 'd':
		cfg.dirOnly = true
	case 'n':
		cfg.output = modeLong
		cfg.numericIDs = true
	case 'F':
		cfg.indicator = indicatorClassify
	case 'p':
		cfg.indicator = indicatorSlash
	case 'L':
		cfg.derefAll = true
	case 'H':
		cfg.derefCmd = true
	case 'Z':
		cfg.showContext = true
	default:
		fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", binName, ch)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", binName)
		os.Exit(2)
	}
}

// handleLongFlag processes long-form flags. Returns true if recognized.
func handleLongFlag(arg string, cfg *vdirConfig) bool {
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
	case "--classify":
		cfg.indicator = indicatorClassify
	case "--color":
		cfg.color = colorAlways
	case "--dereference":
		cfg.derefAll = true
	case "--dereference-command-line":
		cfg.derefCmd = true
	case "--context":
		cfg.showContext = true
	case "--numeric-uid-gid":
		cfg.output = modeLong
		cfg.numericIDs = true
	default:
		return handleLongFlagValue(arg, cfg)
	}
	return true
}

// handleLongFlagValue processes long flags with =value syntax.
func handleLongFlagValue(arg string, cfg *vdirConfig) bool {
	key, val, ok := strings.Cut(arg, "=")
	if !ok {
		return false
	}
	switch key {
	case "--color":
		applyColorValue(val, cfg)
	case "--time":
		applyTimeValue(val, cfg)
	case "--time-style":
		cfg.timeStyle = val
	default:
		return false
	}
	return true
}

// applyColorValue sets the color mode from the --color=VALUE string.
func applyColorValue(val string, cfg *vdirConfig) {
	switch val {
	case "always", "yes", "force":
		cfg.color = colorAlways
	case "never", "no", "none":
		cfg.color = colorNever
	default:
		cfg.color = colorAuto
	}
}

// applyTimeValue sets the time selection from the --time=VALUE string.
func applyTimeValue(val string, cfg *vdirConfig) {
	switch val {
	case "atime", "access", "use":
		cfg.timeSelect = timeAccess
	case "ctime", "status":
		cfg.timeSelect = timeChange
	default:
		cfg.timeSelect = timeMod
	}
}

// run processes all arguments and returns the exit code.
// R2.1: returns 0 when all paths accessed successfully.
// R2.2: returns 1 when minor problems occur.
func run(cfg vdirConfig) int {
	if cfg.dirOnly {
		return runDirOnly(cfg)
	}
	return runNormal(cfg)
}

// runDirOnly handles -d: list entries themselves without descending.
// R2.2: command-line access failures set exit code 2 (matching gvdir).
func runDirOnly(cfg vdirConfig) int {
	sf := dirOnlyStatFunc(cfg)
	entries := make([]entry, 0, len(cfg.args))
	exitCode := 0
	for _, arg := range cfg.args {
		fi, err := sf(arg)
		if err != nil {
			reportError("cannot access", arg, err)
			exitCode = 2
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

// dirOnlyStatFunc returns the stat function for -d mode.
func dirOnlyStatFunc(cfg vdirConfig) func(string) (*sys.FileInfo, error) {
	if cfg.derefAll || cfg.derefCmd {
		return sys.Stat
	}
	return sys.Lstat
}

// runNormal processes arguments separating files and directories.
// R2.2: command-line access failures set exit code 2, continue processing.
func runNormal(cfg vdirConfig) int {
	exitCode := 0
	errCount := 0
	var fileEntries, dirEntries []entry
	for _, arg := range cfg.args {
		fi, err := sys.Stat(arg)
		if err != nil {
			reportError("cannot access", arg, err)
			exitCode = 2
			errCount++
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
	showHeader := len(fileEntries) > 0 || len(dirEntries)+errCount > 1
	exitCode |= listDirs(dirEntries, cfg, needBlank, showHeader)
	return exitCode
}

// listDirs lists multiple directory arguments in order.
func listDirs(dirs []entry, cfg vdirConfig, needBlank, showHeader bool) int {
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
func listDir(dir string, cfg vdirConfig) int {
	rawEntries, err := os.ReadDir(dir)
	if err != nil {
		reportError("cannot open directory", dir, err)
		return 1
	}
	entries := buildDirEntries(dir, rawEntries, cfg.filter)
	if needsStat(cfg) {
		statEntriesInPlace(entries, cfg)
	}
	sortEntries(entries, cfg)
	exitCode := displayDirEntries(entries, cfg)
	if cfg.recursive {
		exitCode |= recurseSubdirs(entries, cfg)
	}
	return exitCode
}

// buildDirEntries creates entry structs from directory entries with filtering.
// R1.4: hide dot-entries by default; -a shows all; -A shows all except . and ..
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

// needsStat returns true when entry metadata is required.
func needsStat(cfg vdirConfig) bool {
	return cfg.output == modeLong ||
		resolveOutputMode(cfg) == modeLong ||
		cfg.sorting == sortTime ||
		cfg.sorting == sortSize ||
		cfg.showBlocks ||
		cfg.showInode ||
		cfg.recursive ||
		cfg.indicator != indicatorNone ||
		cfg.colorActive
}

// statEntriesInPlace populates the info field for each entry.
func statEntriesInPlace(entries []entry, cfg vdirConfig) {
	sf := sys.Lstat
	if cfg.derefAll {
		sf = sys.Stat
	}
	for i := range entries {
		fi, err := sf(entries[i].path)
		if err == nil {
			entries[i].info = fi
		}
	}
}

// sortEntries sorts entries according to the configured sort mode.
func sortEntries(entries []entry, cfg vdirConfig) {
	if cfg.sorting == sortNone || len(entries) <= 1 {
		return
	}
	cmp := selectComparator(cfg)
	if cfg.reverse {
		orig := cmp
		cmp = func(a, b entry) bool { return orig(b, a) }
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return cmp(entries[i], entries[j])
	})
}

// selectComparator returns the less function for the given sort config.
func selectComparator(cfg vdirConfig) func(a, b entry) bool {
	switch cfg.sorting {
	case sortTime:
		tf := cfg.timeSelect
		return func(a, b entry) bool {
			return compareByTimeField(a, b, tf)
		}
	case sortSize:
		return compareBySize
	case sortVersion:
		return compareByVersion
	default:
		return compareByName
	}
}

// recurseSubdirs handles -R recursive listing for subdirectories.
func recurseSubdirs(entries []entry, cfg vdirConfig) int {
	exitCode := 0
	for _, e := range entries {
		if e.name == "." || e.name == ".." {
			continue
		}
		if !isRealDir(e, cfg) {
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

// isRealDir checks if an entry is a real directory (not a symlink to one).
func isRealDir(e entry, cfg vdirConfig) bool {
	if e.info == nil || !e.info.Mode.IsDir() {
		return false
	}
	if cfg.derefAll {
		lfi, err := sys.Lstat(e.path)
		if err != nil {
			return false
		}
		return lfi.Mode&os.ModeSymlink == 0
	}
	return true
}

// joinPath concatenates parent and child, avoiding double slashes.
func joinPath(parent, child string) string {
	if strings.HasSuffix(parent, "/") {
		return parent + child
	}
	return parent + "/" + child
}

// reportError prints a diagnostic to stderr matching GNU vdir format.
// R2.2: must print diagnostic for each inaccessible entry.
func reportError(action, path string, err error) {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "%s: %s '%s': %s\n", binName, action, path, msg)
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Printf(`Usage: %s [OPTION]... [FILE]...
List directory contents in long format with C-style escaping.
Equivalent to ls -l -b.

  -a, --all            do not ignore entries starting with .
  -A, --almost-all     do not list implied . and ..
  -C                   list entries by columns
      --color[=WHEN]   colorize output; WHEN is always, auto, or never
  -d                   list directories themselves, not their contents
  -F, --classify       append indicator (one of /=>@|) to entries
  -h, --human-readable print sizes in human-readable format
  -H                   follow symlinks on the command line
  -i                   print the index number of each file
  -l                   use a long listing format
  -L, --dereference    dereference symlinks, show target info
  -m                   fill width with a comma separated list of entries
  -n, --numeric-uid-gid  like -l, but list numeric user and group IDs
  -p                   append / indicator to directories
  -r, --reverse        reverse order while sorting
  -R, --recursive      list subdirectories recursively
  -s                   print the allocated size of each file
  -S                   sort by file size, largest first
  -t                   sort by time, newest first
      --time=WORD      select timestamp: atime, ctime (default mtime)
      --time-style=STYLE  time format: full-iso, long-iso, iso
  -U                   do not sort; list entries in directory order
  -v                   natural sort of (version) numbers within text
  -x                   list entries by lines instead of by columns
  -1                   list one file per line
      --help           display this help and exit
      --version        output version information and exit
`, binName)
}
