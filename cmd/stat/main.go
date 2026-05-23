// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd082-stat R1.1, R2.1-R2.3, R3.1, R4.1-R4.2, R5.1, R6.1, R7.1-R7.3.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: stat [OPTION]... FILE...
Display file or file system status.

Mandatory arguments to long options are mandatory for short options too.
  -L, --dereference     follow links
  -f, --file-system     display file system status instead of file status
  -c  --format=FORMAT   use the specified FORMAT instead of the default;
                          output a newline after each use of FORMAT
      --printf=FORMAT   like --format, but interpret backslash escapes,
                          and do not output a mandatory trailing newline;
                          if you want a newline, include \n in FORMAT
  -t, --terse           print the information in terse form
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `stat (go-unix-utils) dev
`

type options struct {
	dereference bool
	fileSystem  bool
	format      string
	printf      string
	terse       bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'stat --help' for more information.\n")
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "stat: missing operand\n")
		fmt.Fprintf(os.Stderr, "Try 'stat --help' for more information.\n")
		os.Exit(1)
	}
	os.Exit(run(opts, files))
}

func run(opts options, files []string) int {
	exitCode := 0
	for _, path := range files {
		if err := statPath(opts, path); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func statPath(opts options, path string) error {
	if opts.fileSystem {
		return statFS(opts, path)
	}
	return statFile(opts, path)
}

func statFile(opts options, path string) error {
	var fi *sys.FileInfo
	var err error
	if opts.dereference {
		fi, err = sys.Stat(path)
	} else {
		fi, err = sys.Lstat(path)
	}
	if err != nil {
		return fmtError(path, err)
	}
	output := fileOutput(opts, fi, path)
	fmt.Fprint(os.Stdout, output)
	return nil
}

func fileOutput(opts options, fi *sys.FileInfo, path string) string {
	if opts.printf != "" {
		return expandFormat(fi, path, opts.printf, false)
	}
	if opts.format != "" {
		return expandFormat(fi, path, opts.format, false) + "\n"
	}
	if opts.terse {
		return terseFile(fi, path) + "\n"
	}
	return defaultFile(fi, path)
}

func statFS(opts options, path string) error {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return fmtError(path, err)
	}
	output := fsOutput(opts, &fs, path)
	fmt.Fprint(os.Stdout, output)
	return nil
}

func fsOutput(opts options, fs *syscall.Statfs_t, path string) string {
	if opts.printf != "" {
		return expandFsFormat(fs, path, opts.printf, false)
	}
	if opts.format != "" {
		return expandFsFormat(fs, path, opts.format, false) + "\n"
	}
	if opts.terse {
		return terseFS(fs, path) + "\n"
	}
	return defaultFS(fs, path)
}

func fmtError(path string, err error) error {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	if len(msg) > 0 && unicode.IsLower(rune(msg[0])) {
		msg = string(unicode.ToUpper(rune(msg[0]))) + msg[1:]
	}
	return fmt.Errorf("stat: cannot stat '%s': %s", path, msg)
}

func defaultFile(fi *sys.FileInfo, path string) string {
	var b strings.Builder
	rawMode := rawModeFromInfo(fi)
	writeFileLine(&b, fi, path)
	writeSizeLine(&b, fi)
	writeDeviceLine(&b, fi, rawMode)
	writeAccessLine(&b, fi, rawMode)
	writeTimestamps(&b, fi)
	return b.String()
}

func writeFileLine(b *strings.Builder, fi *sys.FileInfo, path string) {
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			fmt.Fprintf(b, "  File: %s\n", path)
		} else {
			fmt.Fprintf(b, "  File: %s -> %s\n", path, target)
		}
	} else {
		fmt.Fprintf(b, "  File: %s\n", path)
	}
}

func writeSizeLine(b *strings.Builder, fi *sys.FileInfo) {
	fmt.Fprintf(b, "  Size: %-10d\tBlocks: %-10d IO Block: %-6d %s\n",
		fi.Size, fi.Blocks, fi.Blksize, fileTypeStrSize(fi.Mode, fi.Size))
}

