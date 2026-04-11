// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/stat: display file status.
// Implements srd082 R1.1, R2.1-R2.3, R3.1, R4.2.
package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "stat"

// blockSize is the fundamental block size for st_blocks (POSIX 512 bytes).
const blockSize = 512

// options holds parsed command-line flags.
type options struct {
	deref  bool
	terse  bool
	format string
	files  []string
}

// main entry point with SIGPIPE handler and argument dispatch.
// R7.3: InstallSIGPIPEHandler for graceful SIGPIPE exit.
func main() {
	sys.InstallSIGPIPEHandler()
	opts := parseArgs(os.Args[1:])
	if len(opts.files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr,
			"Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}
	exitCode := 0
	for _, f := range opts.files {
		if err := processFile(f, opts); err != nil {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into options.
// R2.1: accepts multiple file operands in order.
func parseArgs(args []string) options {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			opts.files = append(opts.files, args[i+1:]...)
			break
		}
		switch {
		case arg == "-L" || arg == "--dereference":
			opts.deref = true
		case arg == "-t" || arg == "--terse":
			opts.terse = true
		case arg == "-c" || arg == "--format":
			i++
			if i < len(args) {
				opts.format = args[i]
			}
		case strings.HasPrefix(arg, "-c"):
			opts.format = arg[2:]
		case strings.HasPrefix(arg, "--format="):
			opts.format = arg[len("--format="):]
		case strings.HasPrefix(arg, "-") && len(arg) > 1 &&
			!strings.HasPrefix(arg, "--"):
			i = parseShortFlags(arg[1:], args, i, &opts)
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return opts
}

// parseShortFlags handles combined short flags like -Lt.
func parseShortFlags(flags string, args []string, i int, opts *options) int {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'L':
			opts.deref = true
		case 't':
			opts.terse = true
		case 'c':
			if j+1 < len(flags) {
				opts.format = flags[j+1:]
			} else if i+1 < len(args) {
				i++
				opts.format = args[i]
			}
			return i
		default:
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n",
				programName, flags[j])
			fmt.Fprintf(os.Stderr,
				"Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		}
	}
	return i
}

// processFile stats a file and prints output according to options.
// R2.3: returns error on stat failure, prints error to stderr.
func processFile(path string, opts options) error {
	fi, err := statFile(path, opts.deref)
	if err != nil {
		reportError(path, err)
		return err
	}
	switch {
	case opts.format != "":
		fmt.Println(expandFormat(opts.format, fi, path))
	case opts.terse:
		fmt.Print(formatTerse(fi, path))
	default:
		fmt.Print(expandFormat(defaultFormat(fi, path), fi, path))
	}
	return nil
}

// statFile returns file info using Stat (with -L) or Lstat (without).
// R2.2: -L follows symlinks.
func statFile(path string, deref bool) (*sys.FileInfo, error) {
	if deref {
		return sys.Stat(path)
	}
	return sys.Lstat(path)
}

// reportError prints a stat error to stderr matching GNU format.
func reportError(path string, err error) {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "%s: cannot stat '%s': %s\n",
		programName, path, msg)
}

// formatSpec holds a parsed format directive with optional width/flags.
type formatSpec struct {
	flags     string
	width     string
	precision string
	directive byte
}

// defaultFormat returns the default format string for file stat output.
// R1.1: multi-line format matching GNU stat default (unquoted names,
// device as major,minor decimal).
func defaultFormat(fi *sys.FileInfo, path string) string {
	fileName := strings.ReplaceAll(
		defaultFileName(fi, path), "%", "%%")
	major := deviceMajor(fi.Dev)
	minor := deviceMinor(fi.Dev)
	base := fmt.Sprintf("  File: %s\n", fileName) +
		"  Size: %-10s\tBlocks: %-10b IO Block: %-6o %F\n" +
		fmt.Sprintf("Device: %d,%d\tInode: %%-10i  Links: %%h\n",
			major, minor)
	if fi.Mode&os.ModeDevice != 0 {
		base += "Device type: %t,%T\n"
	}
	base += "Access: (%04a/%10.10A)  Uid: (%5u/%8U)   Gid: (%5g/%8G)\n" +
		"Access: %x\n" +
		"Modify: %y\n" +
		"Change: %z\n" +
		" Birth: %w\n"
	return base
}

// defaultFileName returns the unquoted filename for the default File: line.
// For symlinks, includes " -> target".
func defaultFileName(fi *sys.FileInfo, path string) string {
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return path
		}
		return path + " -> " + target
	}
	return path
}

// formatTerse formats file status in terse mode.
// R4.2: single line of space-separated fields matching GNU stat --terse.
func formatTerse(fi *sys.FileInfo, path string) string {
	return fmt.Sprintf(
		"%s %d %d %x %d %d %x %d %d %x %x %d %d %d %d %d\n",
		path, fi.Size, fi.Blocks, rawMode(fi),
		fi.Uid, fi.Gid, fi.Dev, fi.Ino, fi.Nlink,
		deviceMajor(fi.Rdev), deviceMinor(fi.Rdev),
		fi.AccessTime.Unix(), fi.ModTime.Unix(),
		fi.ChangeTime.Unix(), birthEpoch(fi), fi.Blksize)
}

