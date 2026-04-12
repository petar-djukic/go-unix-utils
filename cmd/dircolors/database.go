// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Database parsing and built-in default database for dircolors.
// Implements srd109 R2.1–R2.5, R3.1, R3.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// dbEntry is a single key=value pair in the compiled color database.
type dbEntry struct {
	key   string
	value string
}

// loadDatabase reads a database from filename, stdin ("-"), or built-in defaults.
// R2.4/R2.5: file argument or stdin; no argument uses built-in defaults.
func loadDatabase(filename string) ([]dbEntry, error) {
	if filename == "" {
		return parseDatabaseString(builtinDatabase, "(built-in)")
	}
	if filename == "-" {
		return parseDatabase(os.Stdin, "(stdin)")
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, formatOpenError(filename, err)
	}
	defer f.Close() // best-effort close
	return parseDatabase(f, filename)
}

// formatOpenError formats a file-open error to match GNU dircolors format.
// R3.4: "FILE: Reason" (e.g., "file.db: No such file or directory").
func formatOpenError(filename string, err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		reason := capitalizeFirst(pathErr.Err.Error())
		return fmt.Errorf("%s: %s", filename, reason)
	}
	return fmt.Errorf("%s: %v", filename, err)
}

// capitalizeFirst uppercases the first character of s.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseDatabaseString parses a database from a string.
func parseDatabaseString(data, source string) ([]dbEntry, error) {
	return parseDatabase(strings.NewReader(data), source)
}

// parseDatabase reads lines from r and extracts dbEntry values.
// R2.1: parses comments, blanks, TERM lines, keywords, extensions.
// R2.2: TERM/COLORTERM lines control whether the database applies.
// R3.4: collects all errors and reports them; does not stop at first.
func parseDatabase(r io.Reader, source string) ([]dbEntry, error) {
	scanner := bufio.NewScanner(r)
	var state parseState
	var errs []string
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := parseLine(line, source, lineNum, &state); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// R3.4: report all errors from database parsing.
	if len(errs) > 0 {
		return nil, &parseErrors{msgs: errs}
	}

	// R2.2: if TERM/COLORTERM lines present but none match, empty output.
	if state.hasTermFilter && !termMatches(&state) {
		return nil, nil
	}
	return state.entries, nil
}

// parseState accumulates results during database parsing.
type parseState struct {
	entries       []dbEntry
	termPatterns  []string
	colorPatterns []string
	hasTermFilter bool
}

// parseLine processes a single non-comment, non-blank database line.
// R3.4: returns error with source:line format for invalid lines.
func parseLine(line, source string, lineNum int, state *parseState) error {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		// R3.4: match GNU format for missing second token (double space after semicolon).
		return fmt.Errorf("%s:%d: invalid line;  missing second token",
			source, lineNum)
	}
	keyword, value := fields[0], fields[1]

	switch keyword {
	case "TERM":
		state.hasTermFilter = true
		state.termPatterns = append(state.termPatterns, value)
		return nil
	case "COLORTERM":
		state.hasTermFilter = true
		state.colorPatterns = append(state.colorPatterns, value)
		return nil
	case "COLOR", "OPTIONS", "EIGHTBIT":
		// Recognized but ignored per GNU dircolors documentation.
		return nil
	}

	if lsKey, ok := keywordToLS(keyword); ok {
		state.entries = append(state.entries, dbEntry{key: lsKey, value: value})
		return nil
	}

	return parseExtension(keyword, value, source, lineNum, state)
}

// parseExtension handles extension entries: ".ext" or "*.ext" or "*X".
func parseExtension(
	keyword, value, source string, lineNum int, state *parseState,
) error {
	if strings.HasPrefix(keyword, "*") || strings.HasPrefix(keyword, ".") {
		extKey := keyword
		if strings.HasPrefix(keyword, ".") {
			extKey = "*" + keyword
		}
		state.entries = append(state.entries, dbEntry{key: extKey, value: value})
		return nil
	}
	return fmt.Errorf("%s:%d: unrecognized keyword %s", source, lineNum, keyword)
}