func writeDeviceLine(b *strings.Builder, fi *sys.FileInfo, rawMode uint16) {
	maj := major(fi.Dev)
	min := minor(fi.Dev)
	if fi.Mode&(os.ModeDevice|os.ModeCharDevice) != 0 {
		rMaj := major(fi.Rdev)
		rMin := minor(fi.Rdev)
		fmt.Fprintf(b, "Device: %d,%d\tInode: %-10d  Links: %-5d Device type: %d,%d\n",
			maj, min, fi.Ino, fi.Nlink, rMaj, rMin)
	} else {
		fmt.Fprintf(b, "Device: %d,%d\tInode: %-10d  Links: %d\n",
			maj, min, fi.Ino, fi.Nlink)
	}
	_ = rawMode
}

func writeAccessLine(b *strings.Builder, fi *sys.FileInfo, rawMode uint16) {
	octal := rawMode & 07777
	perm := permString(fi.Mode)
	uname := lookupUser(fi.Uid)
	gname := lookupGroup(fi.Gid)
	fmt.Fprintf(b, "Access: (%04o/%s)  Uid: (%5d/%8s)   Gid: (%5d/%8s)\n",
		octal, perm, fi.Uid, uname, fi.Gid, gname)
}

func writeTimestamps(b *strings.Builder, fi *sys.FileInfo) {
	birth := birthTime(fi)
	fmt.Fprintf(b, "Access: %s\n", formatTime(fi.AccessTime))
	fmt.Fprintf(b, "Modify: %s\n", formatTime(fi.ModTime))
	fmt.Fprintf(b, "Change: %s\n", formatTime(fi.ChangeTime))
	fmt.Fprintf(b, " Birth: %s\n", formatTime(birth))
}

func terseFile(fi *sys.FileInfo, path string) string {
	rawMode := rawModeFromInfo(fi)
	birth := birthTime(fi)
	return fmt.Sprintf("%s %d %d %x %d %d %x %d %d %x %x %d %d %d %d %d",
		path, fi.Size, fi.Blocks, rawMode,
		fi.Uid, fi.Gid, fi.Dev, fi.Ino, fi.Nlink,
		major(fi.Rdev), minor(fi.Rdev),
		fi.AccessTime.Unix(), fi.ModTime.Unix(),
		fi.ChangeTime.Unix(), birth.Unix(),
		fi.Blksize)
}

func defaultFS(fs *syscall.Statfs_t, path string) string {
	var b strings.Builder
	typeName := int8ToStr(fs.Fstypename[:])
	fsid := fsIDHex(fs.Fsid)
	fmt.Fprintf(&b, "  File: \"%s\"\n", path)
	fmt.Fprintf(&b, "    ID: %s Namelen: %-7s Type: %s\n",
		fsid, "?", typeName)
	fmt.Fprintf(&b, "Block size: %-10d Fundamental block size: %d\n",
		fs.Bsize, fs.Bsize)
	fmt.Fprintf(&b, "Blocks: Total: %-10d Free: %-10d Available: %d\n",
		fs.Blocks, fs.Bfree, fs.Bavail)
	fmt.Fprintf(&b, "Inodes: Total: %-10d Free: %d\n",
		fs.Files, fs.Ffree)
	return b.String()
}

func terseFS(fs *syscall.Statfs_t, path string) string {
	fsid := fsIDHex(fs.Fsid)
	return fmt.Sprintf("%s %s ? %x %d %d %d %d %d %d %d",
		path, fsid, fs.Type,
		fs.Bsize, fs.Bsize,
		fs.Blocks, fs.Bfree, fs.Bavail,
		fs.Files, fs.Ffree)
}

func expandFormat(fi *sys.FileInfo, path, format string, _ bool) string {
	var b strings.Builder
	i := 0
	for i < len(format) {
		if format[i] == '\\' {
			ch, adv := parseEscape(format[i:])
			b.WriteString(ch)
			i += adv
		} else if format[i] == '%' {
			val, adv := parseFileDirective(fi, path, format[i:])
			b.WriteString(val)
			i += adv
		} else {
			b.WriteByte(format[i])
			i++
		}
	}
	return b.String()
}

