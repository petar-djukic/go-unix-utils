// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd109-dircolors R1.1, R1.2, R1.3, R1.4: dircolors shell
// output format with built-in default color database.
// Implements prd109-dircolors R2.1-R2.5: database parsing, TERM matching,
// shell output modes, -p/--print-database, and stdin via "-".
// Implements prd109-dircolors R3.1-R3.3: TERM glob patterns, comment/blank
// line handling, all file type keywords and extension entries.

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "dircolors"

// shellMode represents the output shell format.
type shellMode int

const (
	shellAuto    shellMode = iota
	shellBourne            // R1.1: Bourne shell output
	shellCShell            // R1.2: C shell output
)

// defaultTermPatterns contains the TERM glob patterns from the GNU
// dircolors built-in database.
var defaultTermPatterns = []string{
	"Eterm", "ansi", "*color*", "con[0-9]*x[0-9]*", "cons25",
	"console", "cygwin", "*direct*", "dtterm", "gnome", "hurd",
	"jfbterm", "konsole", "kterm", "linux", "linux-c", "mlterm",
	"putty", "rxvt*", "screen*", "st", "terminator", "tmux*",
	"vt100", "vt220", "xterm*",
}

// defaultTypeEntries contains the file type LS_COLORS pairs from the
// built-in database. Order matches the GNU dircolors compiled-in defaults.
var defaultTypeEntries = [][2]string{
	{"rs", "0"}, {"di", "01;34"}, {"ln", "01;36"}, {"mh", "00"},
	{"pi", "40;33"}, {"so", "01;35"}, {"do", "01;35"},
	{"bd", "40;33;01"}, {"cd", "40;33;01"}, {"or", "40;31;01"},
	{"mi", "00"}, {"su", "37;41"}, {"sg", "30;43"}, {"ca", "00"},
	{"tw", "30;42"}, {"ow", "34;42"}, {"st", "37;44"}, {"ex", "01;32"},
}