// termMatches checks if the current TERM or COLORTERM matches any filter.
// R2.2: uses glob pattern matching for TERM/COLORTERM entries.
func termMatches(state *parseState) bool {
	term := os.Getenv("TERM")
	for _, pat := range state.termPatterns {
		if globMatch(pat, term) {
			return true
		}
	}
	colorterm := os.Getenv("COLORTERM")
	for _, pat := range state.colorPatterns {
		if globMatch(pat, colorterm) {
			return true
		}
	}
	return false
}

// globMatch performs glob pattern matching using filepath.Match.
func globMatch(pattern, value string) bool {
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		return false
	}
	return matched
}

// keywordToLS maps a database keyword to its LS_COLORS two-letter code.
// R2.3: complete keyword-to-code mapping.
func keywordToLS(keyword string) (string, bool) {
	code, ok := keywordMap[keyword]
	return code, ok
}

var keywordMap = map[string]string{
	"NORMAL":                "no",
	"FILE":                  "fi",
	"RESET":                 "rs",
	"DIR":                   "di",
	"LINK":                  "ln",
	"MULTIHARDLINK":         "mh",
	"FIFO":                  "pi",
	"SOCK":                  "so",
	"DOOR":                  "do",
	"BLK":                   "bd",
	"CHR":                   "cd",
	"ORPHAN":                "or",
	"MISSING":               "mi",
	"SETUID":                "su",
	"SETGID":                "sg",
	"CAPABILITY":            "ca",
	"STICKY_OTHER_WRITABLE": "tw",
	"OTHER_WRITABLE":        "ow",
	"STICKY":                "st",
	"EXEC":                  "ex",
	"LEFT":                  "lc",
	"RIGHT":                 "rc",
	"END":                   "ec",
}