func expandFsFormat(fs *syscall.Statfs_t, path, format string, _ bool) string {
	var b strings.Builder
	i := 0
	for i < len(format) {
		if format[i] == '\\' {
			ch, adv := parseEscape(format[i:])
			b.WriteString(ch)
			i += adv
		} else if format[i] == '%' {
			val, adv := parseFsDirective(fs, path, format[i:])
			b.WriteString(val)
			i += adv
		} else {
			b.WriteByte(format[i])
			i++
		}
	}
	return b.String()
}

func parseFileDirective(fi *sys.FileInfo, path, s string) (string, int) {
	if len(s) < 2 {
		return "%", 1
	}
	if s[1] == '%' {
		return "%", 2
	}
	flags, width, prec, dir, adv := parseModifiers(s[1:])
	val := fileDirectiveValue(fi, path, dir)
	formatted := applyModifiers(val, flags, width, prec)
	return formatted, 1 + adv
}

func parseFsDirective(fs *syscall.Statfs_t, path, s string) (string, int) {
	if len(s) < 2 {
		return "%", 1
	}
	if s[1] == '%' {
		return "%", 2
	}
	flags, width, prec, dir, adv := parseModifiers(s[1:])
	val := fsDirectiveValue(fs, path, dir)
	formatted := applyModifiers(val, flags, width, prec)
	return formatted, 1 + adv
}

func parseModifiers(s string) (string, int, int, byte, int) {
	i := 0
	var flags string
	for i < len(s) && (s[i] == '-' || s[i] == '+' || s[i] == ' ' || s[i] == '0' || s[i] == '#') {
		flags += string(s[i])
		i++
	}
	width := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		width = width*10 + int(s[i]-'0')
		i++
	}
	prec := -1
	if i < len(s) && s[i] == '.' {
		i++
		prec = 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			prec = prec*10 + int(s[i]-'0')
			i++
		}
	}
	if i >= len(s) {
		return flags, width, prec, 0, i
	}
	return flags, width, prec, s[i], i + 1
}

func applyModifiers(val string, flags string, width int, prec int) string {
	if width == 0 && prec < 0 && flags == "" {
		return val
	}
	fmtStr := "%" + flags
	if width > 0 {
		fmtStr += strconv.Itoa(width)
	}
	if prec >= 0 {
		fmtStr += "." + strconv.Itoa(prec)
	}
	fmtStr += "s"
	return fmt.Sprintf(fmtStr, val)
}

func fileDirectiveValue(fi *sys.FileInfo, path string, dir byte) string {
	rawMode := rawModeFromInfo(fi)
	switch dir {
	case 'a':
		return fmt.Sprintf("%o", rawMode&07777)
	case 'A':
		return permString(fi.Mode)
	case 'b':
		return fmt.Sprintf("%d", fi.Blocks)
	case 'B':
		return "512"
	case 'd':
		return fmt.Sprintf("%d", fi.Dev)
	case 'D':
		return fmt.Sprintf("%x", fi.Dev)
	case 'f':
		return fmt.Sprintf("%x", rawMode)
	case 'F':
		return fileTypeStrSize(fi.Mode, fi.Size)
	case 'g':
		return fmt.Sprintf("%d", fi.Gid)
	case 'G':
		return lookupGroup(fi.Gid)
	case 'h':
		return fmt.Sprintf("%d", fi.Nlink)
	case 'i':
		return fmt.Sprintf("%d", fi.Ino)
	case 'm':
		return mountPoint(path)
	case 'n':
		return path
	case 'N':
		return quotedName(fi, path)
	case 'o':
		return fmt.Sprintf("%d", fi.Blksize)
	case 's':
		return fmt.Sprintf("%d", fi.Size)
	case 't':
		return fmt.Sprintf("%x", major(fi.Rdev))
	case 'T':
		return fmt.Sprintf("%x", minor(fi.Rdev))
	case 'u':
		return fmt.Sprintf("%d", fi.Uid)
	case 'U':
		return lookupUser(fi.Uid)
	default:
		return fileTimeDirective(fi, dir)
	}
}