// expandFormat expands format directives in the format string.
// R3.1: supports all standard format directives.
func expandFormat(format string, fi *sys.FileInfo, path string) string {
	var buf strings.Builder
	for i := 0; i < len(format); {
		if format[i] != '%' {
			buf.WriteByte(format[i])
			i++
			continue
		}
		i++ // skip '%'
		if i >= len(format) {
			buf.WriteByte('%')
			break
		}
		if format[i] == '%' {
			buf.WriteByte('%')
			i++
			continue
		}
		spec, n := parseSpec(format[i:])
		i += n
		buf.WriteString(applyDirective(spec, fi, path))
	}
	return buf.String()
}

// parseSpec parses flags, width, precision, and directive letter.
func parseSpec(s string) (formatSpec, int) {
	var spec formatSpec
	i := 0
	for i < len(s) && strings.ContainsRune("-+0# ", rune(s[i])) {
		spec.flags += string(s[i])
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		spec.width += string(s[i])
		i++
	}
	if i < len(s) && s[i] == '.' {
		spec.precision = "."
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			spec.precision += string(s[i])
			i++
		}
	}
	if i < len(s) {
		spec.directive = s[i]
		i++
	}
	return spec, i
}

// applyDirective formats a single directive value with width/flags.
func applyDirective(spec formatSpec, fi *sys.FileInfo, path string) string {
	d := spec.directive
	prefix := "%" + spec.flags + spec.width + spec.precision
	if isNumericDirective(d) {
		val := numericValue(d, fi, path)
		base := directiveBase(d)
		switch base {
		case 8:
			return fmt.Sprintf(prefix+"o", val)
		case 16:
			return fmt.Sprintf(prefix+"x", val)
		default:
			return fmt.Sprintf(prefix+"d", val)
		}
	}
	if isStringDirective(d) {
		return fmt.Sprintf(prefix+"s", stringValue(d, fi, path))
	}
	return "%" + string(d)
}

// isNumericDirective returns true for directives that produce numeric values.
func isNumericDirective(d byte) bool {
	switch d {
	case 'a', 'b', 'B', 'd', 'D', 'f', 'g', 'h', 'i',
		'o', 's', 't', 'T', 'u', 'W', 'X', 'Y', 'Z':
		return true
	}
	return false
}

// isStringDirective returns true for directives that produce string values.
func isStringDirective(d byte) bool {
	switch d {
	case 'A', 'F', 'G', 'm', 'n', 'N', 'U', 'w', 'x', 'y', 'z':
		return true
	}
	return false
}

// directiveBase returns the numeric base for a directive (8, 10, or 16).
func directiveBase(d byte) int {
	switch d {
	case 'a':
		return 8
	case 'D', 'f', 't', 'T':
		return 16
	default:
		return 10
	}
}

// numericValue returns the numeric value for a format directive.
func numericValue(d byte, fi *sys.FileInfo, _ string) int64 {
	switch d {
	case 'a':
		return int64(rawMode(fi) & 07777)
	case 'b':
		return fi.Blocks
	case 'B':
		return blockSize
	case 'd':
		return int64(fi.Dev)
	case 'D':
		return int64(fi.Dev)
	case 'f':
		return int64(rawMode(fi))
	case 'g':
		return int64(fi.Gid)
	case 'h':
		return int64(fi.Nlink)
	case 'i':
		return int64(fi.Ino)
	case 'o':
		return fi.Blksize
	case 's':
		return fi.Size
	case 't':
		return int64(deviceMajor(fi.Rdev))
	case 'T':
		return int64(deviceMinor(fi.Rdev))
	case 'u':
		return int64(fi.Uid)
	case 'W':
		return birthEpoch(fi)
	case 'X':
		return fi.AccessTime.Unix()
	case 'Y':
		return fi.ModTime.Unix()
	case 'Z':
		return fi.ChangeTime.Unix()
	default:
		return 0
	}
}

// stringValue returns the string value for a format directive.
func stringValue(d byte, fi *sys.FileInfo, path string) string {
	switch d {
	case 'A':
		return humanPerms(fi)
	case 'F':
		return fileTypeName(fi)
	case 'G':
		return lookupGroup(fi.Gid)
	case 'm':
		return mountPoint(path)
	case 'n':
		return path
	case 'N':
		return quotedName(fi, path)
	case 'U':
		return lookupUser(fi.Uid)
	case 'w':
		return birthTimeStr(fi)
	case 'x':
		return formatTime(fi.AccessTime)
	case 'y':
		return formatTime(fi.ModTime)
	case 'z':
		return formatTime(fi.ChangeTime)
	default:
		return ""
	}
}

