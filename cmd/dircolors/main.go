// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// dircolors outputs shell commands to set the LS_COLORS environment variable.
// Implements prd109-dircolors R1.1-R1.4, R2.1-R2.5, R3.1-R3.5.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R1.1, R1.2: shell output format constants.
const (
	shellBourne = iota
	shellC
)

// R2.3: keywordToCode maps file type keywords to LS_COLORS two-letter codes.
var keywordToCode = map[string]string{
	"NORMAL":                 "no",
	"FILE":                   "fi",
	"RESET":                  "rs",
	"DIR":                    "di",
	"LINK":                   "ln",
	"MULTIHARDLINK":          "mh",
	"FIFO":                   "pi",
	"SOCK":                   "so",
	"DOOR":                   "do",
	"BLK":                    "bd",
	"CHR":                    "cd",
	"ORPHAN":                 "or",
	"MISSING":                "mi",
	"SETUID":                 "su",
	"SETGID":                 "sg",
	"CAPABILITY":             "ca",
	"STICKY_OTHER_WRITABLE":  "tw",
	"OTHER_WRITABLE":         "ow",
	"STICKY":                 "st",
	"EXEC":                   "ex",
	"LEFT":                   "lc",
	"RIGHT":                  "rc",
	"END":                    "ec",
}

// ignoredKeywords are recognized by GNU dircolors but produce no LS_COLORS output.
var ignoredKeywords = map[string]bool{
	"COLOR": true, "OPTIONS": true, "EIGHTBIT": true,
}

// dbState accumulates entries during database parsing.
type dbState struct {
	terms      []string
	colorterms []string
	pairs      []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	shellMode, printDB, filename := parseArgs(os.Args[1:])

	if printDB && filename != "" {
		fmt.Fprintf(os.Stderr,
			"dircolors: extra operand '%s'\n"+
				"file operands cannot be combined with "+
				"--print-database (-p)\n", filename)
		os.Exit(1)
	}

	if printDB {
		fmt.Print(defaultDatabase)
		return
	}

	lsColors, err := loadAndParse(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	printShellOutput(shellMode, lsColors)
}

// R1.3: detectShell auto-detects shell from SHELL environment variable.
func detectShell() int {
	shell := os.Getenv("SHELL")
	if strings.HasSuffix(shell, "csh") {
		return shellC
	}
	return shellBourne
}

// R1.4, R2.4: parseArgs processes flags and extracts the filename argument.
// Last -b or -c wins.
func parseArgs(args []string) (int, bool, string) {
	mode := detectShell()
	var printDB bool
	var filename string
	for _, arg := range args {
		switch arg {
		case "-b", "--sh", "--bourne-shell":
			mode = shellBourne
		case "-c", "--csh", "--c-shell":
			mode = shellC
		case "-p", "--print-database":
			printDB = true
		default:
			if !strings.HasPrefix(arg, "-") || arg == "-" {
				filename = arg
			}
		}
	}
	return mode, printDB, filename
}

// R1.1, R1.2: printShellOutput writes the LS_COLORS assignment in the
// requested shell syntax.
func printShellOutput(mode int, value string) {
	switch mode {
	case shellC:
		fmt.Printf("setenv LS_COLORS '%s'\n", value)
	default:
		fmt.Printf("LS_COLORS='%s';\nexport LS_COLORS\n", value)
	}
}

// R2.4, R2.5: loadAndParse reads the database from a file, stdin ("-"),
// or the built-in defaults and parses it into an LS_COLORS string.
func loadAndParse(filename string) (string, error) {
	var content string
	switch {
	case filename == "-":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("dircolors: %v", err)
		}
		content = string(data)
	case filename != "":
		data, err := os.ReadFile(filename)
		if err != nil {
			return "", formatFileError(filename, err)
		}
		content = string(data)
	default:
		content = defaultDatabase
	}
	return parseDatabase(content, filename)
}

// R2.1: parseDatabase parses GNU dircolors database format and returns
// the LS_COLORS value string.
func parseDatabase(content, filename string) (string, error) {
	var state dbState
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := stripComment(scanner.Text())
		if line == "" {
			continue
		}
		if err := parseLine(&state, line, filename, lineNum); err != nil {
			return "", err
		}
	}
	if !state.matchesTerminal() {
		return "", nil
	}
	if len(state.pairs) == 0 {
		return "", nil
	}
	return strings.Join(state.pairs, ":") + ":", nil
}

// parseLine processes a single non-empty, non-comment database line.
func parseLine(s *dbState, line, filename string, lineNum int) error {
	parts := strings.Fields(line)
	keyword := parts[0]
	if keyword == "TERM" {
		if len(parts) >= 2 {
			s.terms = append(s.terms, parts[1])
		}
		return nil
	}
	if keyword == "COLORTERM" {
		if len(parts) >= 2 {
			s.colorterms = append(s.colorterms, parts[1])
		}
		return nil
	}
	if ignoredKeywords[keyword] {
		return nil
	}
	if len(parts) < 2 {
		return dbError(filename, lineNum, keyword)
	}
	value := parts[1]
	if code, ok := keywordToCode[keyword]; ok {
		s.pairs = append(s.pairs, code+"="+value)
		return nil
	}
	if keyword[0] == '*' || keyword[0] == '.' {
		return parseExtension(s, keyword, value)
	}
	return dbError(filename, lineNum, keyword)
}

// parseExtension handles extension entries (lines starting with . or *).
func parseExtension(s *dbState, keyword, value string) error {
	ext := keyword
	if keyword[0] == '.' {
		ext = "*" + ext
	}
	s.pairs = append(s.pairs, ext+"="+value)
	return nil
}

// R2.2: matchesTerminal checks if the current terminal matches any TERM
// or COLORTERM pattern in the database.
func (s *dbState) matchesTerminal() bool {
	if len(s.terms) == 0 && len(s.colorterms) == 0 {
		return true
	}
	term := os.Getenv("TERM")
	for _, pattern := range s.terms {
		if matched, _ := path.Match(pattern, term); matched {
			return true
		}
	}
	colorterm := os.Getenv("COLORTERM")
	for _, pattern := range s.colorterms {
		if matched, _ := path.Match(pattern, colorterm); matched {
			return true
		}
	}
	return false
}

// stripComment removes comments from a database line. A '#' starts a comment
// only at the beginning of a line or after whitespace, not inside tokens like *#.
func stripComment(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
			line = line[:i]
			break
		}
	}
	return strings.TrimSpace(line)
}

// R3.4: formatFileError produces GNU-compatible error messages for file I/O errors.
// GNU format: "dircolors: <path>: <OS error>" (no "open" prefix, capitalized).
func formatFileError(filename string, err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("dircolors: %s: %v", filename, pathErr.Err)
	}
	return fmt.Errorf("dircolors: %s: %v", filename, err)
}

func dbError(filename string, lineNum int, keyword string) error {
	src := filename
	if src == "" {
		src = "<internal>"
	}
	return fmt.Errorf("dircolors: %s:%d: unrecognized keyword %s",
		src, lineNum, keyword)
}

// defaultDatabase is the built-in GNU dircolors default color database.
// This must match the output of gdircolors --print-database byte-for-byte.
const defaultDatabase = `# Configuration file for dircolors, a utility to help you set the
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