func fileTimeDirective(fi *sys.FileInfo, dir byte) string {
	birth := birthTime(fi)
	switch dir {
	case 'w':
		return formatTimeBirth(birth)
	case 'W':
		return epochBirth(birth)
	case 'x':
		return formatTime(fi.AccessTime)
	case 'X':
		return fmt.Sprintf("%d", fi.AccessTime.Unix())
	case 'y':
		return formatTime(fi.ModTime)
	case 'Y':
		return fmt.Sprintf("%d", fi.ModTime.Unix())
	case 'z':
		return formatTime(fi.ChangeTime)
	case 'Z':
		return fmt.Sprintf("%d", fi.ChangeTime.Unix())
	default:
		return "%" + string(dir)
	}
}

func fsDirectiveValue(fs *syscall.Statfs_t, path string, dir byte) string {
	switch dir {
	case 'a':
		return fmt.Sprintf("%d", fs.Bavail)
	case 'b':
		return fmt.Sprintf("%d", fs.Blocks)
	case 'c':
		return fmt.Sprintf("%d", fs.Files)
	case 'd':
		return fmt.Sprintf("%d", fs.Ffree)
	case 'f':
		return fmt.Sprintf("%d", fs.Bfree)
	case 'i':
		return fsIDHex(fs.Fsid)
	case 'l':
		return "?"
	case 'n':
		return path
	case 's':
		return fmt.Sprintf("%d", fs.Bsize)
	case 'S':
		return fmt.Sprintf("%d", fs.Bsize)
	case 't':
		return fmt.Sprintf("%x", fs.Type)
	case 'T':
		return int8ToStr(fs.Fstypename[:])
	default:
		return "%" + string(dir)
	}
}

func parseEscape(s string) (string, int) {
	if len(s) < 2 {
		return "\\", 1
	}
	switch s[1] {
	case 'n':
		return "\n", 2
	case 't':
		return "\t", 2
	case 'r':
		return "\r", 2
	case '\\':
		return "\\", 2
	case 'a':
		return "\a", 2
	case 'b':
		return "\b", 2
	case 'f':
		return "\f", 2
	case 'v':
		return "\v", 2
	case '0':
		return parseOctalEscape(s)
	case 'x':
		return parseHexEscape(s)
	default:
		return string(s[:2]), 2
	}
}

func parseOctalEscape(s string) (string, int) {
	n := 2
	val := 0
	for n < len(s) && n < 5 && s[n] >= '0' && s[n] <= '7' {
		val = val*8 + int(s[n]-'0')
		n++
	}
	if n == 2 {
		return "\x00", 2
	}
	return string(rune(val)), n
}

func parseHexEscape(s string) (string, int) {
	n := 2
	val := 0
	for n < len(s) && n < 4 {
		c := s[n]
		if c >= '0' && c <= '9' {
			val = val*16 + int(c-'0')
		} else if c >= 'a' && c <= 'f' {
			val = val*16 + int(c-'a') + 10
		} else if c >= 'A' && c <= 'F' {
			val = val*16 + int(c-'A') + 10
		} else {
			break
		}
		n++
	}
	if n == 2 {
		return "\\x", 2
	}
	return string(rune(val)), n
}

func fileTypeStrSize(mode os.FileMode, size int64) string {
	if mode.IsRegular() && size == 0 {
		return "regular empty file"
	}
	return fileTypeStr(mode)
}

func fileTypeStr(mode os.FileMode) string {
	switch {
	case mode.IsRegular():
		return "regular file"
	case mode.IsDir():
		return "directory"
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return "character special file"
	case mode&os.ModeDevice != 0:
		return "block special file"
	case mode&os.ModeNamedPipe != 0:
		return "fifo"
	case mode&os.ModeSocket != 0:
		return "socket"
	default:
		return "weird file"
	}
}

func permString(mode os.FileMode) string {
	var buf [10]byte
	buf[0] = fileTypeChar(mode)
	const rwx = "rwx"
	perm := mode.Perm()
	for i := range 9 {
		if perm&(1<<uint(8-i)) != 0 {
			buf[1+i] = rwx[i%3]
		} else {
			buf[1+i] = '-'
		}
	}
	applySpecialBits(mode, &buf)
	return string(buf[:])
}

