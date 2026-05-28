// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

var keywordToCode = map[string]string{
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

var ignoredKeywords = map[string]bool{
	"COLOR":    true,
	"OPTIONS":  true,
	"EIGHTBIT": true,
}

type shellFormat int

const (
	shellAuto shellFormat = iota
	shellBourne
	shellCsh
)

func main() {
	sys.InstallSIGPIPEHandler()

	shell := shellAuto
	printDB := false
	args := os.Args[1:]
	var positional []string

	for i := range len(args) {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		switch a {
		case "-b", "--sh", "--bourne-shell":
			shell = shellBourne
		case "-c", "--csh", "--c-shell":
			shell = shellCsh
		case "-p", "--print-database":
			printDB = true
		case "--help":
			fmt.Fprint(os.Stdout, "Usage: dircolors [OPTION]... [FILE]\nOutput commands to set the LS_COLORS environment variable.\n\nDetermine format of output:\n  -b, --sh, --bourne-shell    output Bourne shell code to set LS_COLORS\n  -c, --csh, --c-shell        output C shell code to set LS_COLORS\n  -p, --print-database        output defaults\n      --help        display this help and exit\n      --version     output version information and exit\n")
			os.Exit(0)
		case "--version":
			fmt.Fprint(os.Stdout, "dircolors (go-unix-utils) 0.1\n")
			os.Exit(0)
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				fmt.Fprintf(os.Stderr, "dircolors: invalid option -- '%s'\nTry 'dircolors --help' for more information.\n", a[1:])
				os.Exit(1)
			}
			positional = append(positional, a)
		}
	}

	if printDB && len(positional) > 0 {
		fmt.Fprintf(os.Stderr, "dircolors: extra operand '%s'\nfile operands cannot be combined with --print-database (-p)\nTry 'dircolors --help' for more information.\n", positional[0])
		os.Exit(1)
	}

	if printDB {
		fmt.Fprint(os.Stdout, defaultDatabase)
		os.Exit(0)
	}

	if len(positional) > 1 {
		fmt.Fprintf(os.Stderr, "dircolors: extra operand '%s'\nTry 'dircolors --help' for more information.\n", positional[1])
		os.Exit(1)
	}

	var reader io.Reader
	var filename string
	if len(positional) == 1 {
		filename = positional[0]
		if filename == "-" {
			reader = os.Stdin
		} else {
			f, err := os.Open(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "dircolors: %s\n", err)
				os.Exit(1)
			}
			defer f.Close()
			reader = f
		}
	} else {
		reader = strings.NewReader(defaultDatabase)
		filename = ""
	}

	lsColors, errs := parseDatabase(reader, filename)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
	}

	if shell == shellAuto {
		shell = detectShell()
	}

	switch shell {
	case shellBourne:
		fmt.Fprintf(os.Stdout, "LS_COLORS='%s';\nexport LS_COLORS\n", lsColors)
	case shellCsh:
		fmt.Fprintf(os.Stdout, "setenv LS_COLORS '%s'\n", lsColors)
	}
}

func detectShell() shellFormat {
	sh := os.Getenv("SHELL")
	base := filepath.Base(sh)
	if strings.HasSuffix(base, "csh") {
		return shellCsh
	}
	return shellBourne
}

func parseDatabase(r io.Reader, filename string) (string, []string) {
	scanner := bufio.NewScanner(r)
	var termEntries []string
	var colortermEntries []string
	var entries []string
	var errs []string
	lineNum := 0

	displayName := filename
	if displayName == "" {
		displayName = "<internal>"
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		fields := strings.Fields(line)
		keyword := fields[0]

		if len(fields) > 2 && fields[2][0] == '#' {
			fields = fields[:2]
		}

		if keyword == "TERM" {
			if len(fields) < 2 {
				errs = append(errs, fmt.Sprintf("dircolors: %s:%d: invalid line;  missing second token", displayName, lineNum))
				continue
			}
			termEntries = append(termEntries, fields[1])
			continue
		}

		if keyword == "COLORTERM" {
			if len(fields) < 2 {
				errs = append(errs, fmt.Sprintf("dircolors: %s:%d: invalid line;  missing second token", displayName, lineNum))
				continue
			}
			colortermEntries = append(colortermEntries, fields[1])
			continue
		}

		if ignoredKeywords[keyword] {
			continue
		}

		if code, ok := keywordToCode[keyword]; ok {
			if len(fields) < 2 {
				errs = append(errs, fmt.Sprintf("dircolors: %s:%d: invalid line;  missing second token", displayName, lineNum))
				continue
			}
			entries = append(entries, code+"="+fields[1])
			continue
		}

		if strings.HasPrefix(keyword, ".") || strings.HasPrefix(keyword, "*") {
			if len(fields) < 2 {
				errs = append(errs, fmt.Sprintf("dircolors: %s:%d: invalid line;  missing second token", displayName, lineNum))
				continue
			}
			ext := keyword
			if strings.HasPrefix(ext, ".") {
				ext = "*" + ext
			}
			entries = append(entries, ext+"="+fields[1])
			continue
		}

		if len(fields) < 2 {
			errs = append(errs, fmt.Sprintf("dircolors: %s:%d: invalid line;  missing second token", displayName, lineNum))
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		errs = append(errs, fmt.Sprintf("dircolors: %s: %v", displayName, err))
	}

	if len(errs) > 0 {
		return "", errs
	}

	if len(termEntries) > 0 || len(colortermEntries) > 0 {
		if !termMatches(termEntries, colortermEntries) {
			return "", nil
		}
	}

	if len(entries) == 0 {
		return "", nil
	}
	return strings.Join(entries, ":") + ":", nil
}

func termMatches(termEntries, colortermEntries []string) bool {
	currentTerm := os.Getenv("TERM")
	for _, pattern := range termEntries {
		matched, err := filepath.Match(pattern, currentTerm)
		if err == nil && matched {
			return true
		}
	}

	currentColorterm := os.Getenv("COLORTERM")
	for _, pattern := range colortermEntries {
		matched, err := filepath.Match(pattern, currentColorterm)
		if err == nil && matched {
			return true
		}
	}

	return false
}

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