// builtinDatabase is the GNU dircolors default database text.
// R3.1: must match gdircolors --print-database byte-for-byte.
const builtinDatabase = "" +
	"# Configuration file for dircolors, a utility to help you set the\n" +
	"# LS_COLORS environment variable used by GNU ls with the --color option.\n" +
	"# Copyright (C) 1996-2026 Free Software Foundation, Inc.\n" +
	"# Copying and distribution of this file, with or without modification,\n" +
	"# are permitted provided the copyright notice and this notice are preserved.\n" +
	"#\n" +
	"# The keywords COLOR, OPTIONS, and EIGHTBIT (honored by the\n" +
	"# slackware version of dircolors) are recognized but ignored.\n" +
	"# Global config options can be specified before TERM or COLORTERM entries\n" +
	"# ===================================================================\n" +
	"# Terminal filters\n" +
	"# ===================================================================\n" +
	"# Below are TERM or COLORTERM entries, which can be glob patterns, which\n" +
	"# restrict following config to systems with matching environment variables.\n" +
	"COLORTERM ?*\n" +
	"TERM Eterm\n" +
	"TERM ansi\n" +
	"TERM *color*\n" +
	"TERM con[0-9]*x[0-9]*\n" +
	"TERM cons25\n" +
	"TERM console\n" +
	"TERM cygwin\n" +
	"TERM *direct*\n" +
	"TERM dtterm\n" +
	"TERM gnome\n" +
	"TERM hurd\n" +
	"TERM jfbterm\n" +
	"TERM konsole\n" +
	"TERM kterm\n" +
	"TERM linux\n" +
	"TERM linux-c\n" +
	"TERM mlterm\n" +
	"TERM putty\n" +
	"TERM rxvt*\n" +
	"TERM screen*\n" +
	"TERM st\n" +
	"TERM terminator\n" +
	"TERM tmux*\n" +
	"TERM vt100\n" +
	"TERM vt220\n" +
	"TERM xterm*\n" +
	"# ===================================================================\n" +
	"# Basic file attributes\n" +
	"# ===================================================================\n" +
	"# Below are the color init strings for the basic file types.\n" +
	"# One can use codes for 256 or more colors supported by modern terminals.\n" +
	"# The default color codes use the capabilities of an 8 color terminal\n" +
	"# with some additional attributes as per the following codes:\n" +
	"# Attribute codes:\n" +
	"# 00=none 01=bold 04=underscore 05=blink 07=reverse 08=concealed\n" +
	"# Text color codes:\n" +
	"# 30=black 31=red 32=green 33=yellow 34=blue 35=magenta 36=cyan 37=white\n" +
	"# Background color codes:\n" +
	"# 40=black 41=red 42=green 43=yellow 44=blue 45=magenta 46=cyan 47=white\n" +
	"#NORMAL 00 # no color code at all\n" +
	"#FILE 00 # regular file: use no color at all\n" +
	"RESET 0 # reset to \"normal\" color\n" +
	"DIR 01;34 # directory\n" +
	"LINK 01;36 # symbolic link. (If you set this to 'target' instead of a\n" +
	" # numerical value, the color is as for the file pointed to.)\n" +
	"MULTIHARDLINK 00 # regular file with more than one link\n" +
	"FIFO 40;33 # pipe\n" +
	"SOCK 01;35 # socket\n" +
	"DOOR 01;35 # door\n" +
	"BLK 40;33;01 # block device driver\n" +
	"CHR 40;33;01 # character device driver\n" +
	"ORPHAN 40;31;01 # symlink to nonexistent file, or non-stat'able file ...\n" +
	"MISSING 00 # ... and the files they point to\n" +
	"SETUID 37;41 # regular file that is setuid (u+s)\n" +
	"SETGID 30;43 # regular file that is setgid (g+s)\n" +
	"CAPABILITY 00 # regular file with capability (very expensive to lookup)\n" +
	"STICKY_OTHER_WRITABLE 30;42 # dir that is sticky and other-writable (+t,o+w)\n" +
	"OTHER_WRITABLE 34;42 # dir that is other-writable (o+w) and not sticky\n" +
	"STICKY 37;44 # dir with the sticky bit set (+t) and not other-writable\n" +
	"# This is for regular files with execute permission:\n" +
	"EXEC 01;32\n" +
	"# ===================================================================\n" +
	"# File extension attributes\n" +
	"# ===================================================================\n" +
	"# List any file extensions like '.gz' or '.tar' that you would like ls\n" +
	"# to color below. Put the suffix, a space, and the color init string.\n" +
	"# (and any comments you want to add after a '#').\n" +
	"# Suffixes are matched case insensitively, but if you define different\n" +
	"# init strings for separate cases, those will be honored.\n" +
	"#\n" +
	"# If you use DOS-style suffixes, you may want to uncomment the following:\n" +
	"#.cmd 01;32 # executables (bright green)\n" +
	"#.exe 01;32\n" +
	"#.com 01;32\n" +
	"#.btm 01;32\n" +
	"#.bat 01;32\n" +
	"# Or if you want to color scripts even if they do not have the\n" +
	"# executable bit actually set.\n" +
	"#.sh 01;32\n" +
	"#.csh 01;32\n" +
	"# archives or compressed (bright red)\n" +
	".7z 01;31\n" +
	".ace 01;31\n" +
	".alz 01;31\n" +
	".apk 01;31\n" +
	".arc 01;31\n" +
	".arj 01;31\n" +
	".bz 01;31\n" +
	".bz2 01;31\n" +
	".cab 01;31\n" +
	".cpio 01;31\n" +
	".crate 01;31\n" +
	".deb 01;31\n" +
	".drpm 01;31\n" +
	".dwm 01;31\n" +
	".dz 01;31\n" +
	".ear 01;31\n" +
	".egg 01;31\n" +
	".esd 01;31\n" +
	".gz 01;31\n" +
	".jar 01;31\n" +
	".lha 01;31\n" +
	".lrz 01;31\n" +
	".lz 01;31\n" +
	".lz4 01;31\n" +
	".lzh 01;31\n" +
	".lzma 01;31\n" +
	".lzo 01;31\n" +
	".pyz 01;31\n" +
	".rar 01;31\n" +
	".rpm 01;31\n" +
	".rz 01;31\n" +
	".sar 01;31\n" +
	".swm 01;31\n" +
	".t7z 01;31\n" +
	".tar 01;31\n" +
	".taz 01;31\n" +
	".tbz 01;31\n" +
	".tbz2 01;31\n" +
	".tgz 01;31\n" +
	".tlz 01;31\n" +
	".txz 01;31\n" +
	".tz 01;31\n" +
	".tzo 01;31\n" +
	".tzst 01;31\n" +
	".udeb 01;31\n" +
	".war 01;31\n" +
	".whl 01;31\n" +
	".wim 01;31\n" +
	".xz 01;31\n" +
	".z 01;31\n" +
	".zip 01;31\n" +
	".zoo 01;31\n" +
	".zst 01;31\n" +
	"# image formats\n" +
	".avif 01;35\n" +
	".jpg 01;35\n" +
	".jpeg 01;35\n" +
	".jxl 01;35\n" +
	".mjpg 01;35\n" +
	".mjpeg 01;35\n" +
	".gif 01;35\n" +
	".bmp 01;35\n" +
	".pbm 01;35\n" +
	".pgm 01;35\n" +
	".ppm 01;35\n" +
	".tga 01;35\n" +
	".xbm 01;35\n" +
	".xpm 01;35\n" +
	".tif 01;35\n" +
	".tiff 01;35\n" +
	".png 01;35\n" +
	".svg 01;35\n" +
	".svgz 01;35\n" +
	".mng 01;35\n" +
	".pcx 01;35\n" +
	".mov 01;35\n" +
	".mpg 01;35\n" +
	".mpeg 01;35\n" +
	".m2v 01;35\n" +
	".mkv 01;35\n" +
	".webm 01;35\n" +
	".webp 01;35\n" +
	".ogm 01;35\n" +
	".mp4 01;35\n" +
	".m4v 01;35\n" +
	".mp4v 01;35\n" +
	".vob 01;35\n" +
	".qt 01;35\n" +
	".nuv 01;35\n" +
	".wmv 01;35\n" +
	".asf 01;35\n" +
	".rm 01;35\n" +
	".rmvb 01;35\n" +
	".flc 01;35\n" +
	".avi 01;35\n" +
	".fli 01;35\n" +
	".flv 01;35\n" +
	".gl 01;35\n" +
	".dl 01;35\n" +
	".xcf 01;35\n" +
	".xwd 01;35\n" +
	".yuv 01;35\n" +
	".cgm 01;35\n" +
	".emf 01;35\n" +
	"# https://wiki.xiph.org/MIME_Types_and_File_Extensions\n" +
	".ogv 01;35\n" +
	".ogx 01;35\n" +
	"# audio formats\n" +
	".aac 00;36\n" +
	".au 00;36\n" +
	".flac 00;36\n" +
	".m4a 00;36\n" +
	".mid 00;36\n" +
	".midi 00;36\n" +
	".mka 00;36\n" +
	".mp3 00;36\n" +
	".mpc 00;36\n" +
	".ogg 00;36\n" +
	".ra 00;36\n" +
	".wav 00;36\n" +
	"# https://wiki.xiph.org/MIME_Types_and_File_Extensions\n" +
	".oga 00;36\n" +
	".opus 00;36\n" +
	".spx 00;36\n" +
	".xspf 00;36\n" +
	"# backup files\n" +
	"*~ 00;90\n" +
	"*# 00;90\n" +
	".bak 00;90\n" +
	".crdownload 00;90\n" +
	".dpkg-dist 00;90\n" +
	".dpkg-new 00;90\n" +
	".dpkg-old 00;90\n" +
	".dpkg-tmp 00;90\n" +
	".old 00;90\n" +
	".orig 00;90\n" +
	".part 00;90\n" +
	".rej 00;90\n" +
	".rpmnew 00;90\n" +
	".rpmorig 00;90\n" +
	".rpmsave 00;90\n" +
	".swp 00;90\n" +
	".tmp 00;90\n" +
	".ucf-dist 00;90\n" +
	".ucf-new 00;90\n" +
	".ucf-old 00;90\n" +
	"#\n" +
	"# Subsequent TERM or COLORTERM entries, can be used to add / override\n" +
	"# config specific to those matching environment variables.\n"