// defaultExtEntries contains the extension LS_COLORS pairs from the
// built-in database. Order matches the GNU dircolors compiled-in defaults.
var defaultExtEntries = [][2]string{
	// Archives (bright red)
	{"*.7z", "01;31"}, {"*.ace", "01;31"}, {"*.alz", "01;31"},
	{"*.apk", "01;31"}, {"*.arc", "01;31"}, {"*.arj", "01;31"},
	{"*.bz", "01;31"}, {"*.bz2", "01;31"}, {"*.cab", "01;31"},
	{"*.cpio", "01;31"}, {"*.crate", "01;31"}, {"*.deb", "01;31"},
	{"*.drpm", "01;31"}, {"*.dwm", "01;31"}, {"*.dz", "01;31"},
	{"*.ear", "01;31"}, {"*.egg", "01;31"}, {"*.esd", "01;31"},
	{"*.gz", "01;31"}, {"*.jar", "01;31"}, {"*.lha", "01;31"},
	{"*.lrz", "01;31"}, {"*.lz", "01;31"}, {"*.lz4", "01;31"},
	{"*.lzh", "01;31"}, {"*.lzma", "01;31"}, {"*.lzo", "01;31"},
	{"*.pyz", "01;31"}, {"*.rar", "01;31"}, {"*.rpm", "01;31"},
	{"*.rz", "01;31"}, {"*.sar", "01;31"}, {"*.swm", "01;31"},
	{"*.t7z", "01;31"}, {"*.tar", "01;31"}, {"*.taz", "01;31"},
	{"*.tbz", "01;31"}, {"*.tbz2", "01;31"}, {"*.tgz", "01;31"},
	{"*.tlz", "01;31"}, {"*.txz", "01;31"}, {"*.tz", "01;31"},
	{"*.tzo", "01;31"}, {"*.tzst", "01;31"}, {"*.udeb", "01;31"},
	{"*.war", "01;31"}, {"*.whl", "01;31"}, {"*.wim", "01;31"},
	{"*.xz", "01;31"}, {"*.z", "01;31"}, {"*.zip", "01;31"},
	{"*.zoo", "01;31"}, {"*.zst", "01;31"},
	// Images and video (bright magenta)
	{"*.avif", "01;35"}, {"*.jpg", "01;35"}, {"*.jpeg", "01;35"},
	{"*.jxl", "01;35"}, {"*.mjpg", "01;35"}, {"*.mjpeg", "01;35"},
	{"*.gif", "01;35"}, {"*.bmp", "01;35"}, {"*.pbm", "01;35"},
	{"*.pgm", "01;35"}, {"*.ppm", "01;35"}, {"*.tga", "01;35"},
	{"*.xbm", "01;35"}, {"*.xpm", "01;35"}, {"*.tif", "01;35"},
	{"*.tiff", "01;35"}, {"*.png", "01;35"}, {"*.svg", "01;35"},
	{"*.svgz", "01;35"}, {"*.mng", "01;35"}, {"*.pcx", "01;35"},
	{"*.mov", "01;35"}, {"*.mpg", "01;35"}, {"*.mpeg", "01;35"},
	{"*.m2v", "01;35"}, {"*.mkv", "01;35"}, {"*.webm", "01;35"},
	{"*.webp", "01;35"}, {"*.ogm", "01;35"}, {"*.mp4", "01;35"},
	{"*.m4v", "01;35"}, {"*.mp4v", "01;35"}, {"*.vob", "01;35"},
	{"*.qt", "01;35"}, {"*.nuv", "01;35"}, {"*.wmv", "01;35"},
	{"*.asf", "01;35"}, {"*.rm", "01;35"}, {"*.rmvb", "01;35"},
	{"*.flc", "01;35"}, {"*.avi", "01;35"}, {"*.fli", "01;35"},
	{"*.flv", "01;35"}, {"*.gl", "01;35"}, {"*.dl", "01;35"},
	{"*.xcf", "01;35"}, {"*.xwd", "01;35"}, {"*.yuv", "01;35"},
	{"*.cgm", "01;35"}, {"*.emf", "01;35"},
	{"*.ogv", "01;35"}, {"*.ogx", "01;35"},
	// Audio (cyan)
	{"*.aac", "00;36"}, {"*.au", "00;36"}, {"*.flac", "00;36"},
	{"*.m4a", "00;36"}, {"*.mid", "00;36"}, {"*.midi", "00;36"},
	{"*.mka", "00;36"}, {"*.mp3", "00;36"}, {"*.mpc", "00;36"},
	{"*.ogg", "00;36"}, {"*.ra", "00;36"}, {"*.wav", "00;36"},
	{"*.oga", "00;36"}, {"*.opus", "00;36"}, {"*.spx", "00;36"},
	{"*.xspf", "00;36"},
	// Backup and temporary files (dark gray)
	{"*~", "00;90"}, {"*#", "00;90"},
	{"*.bak", "00;90"}, {"*.crdownload", "00;90"},
	{"*.dpkg-dist", "00;90"}, {"*.dpkg-new", "00;90"},
	{"*.dpkg-old", "00;90"}, {"*.dpkg-tmp", "00;90"},
	{"*.old", "00;90"}, {"*.orig", "00;90"}, {"*.part", "00;90"},
	{"*.rej", "00;90"}, {"*.rpmnew", "00;90"}, {"*.rpmorig", "00;90"},
	{"*.rpmsave", "00;90"}, {"*.swp", "00;90"}, {"*.tmp", "00;90"},
	{"*.ucf-dist", "00;90"}, {"*.ucf-new", "00;90"}, {"*.ucf-old", "00;90"},
}

// keywordMap maps dircolors file type keywords to LS_COLORS codes.
// R2.3, R3.3: all standard file type keywords.
var keywordMap = map[string]string{
	"NORMAL": "no", "FILE": "fi", "RESET": "rs",
	"DIR": "di", "LINK": "ln", "MULTIHARDLINK": "mh",
	"FIFO": "pi", "SOCK": "so", "DOOR": "do",
	"BLK": "bd", "CHR": "cd", "ORPHAN": "or",
	"MISSING": "mi", "SETUID": "su", "SETGID": "sg",
	"CAPABILITY": "ca", "STICKY_OTHER_WRITABLE": "tw",
	"OTHER_WRITABLE": "ow", "STICKY": "st", "EXEC": "ex",
	"LEFT": "lc", "RIGHT": "rc", "END": "ec",
}