// rawMode returns the raw Unix st_mode value from syscall.Stat_t.
func rawMode(fi *sys.FileInfo) uint32 {
	if stat, ok := fi.Info.Sys().(*syscall.Stat_t); ok {
		return uint32(stat.Mode)
	}
	return uint32(fi.Mode.Perm())
}

// deviceMajor extracts the major device number (Darwin encoding).
func deviceMajor(rdev uint64) uint32 { return uint32((rdev >> 24) & 0xff) }

// deviceMinor extracts the minor device number (Darwin encoding).
func deviceMinor(rdev uint64) uint32 { return uint32(rdev & 0xffffff) }

// fileTypeName returns the human-readable file type name.
// R1.1: matches GNU stat file type names.
func fileTypeName(fi *sys.FileInfo) string {
	mode := fi.Mode
	switch {
	case mode.IsRegular() && fi.Size == 0:
		return "regular empty file"
	case mode.IsRegular():
		return "regular file"
	case mode.IsDir():
		return "directory"
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode&os.ModeNamedPipe != 0:
		return "fifo"
	case mode&os.ModeSocket != 0:
		return "socket"
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return "character special file"
	case mode&os.ModeDevice != 0:
		return "block special file"
	default:
		return "weird file"
	}
}

// typeChar returns the file type character for the permission string.
func typeChar(mode os.FileMode) byte {
	switch {
	case mode.IsDir():
		return 'd'
	case mode&os.ModeSymlink != 0:
		return 'l'
	case mode&os.ModeNamedPipe != 0:
		return 'p'
	case mode&os.ModeSocket != 0:
		return 's'
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return 'c'
	case mode&os.ModeDevice != 0:
		return 'b'
	default:
		return '-'
	}
}

// humanPerms returns the human-readable permission string like -rwxr-xr-x.
// R3.1: %A directive.
func humanPerms(fi *sys.FileInfo) string {
	raw := rawMode(fi)
	buf := [10]byte{
		typeChar(fi.Mode),
		'-', '-', '-', '-', '-', '-', '-', '-', '-',
	}
	permBits(raw, buf[:])
	specialBits(fi.Mode, buf[:])
	return string(buf[:])
}

// permBits fills the 9 permission characters from raw mode bits.
func permBits(raw uint32, buf []byte) {
	const chars = "rwx"
	for i := 0; i < 9; i++ {
		if raw&(1<<uint(8-i)) != 0 {
			buf[i+1] = chars[i%3]
		}
	}
}

// specialBits applies setuid, setgid, and sticky bit overrides.
func specialBits(mode os.FileMode, buf []byte) {
	if mode&os.ModeSetuid != 0 {
		if buf[3] == 'x' {
			buf[3] = 's'
		} else {
			buf[3] = 'S'
		}
	}
	if mode&os.ModeSetgid != 0 {
		if buf[6] == 'x' {
			buf[6] = 's'
		} else {
			buf[6] = 'S'
		}
	}
	if mode&os.ModeSticky != 0 {
		if buf[9] == 'x' {
			buf[9] = 't'
		} else {
			buf[9] = 'T'
		}
	}
}

// formatTime formats a time in GNU stat's human-readable format.
// R3.1: %x, %y, %z, %w directives.
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05") +
		fmt.Sprintf(".%09d ", t.Nanosecond()) +
		t.Format("-0700")
}

// birthTimeStr returns the birth time as a human-readable string, or "-".
func birthTimeStr(fi *sys.FileInfo) string {
	bt, ok := birthTime(fi)
	if !ok {
		return "-"
	}
	return formatTime(bt)
}

// birthEpoch returns the birth time as epoch seconds, or 0.
func birthEpoch(fi *sys.FileInfo) int64 {
	bt, ok := birthTime(fi)
	if !ok {
		return 0
	}
	return bt.Unix()
}

// quotedName returns the filename quoted, with symlink target for links.
// R3.1: %N directive.
func quotedName(fi *sys.FileInfo, path string) string {
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return quoteStr(path)
		}
		return quoteStr(path) + " -> " + quoteStr(target)
	}
	return quoteStr(path)
}

// quoteStr wraps s in single quotes.
func quoteStr(s string) string { return "'" + s + "'" }

// lookupUser returns the username for a UID, or the UID string.
func lookupUser(uid uint32) string {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// lookupGroup returns the group name for a GID, or the GID string.
func lookupGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// mountPoint returns the mount point for the given path.
// R3.1: %m directive.
func mountPoint(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	dev := deviceOfPath(abs)
	if dev == 0 {
		return abs
	}
	return walkToMount(abs, dev)
}

// deviceOfPath returns the device ID for a path via syscall.Stat.
func deviceOfPath(path string) uint64 {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0
	}
	return uint64(st.Dev)
}

// walkToMount walks up the directory tree until the device ID changes.
func walkToMount(path string, dev uint64) string {
	for {
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		if deviceOfPath(parent) != dev {
			return path
		}
		path = parent
	}
}
