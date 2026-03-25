// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd109-dircolors R1.1, R1.2, R1.3, R1.4: dircolors shell
// output format with built-in default color database.

package main

import (
	"fmt"
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
// R1.3 (AC2): includes all standard file types.
var defaultTypeEntries = [][2]string{
	{"rs", "0"}, {"di", "01;34"}, {"ln", "01;36"}, {"mh", "00"},
	{"pi", "40;33"}, {"so", "01;35"}, {"do", "01;35"},
	{"bd", "40;33;01"}, {"cd", "40;33;01"}, {"or", "40;31;01"},
	{"mi", "00"}, {"su", "37;41"}, {"sg", "30;43"}, {"ca", "00"},
	{"tw", "30;42"}, {"ow", "34;42"}, {"st", "37;44"}, {"ex", "01;32"},
}

// defaultExtEntries contains the extension LS_COLORS pairs from the
// built-in database. Order matches the GNU dircolors compiled-in defaults.
// R1.4 (AC3): includes archives, images, audio, video, backups.
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

// buildLSColors constructs the colon-separated LS_COLORS value string
// from the default type and extension entries. Each pair is followed by
// a colon, including the last one, matching GNU dircolors output.
func buildLSColors() string {
	total := len(defaultTypeEntries) + len(defaultExtEntries)
	entries := make([][2]string, 0, total)
	entries = append(entries, defaultTypeEntries...)
	entries = append(entries, defaultExtEntries...)
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e[0])
		b.WriteByte('=')
		b.WriteString(e[1])
		b.WriteByte(':')
	}
	return b.String()
}

// termMatches checks if the given terminal name matches any default
// TERM glob pattern from the built-in database.
func termMatches(term string) bool {
	for _, p := range defaultTermPatterns {
		if matched, _ := filepath.Match(p, term); matched {
			return true
		}
	}
	return false
}

// colorsApply checks if colors should be applied based on the current
// terminal environment. The built-in database has "COLORTERM ?*" (any
// non-empty COLORTERM) and a list of TERM glob patterns.
func colorsApply() bool {
	if os.Getenv("COLORTERM") != "" {
		return true
	}
	term := os.Getenv("TERM")
	return term != "" && termMatches(term)
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
func handlePositional(cfg *config, arg string) error {
	if strings.HasPrefix(arg, "-") {
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
	lsColors := ""
	if colorsApply() {
		lsColors = buildLSColors()
	}
	outputForShell(shell, lsColors)
	return 0
}

// handlePrintDB handles the -p/--print-database option.
// R3.1: full implementation deferred to future task.
func handlePrintDB(cfg config) int {
	if cfg.filename != "" {
		fmt.Fprintf(os.Stderr,
			"%s: the options to output dircolors' internal database and\n"+
				"to select a shell syntax are mutually exclusive\n", progName)
		return 1
	}
	// TODO: R3.1 -- print full default database text (future task)
	return 0
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}