// ignoredKeywords are recognized by GNU dircolors but silently skipped.
var ignoredKeywords = map[string]bool{
	"COLOR": true, "OPTIONS": true, "EIGHTBIT": true,
}

// colorDatabase holds entries parsed from a dircolors configuration file.
// R3.1: includes TERM patterns for terminal type matching.
type colorDatabase struct {
	termPatterns      []string
	colortermPatterns []string
	typeEntries       [][2]string
	extEntries        [][2]string
}

// buildLSColors constructs the colon-separated LS_COLORS value string
// from the default type and extension entries.
func buildLSColors() string {
	return buildLSColorsFromEntries(defaultTypeEntries, defaultExtEntries)
}

// buildLSColorsFromEntries constructs the colon-separated LS_COLORS value
// from type and extension entry slices. Each pair is followed by a colon,
// including the last one, matching GNU dircolors output.
func buildLSColorsFromEntries(types, exts [][2]string) string {
	total := len(types) + len(exts)
	entries := make([][2]string, 0, total)
	entries = append(entries, types...)
	entries = append(entries, exts...)
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e[0])
		b.WriteByte('=')
		b.WriteString(e[1])
		b.WriteByte(':')
	}
	return b.String()
}

// buildLSColorsFromDB constructs the LS_COLORS value from a parsed database.
func buildLSColorsFromDB(db *colorDatabase) string {
	return buildLSColorsFromEntries(db.typeEntries, db.extEntries)
}

// matchesAnyPattern checks if value matches any of the glob patterns.
// R3.1: uses filepath.Match for TERM glob matching.
func matchesAnyPattern(value string, patterns []string) bool {
	if value == "" {
		return false
	}
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, value); matched {
			return true
		}
	}
	return false
}

// termMatches checks if the given terminal name matches any default
// TERM glob pattern from the built-in database.
func termMatches(term string) bool {
	return matchesAnyPattern(term, defaultTermPatterns)
}

// colorsApply checks if colors should be applied based on the current
// terminal environment using the built-in database patterns.
func colorsApply() bool {
	if os.Getenv("COLORTERM") != "" {
		return true
	}
	term := os.Getenv("TERM")
	return term != "" && termMatches(term)
}

// dbColorsApply checks if colors should apply based on a parsed database's
// TERM and COLORTERM patterns. R2.2, R3.1: when no filter lines are
// present, colors apply to all terminals.
func dbColorsApply(db *colorDatabase) bool {
	hasFilters := len(db.termPatterns) > 0 || len(db.colortermPatterns) > 0
	if !hasFilters {
		return true
	}
	if matchesAnyPattern(os.Getenv("COLORTERM"), db.colortermPatterns) {
		return true
	}
	return matchesAnyPattern(os.Getenv("TERM"), db.termPatterns)
}

// detectShell determines shell format from the SHELL environment variable.
// R1.3: if SHELL ends with "csh", use C shell format; otherwise Bourne.
func detectShell() shellMode {
	shell := os.Getenv("SHELL")
	if strings.HasSuffix(shell, "csh") {
		return shellCShell
	}
	return shellBourne
}

// outputBourne prints LS_COLORS in Bourne shell format.
// R1.1: LS_COLORS='<value>';\nexport LS_COLORS
func outputBourne(value string) {
	fmt.Printf("LS_COLORS='%s';\nexport LS_COLORS\n", value)
}

// outputCShell prints LS_COLORS in C shell format.
// R1.2: setenv LS_COLORS '<value>'
func outputCShell(value string) {
	fmt.Printf("setenv LS_COLORS '%s'\n", value)
}

// config holds parsed command-line options.
type config struct {
	shell    shellMode
	printDB  bool
	filename string
}

