// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/dircolors implements GNU dircolors — outputs shell commands to set LS_COLORS.
// Implements srd109 R1.1–R1.4: shell output format with built-in default database.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "dircolors"

// shellFormat selects the output format.
type shellFormat int

const (
	shellBourne shellFormat = iota
	shellCsh
)

func main() {
	sys.InstallSIGPIPEHandler()

	format, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}

	value := buildLSColors()
	printShellOutput(format, value)
}

// parseArgs processes flags and returns the selected shell format.
// R1.4: -b and -c are mutually exclusive; last one wins.
func parseArgs(args []string) (shellFormat, error) {
	explicit := false
	format := shellBourne

	for _, arg := range args {
		switch arg {
		case "-b", "--sh", "--bourne-shell":
			format = shellBourne
			explicit = true
		case "-c", "--csh", "--c-shell":
			format = shellCsh
			explicit = true
		case "--help":
			printUsage()
			os.Exit(0)
		case "--version":
			fmt.Println(programName)
			os.Exit(0)
		default:
			return 0, fmt.Errorf("unrecognized option: %s", arg)
		}
	}

	// R1.3: auto-detect from SHELL env when no explicit flag given.
	if !explicit {
		format = detectShell()
	}
	return format, nil
}

// detectShell checks SHELL env var; if it ends with "csh", use C shell format.
func detectShell() shellFormat {
	shell := os.Getenv("SHELL")
	base := shell
	if idx := strings.LastIndex(shell, "/"); idx >= 0 {
		base = shell[idx+1:]
	}
	if strings.HasSuffix(base, "csh") {
		return shellCsh
	}
	return shellBourne
}

// printShellOutput writes the LS_COLORS assignment in the selected format.
func printShellOutput(format shellFormat, value string) {
	switch format {
	case shellCsh:
		// R1.2: setenv LS_COLORS '<value>'
		fmt.Printf("setenv LS_COLORS '%s'\n", value)
	default:
		// R1.1: LS_COLORS='<value>';\nexport LS_COLORS
		fmt.Printf("LS_COLORS='%s';\nexport LS_COLORS\n", value)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [OPTION]...\nOutput commands to set LS_COLORS.\n"+
			"\n  -b, --sh, --bourne-shell    output Bourne shell code\n"+
			"  -c, --csh, --c-shell        output C shell code\n"+
			"      --help                  display this help\n"+
			"      --version               output version information\n",
		programName)
}

// buildLSColors constructs the colon-separated LS_COLORS value from the
// built-in default database. The order matches GNU dircolors compiled defaults.
func buildLSColors() string {
	entries := defaultDatabase()
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = e.key + "=" + e.value
	}
	return strings.Join(parts, ":") + ":"
}

// dbEntry is a single key=value pair in the color database.
type dbEntry struct {
	key   string
	value string
}

// defaultDatabase returns the built-in GNU dircolors default color database.
// D2: stored as a Go slice literal matching gdircolors compiled-in defaults.
func defaultDatabase() []dbEntry {
	return append(defaultFileTypes(), defaultExtensions()...)
}

// defaultFileTypes returns file-type entries matching GNU dircolors defaults.
func defaultFileTypes() []dbEntry {
	return []dbEntry{
		{"rs", "0"},
		{"di", "01;34"},
		{"ln", "01;36"},
		{"mh", "00"},
		{"pi", "40;33"},
		{"so", "01;35"},
		{"do", "01;35"},
		{"bd", "40;33;01"},
		{"cd", "40;33;01"},
		{"or", "40;31;01"},
		{"mi", "00"},
		{"su", "37;41"},
		{"sg", "30;43"},
		{"ca", "00"},
		{"tw", "30;42"},
		{"ow", "34;42"},
		{"st", "37;44"},
		{"ex", "01;32"},
	}
}

// defaultExtensions returns extension entries matching GNU dircolors defaults.
func defaultExtensions() []dbEntry {
	return []dbEntry{
		// archives or compressed (bright red)
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
		// image formats
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
		// audio formats
		{"*.aac", "00;36"}, {"*.au", "00;36"}, {"*.flac", "00;36"},
		{"*.m4a", "00;36"}, {"*.mid", "00;36"}, {"*.midi", "00;36"},
		{"*.mka", "00;36"}, {"*.mp3", "00;36"}, {"*.mpc", "00;36"},
		{"*.ogg", "00;36"}, {"*.ra", "00;36"}, {"*.wav", "00;36"},
		{"*.oga", "00;36"}, {"*.opus", "00;36"}, {"*.spx", "00;36"},
		{"*.xspf", "00;36"},
		// backup files
		{"*~", "00;90"}, {"*#", "00;90"}, {"*.bak", "00;90"},
		{"*.crdownload", "00;90"}, {"*.dpkg-dist", "00;90"},
		{"*.dpkg-new", "00;90"}, {"*.dpkg-old", "00;90"},
		{"*.dpkg-tmp", "00;90"}, {"*.old", "00;90"}, {"*.orig", "00;90"},
		{"*.part", "00;90"}, {"*.rej", "00;90"}, {"*.rpmnew", "00;90"},
		{"*.rpmorig", "00;90"}, {"*.rpmsave", "00;90"}, {"*.swp", "00;90"},
		{"*.tmp", "00;90"}, {"*.ucf-dist", "00;90"}, {"*.ucf-new", "00;90"},
		{"*.ucf-old", "00;90"},
	}
}
