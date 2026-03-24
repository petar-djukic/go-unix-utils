// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd008-ls R1.1-R1.14, R2.1-R2.15, R3.1-R3.15, R4.1-R4.9:
// directory listing with output modes (-1, -C, -m, -x), sorting flags
// (-t, -S, -r, -U, -v), filtering (-a, -A, -d), metadata display (-s, -i, -n),
// human-readable sizes (-h), color output (--color), symlink handling (-L, -H),
// type indicators (-F, -p), time selection (--time), time formatting
// (--time-style), recursive listing (-R), and exit code contracts.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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

// lsConfig holds parsed command-line options.
type lsConfig struct {
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

// TODO: -w / --width flag skipped — listed in prd008-ls non_goals as out of scope (E6).
// TODO: -T / --tabsize flag skipped — not specified in prd008-ls (E7).

func main() {
	sys.InstallSIGPIPEHandler()
	// R4.5: install SIGWINCH handler for terminal resize adaptation.
	// Terminal width is queried dynamically in termWidthOrDefault,
	// so the callback ensures the process responds to SIGWINCH signals.
	sys.OnTerminalResize(func(_ int) {})
	// R1.3: C locale sort order.
	os.Setenv("LC_ALL", "C")

	cfg := parseArgs(os.Args[1:])
	applyColorMode(&cfg)
	os.Exit(run(cfg))
}

// applyColorMode resolves the --color flag into the effective color state.
// R3.1: --color=auto enables color only when stdout is a TTY.
// R3.2: uses pkg/sys.IsTerminal for TTY detection.
func applyColorMode(cfg *lsConfig) {
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
// R4.3: exits 2 for unrecognized options.
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
// R4.3: unrecognized --flags produce exit code 2.
func parseLongFlag(arg string, cfg *lsConfig) {
	if handleLongFlag(arg, cfg) {
		return
	}
	fmt.Fprintf(os.Stderr, "ls: unrecognized option '%s'\n", arg)
	fmt.Fprintln(os.Stderr, "Try 'ls --help' for more information.")
	os.Exit(2)
}

// parseShortFlags processes a cluster of short flags (e.g., "-laRtSrFpLH").
func parseShortFlags(flags string, cfg *lsConfig) {
	for _, ch := range flags {
		applyShortFlag(ch, cfg)
	}
}

// applyShortFlag applies a single short flag character to the config.
// R4.3: unrecognized short flags produce exit code 2.
func applyShortFlag(ch rune, cfg *lsConfig) {
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
		// R2.9: version sort.
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
		// R2.14: -n implies long format with numeric UID/GID.
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
	default:
		// R4.3: exit 2 for unrecognized options.
		fmt.Fprintf(os.Stderr, "ls: invalid option -- '%c'\n", ch)
		fmt.Fprintln(os.Stderr, "Try 'ls --help' for more information.")
		os.Exit(2)
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
	case "--classify":
		cfg.indicator = indicatorClassify
	case "--color":
		cfg.color = colorAlways
	case "--dereference":
		cfg.derefAll = true
	case "--dereference-command-line":
		cfg.derefCmd = true
	case "--numeric-uid-gid":
		// R2.14: long-form of -n.
		cfg.output = modeLong
		cfg.numericIDs = true
	default:
		return handleLongFlagValue(arg, cfg)
	}
	return true
}

// handleLongFlagValue processes long flags with =value syntax.
func handleLongFlagValue(arg string, cfg *lsConfig) bool {
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
// R3.1: supports always, auto, never (and GNU aliases).
func applyColorValue(val string, cfg *lsConfig) {
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
// R3.5: selects which timestamp to display: mtime (default), atime, ctime.
func applyTimeValue(val string, cfg *lsConfig) {
	switch val {
	case "atime", "access", "use":
		cfg.timeSelect = timeAccess
	case "ctime", "status":
		cfg.timeSelect = timeChange
	default:
		cfg.timeSelect = timeMod
	}
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: ls [OPTION]... [FILE]...
List information about the FILEs (the current directory by default).

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
`)
}

// run processes all arguments and returns the exit code.
// R4.1: returns 0 when all paths accessed successfully.
// R4.2: returns 1 when minor problems occur (inaccessible entries).
func run(cfg lsConfig) int {
	if cfg.dirOnly {
		return runDirOnly(cfg)
	}
	return runNormal(cfg)
}

// runDirOnly handles -d: list entries themselves without descending.
// R2.3: directories are listed as entries, not their contents.
// R3.3: -L dereferences all symlinks. R3.4: -H dereferences command-line args.
func runDirOnly(cfg lsConfig) int {
	sf := dirOnlyStatFunc(cfg)
	entries := make([]entry, 0, len(cfg.args))
	exitCode := 0
	for _, arg := range cfg.args {
		fi, err := sf(arg)
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

// dirOnlyStatFunc returns the stat function for -d mode command-line args.
// R3.3: -L follows all symlinks. R3.4: -H follows command-line symlinks.
func dirOnlyStatFunc(cfg lsConfig) func(string) (*sys.FileInfo, error) {
	if cfg.derefAll || cfg.derefCmd {
		return sys.Stat
	}
	return sys.Lstat
}

// runNormal processes arguments separating files and directories.
// R4.2: reports errors for inaccessible entries and continues.
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
		statEntriesInPlace(entries, cfg)
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
// R3.14: filter flags apply to each subdirectory in -R mode.
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
		cfg.recursive ||
		cfg.indicator != indicatorNone ||
		cfg.colorActive
}

// statEntriesInPlace populates the info field for each entry.
// R3.3: -L uses Stat to follow symlinks; default uses Lstat.
func statEntriesInPlace(entries []entry, cfg lsConfig) {
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
// R2.5: -t sorts by mtime newest first.
// R2.6: -S sorts by size largest first.
// R2.7: -r reverses the sort order.
// R2.8: -U disables sorting.
// R3.15: -R recurses subdirectories in the same sort order.
func sortEntries(entries []entry, cfg lsConfig) {
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
func selectComparator(cfg lsConfig) func(a, b entry) bool {
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

// compareByName sorts alphabetically by name in C locale order.
func compareByName(a, b entry) bool {
	return a.name < b.name
}

// compareByTimeField sorts by the selected time field, newest first.
// R2.5: ties broken by name in C locale order.
func compareByTimeField(a, b entry, tf timeField) bool {
	if a.info == nil || b.info == nil {
		return compareByName(a, b)
	}
	at := entryTime(a.info, tf)
	bt := entryTime(b.info, tf)
	if !at.Equal(bt) {
		return at.After(bt)
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

// compareByVersion sorts using version sort semantics.
// R2.9: digit runs are compared numerically (strverscmp behavior).
func compareByVersion(a, b entry) bool {
	return versionLess(a.name, b.name)
}

// entryTime returns the selected timestamp from a FileInfo.
// R3.5: --time selects atime, ctime, or mtime (default).
func entryTime(info *sys.FileInfo, tf timeField) time.Time {
	switch tf {
	case timeAccess:
		return info.AccessTime
	case timeChange:
		return info.ChangeTime
	default:
		return info.ModTime
	}
}

// typeIndicator returns the suffix character for -F or -p.
// R3.8: -F appends / for dirs, * for executables, @ for symlinks,
// | for FIFOs, = for sockets. -p appends / for dirs only.
func typeIndicator(info *sys.FileInfo, im indicatorMode) string {
	if im == indicatorNone || info == nil {
		return ""
	}
	if info.Mode.IsDir() {
		return "/"
	}
	if im == indicatorSlash {
		return ""
	}
	return classifyIndicator(info.Mode)
}

// classifyIndicator returns the -F indicator for non-directory entries.
// R3.9: executable is any execute bit set (0o111).
func classifyIndicator(mode os.FileMode) string {
	switch {
	case mode&os.ModeSymlink != 0:
		return "@"
	case mode&os.ModeNamedPipe != 0:
		return "|"
	case mode&os.ModeSocket != 0:
		return "="
	case mode.IsRegular() && mode&0o111 != 0:
		return "*"
	default:
		return ""
	}
}

// recurseSubdirs handles -R recursive listing for subdirectories.
// R3.13: does not follow symbolic links to directories.
// R3.15: recurses in the same sort order as the directory listing.
func recurseSubdirs(_ string, entries []entry, cfg lsConfig) int {
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
// R3.13: -R must not follow symbolic links to directories.
func isRealDir(e entry, cfg lsConfig) bool {
	if e.info == nil || !e.info.Mode.IsDir() {
		return false
	}
	// When -L is active, Stat makes symlinks appear as dirs.
	// Use Lstat to verify the entry is not actually a symlink.
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

// reportError prints a diagnostic to stderr matching GNU ls format.
// R4.2: must print diagnostic for each inaccessible entry.
func reportError(action, path string, err error) {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "ls: %s '%s': %s\n", action, path, msg)
}

// versionLess compares two strings using version sort semantics.
// Digit runs are compared numerically so "file2" < "file10".
func versionLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		if isDigit(ca) && isDigit(cb) {
			cmp := compareDigitRuns(a, b, &ai, &bi)
			if cmp != 0 {
				return cmp < 0
			}
			continue
		}
		if ca != cb {
			return ca < cb
		}
		ai++
		bi++
	}
	return len(a) < len(b)
}

// compareDigitRuns extracts and compares digit runs numerically.
// Advances ai and bi past the digit sequences.
func compareDigitRuns(a, b string, ai, bi *int) int {
	na := extractDigitRun(a, ai)
	nb := extractDigitRun(b, bi)
	ta := trimLeadingZeros(na)
	tb := trimLeadingZeros(nb)
	if len(ta) != len(tb) {
		return len(ta) - len(tb)
	}
	return strings.Compare(ta, tb)
}

// extractDigitRun returns the digit substring starting at *pos
// and advances *pos past it.
func extractDigitRun(s string, pos *int) string {
	start := *pos
	for *pos < len(s) && isDigit(s[*pos]) {
		*pos++
	}
	return s[start:*pos]
}

// trimLeadingZeros removes leading zeros, preserving at least one digit.
func trimLeadingZeros(s string) string {
	i := 0
	for i < len(s)-1 && s[i] == '0' {
		i++
	}
	return s[i:]
}

// isDigit returns true if c is an ASCII digit.
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