func fileTypeChar(mode os.FileMode) byte {
	switch {
	case mode.IsRegular():
		return '-'
	case mode.IsDir():
		return 'd'
	case mode&os.ModeSymlink != 0:
		return 'l'
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return 'c'
	case mode&os.ModeDevice != 0:
		return 'b'
	case mode&os.ModeNamedPipe != 0:
		return 'p'
	case mode&os.ModeSocket != 0:
		return 's'
	default:
		return '?'
	}
}

func applySpecialBits(mode os.FileMode, buf *[10]byte) {
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

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000000000 -0700")
}

func formatTimeBirth(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return formatTime(t)
}

func epochBirth(t time.Time) string {
	if t.IsZero() {
		return "0"
	}
	return fmt.Sprintf("%d", t.Unix())
}

func birthTime(fi *sys.FileInfo) time.Time {
	if fi.Info == nil {
		return time.Time{}
	}
	stat, ok := fi.Info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}
	}
	return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}

func rawModeFromInfo(fi *sys.FileInfo) uint16 {
	if fi.Info == nil {
		return 0
	}
	stat, ok := fi.Info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return stat.Mode
}

func lookupUser(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.Itoa(int(uid))
	}
	return u.Username
}

func lookupGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.Itoa(int(gid))
	}
	return g.Name
}

func major(dev uint64) uint64 {
	return (dev >> 24) & 0xff
}

func minor(dev uint64) uint64 {
	return dev & 0xffffff
}

func quoteFile(name string) string {
	return "'" + name + "'"
}

func quotedName(fi *sys.FileInfo, path string) string {
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return quoteFile(path)
		}
		return quoteFile(path) + " -> " + quoteFile(target)
	}
	return quoteFile(path)
}

func mountPoint(path string) string {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return "?"
	}
	mp := int8ToStr(fs.Mntonname[:])
	if mp == "/System/Volumes/Data" {
		return "/"
	}
	return mp
}

func int8ToStr(arr []int8) string {
	buf := make([]byte, 0, len(arr))
	for _, v := range arr {
		if v == 0 {
			break
		}
		buf = append(buf, byte(v))
	}
	return string(buf)
}

func fsIDHex(fsid syscall.Fsid) string {
	return fmt.Sprintf("%x", uint64(uint32(fsid.Val[0]))<<32|uint64(uint32(fsid.Val[1])))
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var files []string
	endFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endFlags || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endFlags = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			adv, err := parseLongFlag(arg, args[i+1:], &opts)
			if err != nil {
				return options{}, nil, err
			}
			i += adv
			continue
		}
		adv, err := parseShortFlags(arg[1:], args[i+1:], &opts)
		if err != nil {
			return options{}, nil, err
		}
		i += adv
	}
	return opts, files, nil
}

func parseLongFlag(flag string, rest []string, opts *options) (int, error) {
	if strings.HasPrefix(flag, "--format=") {
		opts.format = flag[len("--format="):]
		return 0, nil
	}
	if strings.HasPrefix(flag, "--printf=") {
		opts.printf = flag[len("--printf="):]
		return 0, nil
	}
	switch flag {
	case "--format":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '%s' requires an argument", flag)
		}
		opts.format = rest[0]
		return 1, nil
	case "--printf":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '%s' requires an argument", flag)
		}
		opts.printf = rest[0]
		return 1, nil
	case "--dereference":
		opts.dereference = true
	case "--file-system":
		opts.fileSystem = true
	case "--terse":
		opts.terse = true
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return 0, nil
}

func parseShortFlags(flags string, rest []string, opts *options) (int, error) {
	for i, ch := range flags {
		switch ch {
		case 'L':
			opts.dereference = true
		case 'f':
			opts.fileSystem = true
		case 't':
			opts.terse = true
		case 'c':
			remaining := flags[i+1:]
			if remaining != "" {
				opts.format = remaining
				return 0, nil
			}
			if len(rest) == 0 {
				return 0, fmt.Errorf("option requires an argument -- 'c'")
			}
			opts.format = rest[0]
			return 1, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}