// parseArgs parses command-line arguments into a config.
// R1.4: -b and -c are mutually exclusive; last one wins.
func parseArgs(args []string) (config, error) {
	var cfg config
	for _, arg := range args {
		switch arg {
		case "-b", "--sh", "--bourne-shell":
			cfg.shell = shellBourne
		case "-c", "--csh", "--c-shell":
			cfg.shell = shellCShell
		case "-p", "--print-database":
			cfg.printDB = true
		case "--help":
			printUsage()
			os.Exit(0)
		case "--version":
			fmt.Printf("%s (go-unix-utils)\n", progName)
			os.Exit(0)
		default:
			if err := handlePositional(&cfg, arg); err != nil {
				return cfg, err
			}
		}
	}
	return cfg, nil
}

// handlePositional processes a non-flag argument as a filename.
// R2.5: "-" is accepted as a valid filename meaning stdin.
func handlePositional(cfg *config, arg string) error {
	if arg != "-" && strings.HasPrefix(arg, "-") {
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	if cfg.filename != "" {
		return fmt.Errorf("extra operand '%s'", arg)
	}
	cfg.filename = arg
	return nil
}

// printUsage prints a brief usage message.
func printUsage() {
	fmt.Fprintf(os.Stdout, "Usage: %s [OPTION]... [FILE]\n", progName)
	fmt.Fprintln(os.Stdout, "Output commands to set LS_COLORS.")
}

// outputForShell dispatches to the appropriate shell output function.
func outputForShell(shell shellMode, value string) {
	switch shell {
	case shellCShell:
		outputCShell(value)
	default:
		outputBourne(value)
	}
}

// openDatabase opens the database source.
// R2.5: when filename is "-", reads from stdin.
func openDatabase(filename string) (io.ReadCloser, error) {
	if filename == "-" {
		return os.Stdin, nil
	}
	return os.Open(filename)
}

// parseDatabase reads and parses a dircolors configuration file.
// R3.1: collects TERM entries with glob patterns.
// R3.2: skips comments and blank lines.
// R3.3: supports all file type keywords and extension entries.
func parseDatabase(r io.Reader, filename string) (*colorDatabase, error) {
	db := &colorDatabase{}
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if err := parseLine(db, scanner.Text(), filename, lineNum); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	return db, nil
}

// stripComment removes an inline comment (everything from # onward).
func stripComment(line string) string {
	before, _, found := strings.Cut(line, "#")
	if found {
		return before
	}
	return line
}

// parseLine parses a single configuration file line into the database.
// R3.2: blank lines and comment-only lines are silently skipped.
func parseLine(db *colorDatabase, raw, filename string, lineNum int) error {
	line := strings.TrimSpace(stripComment(raw))
	if line == "" {
		return nil
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return fmt.Errorf("%s:%d: invalid line; missing second token",
			filename, lineNum)
	}
	return classifyEntry(db, fields[0], fields[1], filename, lineNum)
}

// classifyEntry routes a keyword-value pair to the correct database slot.
// R3.3: handles TERM, COLORTERM, ignored keywords, file type keywords,
// and extension entries.
func classifyEntry(db *colorDatabase, kw, val, fn string, ln int) error {
	switch {
	case kw == "TERM":
		db.termPatterns = append(db.termPatterns, val)
	case kw == "COLORTERM":
		db.colortermPatterns = append(db.colortermPatterns, val)
	case ignoredKeywords[kw]:
		// Silently ignored per GNU dircolors behavior
	case keywordMap[kw] != "":
		db.typeEntries = append(db.typeEntries, [2]string{keywordMap[kw], val})
	case strings.HasPrefix(kw, ".") || strings.HasPrefix(kw, "*"):
		db.extEntries = append(db.extEntries, [2]string{ensureStarPrefix(kw), val})
	default:
		return fmt.Errorf("%s:%d: unrecognized keyword %s", fn, ln, kw)
	}
	return nil
}

// ensureStarPrefix ensures extension entries have the "*" prefix for LS_COLORS.
// ".tar" becomes "*.tar"; "*~" stays "*~".
func ensureStarPrefix(ext string) string {
	if strings.HasPrefix(ext, ".") {
		return "*" + ext
	}
	return ext
}

// handleFileDB reads a custom database file and outputs LS_COLORS.
// R2.4: file argument replaces built-in defaults.
// R2.5: "-" reads from stdin.
func handleFileDB(filename string, shell shellMode) int {
	r, err := openDatabase(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	if filename != "-" {
		defer r.Close() // best-effort file close
	}
	db, err := parseDatabase(r, filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	lsColors := ""
	if dbColorsApply(db) {
		lsColors = buildLSColorsFromDB(db)
	}
	outputForShell(shell, lsColors)
	return 0
}

// run contains the main logic and returns the exit code.
func run() int {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	if cfg.printDB {
		return handlePrintDB(cfg)
	}
	shell := cfg.shell
	if shell == shellAuto {
		shell = detectShell()
	}
	if cfg.filename != "" {
		return handleFileDB(cfg.filename, shell)
	}
	lsColors := ""
	if colorsApply() {
		lsColors = buildLSColors()
	}
	outputForShell(shell, lsColors)
	return 0
}

// handlePrintDB handles the -p/--print-database option.
// R3.1: prints the built-in default color database.
// R3.2: -p is incompatible with a filename argument.
func handlePrintDB(cfg config) int {
	if cfg.filename != "" {
		fmt.Fprintf(os.Stderr,
			"%s: the options to output dircolors' internal database and\n"+
				"to select a shell syntax are mutually exclusive\n", progName)
		return 1
	}
	fmt.Print(defaultDBText)
	return 0
}

// defaultDBText contains the full built-in default color database text
// matching the output of gdircolors --print-database.
const defaultDBText = `# Configuration file for dircolors, a utility to help you set the
# LS_COLORS environment variable used by GNU ls with the --color option.
# Copyright (C) 1996-2026 Free Software Foundation, Inc.
# Copying and distribution of this file, with or without modification,
# are permitted provided the copyright notice and this notice are preserved.
#
# The keywords COLOR, OPTIONS, and EIGHTBIT (honored by the
# slackware version of dircolors) are recognized but ignored.
# Global config options can be specified before TERM or COLORTERM entries
# ===================================================================
# Terminal filters
# ===================================================================
# Below are TERM or COLORTERM entries, which can be glob patterns, which
# restrict following config to systems with matching environment variables.
COLORTERM ?*
TERM Eterm
TERM ansi
TERM *color*
TERM con[0-9]*x[0-9]*
TERM cons25
TERM console
TERM cygwin
TERM *direct*
TERM dtterm
TERM gnome
TERM hurd
TERM jfbterm
TERM konsole
TERM kterm
TERM linux
TERM linux-c
TERM mlterm
TERM putty
TERM rxvt*
TERM screen*
TERM st
TERM terminator
TERM tmux*
TERM vt100
TERM vt220
TERM xterm*
# ===================================================================
# Basic file attributes
# ===================================================================
# Below are the color init strings for the basic file types.
# One can use codes for 256 or more colors supported by modern terminals.
# The default color codes use the capabilities of an 8 color terminal
# with some additional attributes as per the following codes:
# Attribute codes:
# 00=none 01=bold 04=underscore 05=blink 07=reverse 08=concealed
# Text color codes:
# 30=black 31=red 32=green 33=yellow 34=blue 35=magenta 36=cyan 37=white
# Background color codes:
# 40=black 41=red 42=green 43=yellow 44=blue 45=magenta 46=cyan 47=white
#NORMAL 00 # no color code at all
#FILE 00 # regular file: use no color at all
RESET 0 # reset to "normal" color
DIR 01;34 # directory
LINK 01;36 # symbolic link. (If you set this to 'target' instead of a
 # numerical value, the color is as for the file pointed to.)
MULTIHARDLINK 00 # regular file with more than one link
FIFO 40;33 # pipe
SOCK 01;35 # socket
DOOR 01;35 # door
BLK 40;33;01 # block device driver
CHR 40;33;01 # character device driver
ORPHAN 40;31;01 # symlink to nonexistent file, or non-stat'able file ...
MISSING 00 # ... and the files they point to
SETUID 37;41 # regular file that is setuid (u+s)
SETGID 30;43 # regular file that is setgid (g+s)
CAPABILITY 00 # regular file with capability (very expensive to lookup)
STICKY_OTHER_WRITABLE 30;42 # dir that is sticky and other-writable (+t,o+w)
OTHER_WRITABLE 34;42 # dir that is other-writable (o+w) and not sticky
STICKY 37;44 # dir with the sticky bit set (+t) and not other-writable
# This is for regular files with execute permission:
EXEC 01;32
# ===================================================================
# File extension attributes
# ===================================================================
# List any file extensions like '.gz' or '.tar' that you would like ls
# to color below. Put the suffix, a space, and the color init string.
# (and any comments you want to add after a '#').
# Suffixes are matched case insensitively, but if you define different
# init strings for separate cases, those will be honored.
#
# If you use DOS-style suffixes, you may want to uncomment the following:
#.cmd 01;32 # executables (bright green)
#.exe 01;32
#.com 01;32
#.btm 01;32
#.bat 01;32
# Or if you want to color scripts even if they do not have the
# executable bit actually set.
#.sh 01;32
#.csh 01;32
# archives or compressed (bright red)
.7z 01;31
.ace 01;31
.alz 01;31
.apk 01;31
.arc 01;31
.arj 01;31
.bz 01;31
.bz2 01;31
.cab 01;31
.cpio 01;31
.crate 01;31
.deb 01;31
.drpm 01;31
.dwm 01;31
.dz 01;31
.ear 01;31
.egg 01;31
.esd 01;31
.gz 01;31
.jar 01;31
.lha 01;31
.lrz 01;31
.lz 01;31
.lz4 01;31
.lzh 01;31
.lzma 01;31
.lzo 01;31
.pyz 01;31
.rar 01;31
.rpm 01;31
.rz 01;31
.sar 01;31
.swm 01;31
.t7z 01;31
.tar 01;31
.taz 01;31
.tbz 01;31
.tbz2 01;31
.tgz 01;31
.tlz 01;31
.txz 01;31
.tz 01;31
.tzo 01;31
.tzst 01;31
.udeb 01;31
.war 01;31
.whl 01;31
.wim 01;31
.xz 01;31
.z 01;31
.zip 01;31
.zoo 01;31
.zst 01;31
# image formats
.avif 01;35
.jpg 01;35
.jpeg 01;35
.jxl 01;35
.mjpg 01;35
.mjpeg 01;35
.gif 01;35
.bmp 01;35
.pbm 01;35
.pgm 01;35
.ppm 01;35
.tga 01;35
.xbm 01;35
.xpm 01;35
.tif 01;35
.tiff 01;35
.png 01;35
.svg 01;35
.svgz 01;35
.mng 01;35
.pcx 01;35
.mov 01;35
.mpg 01;35
.mpeg 01;35
.m2v 01;35
.mkv 01;35
.webm 01;35
.webp 01;35
.ogm 01;35
.mp4 01;35
.m4v 01;35
.mp4v 01;35
.vob 01;35
.qt 01;35
.nuv 01;35
.wmv 01;35
.asf 01;35
.rm 01;35
.rmvb 01;35
.flc 01;35
.avi 01;35
.fli 01;35
.flv 01;35
.gl 01;35
.dl 01;35
.xcf 01;35
.xwd 01;35
.yuv 01;35
.cgm 01;35
.emf 01;35
# https://wiki.xiph.org/MIME_Types_and_File_Extensions
.ogv 01;35
.ogx 01;35
# audio formats
.aac 00;36
.au 00;36
.flac 00;36
.m4a 00;36
.mid 00;36
.midi 00;36
.mka 00;36
.mp3 00;36
.mpc 00;36
.ogg 00;36
.ra 00;36
.wav 00;36
# https://wiki.xiph.org/MIME_Types_and_File_Extensions
.oga 00;36
.opus 00;36
.spx 00;36
.xspf 00;36
# backup files
*~ 00;90
*# 00;90
.bak 00;90
.crdownload 00;90
.dpkg-dist 00;90
.dpkg-new 00;90
.dpkg-old 00;90
.dpkg-tmp 00;90
.old 00;90
.orig 00;90
.part 00;90
.rej 00;90
.rpmnew 00;90
.rpmorig 00;90
.rpmsave 00;90
.swp 00;90
.tmp 00;90
.ucf-dist 00;90
.ucf-new 00;90
.ucf-old 00;90
#
# Subsequent TERM or COLORTERM entries, can be used to add / override
# config specific to those matching environment variables.
`

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}
