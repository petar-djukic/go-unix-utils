// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd106-df R1.1-R1.5, R2.1-R2.3, R3.1-R3.7.
package main

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"golang.org/x/sys/unix"
)

type sizeMode int

const (
	modeDefault sizeMode = iota
	modeHuman
	modeSI
)

type options struct {
	sizeMode     sizeMode
	showType     bool
	showInodes   bool
	showAll      bool
	localOnly    bool
	includeTypes []string
	excludeTypes []string
	outputFields []string
	outputMode   bool
}

type fsEntry struct {
	filesystem      string
	fsType          string
	blocks1K        int64
	used            int64
	available       int64
	sizeBytes       int64
	usedBytes       int64
	availBytes      int64
	usePercent      string
	inodes          int64
	inodesUsed      int64
	inodesFree      int64
	inodeUsePercent string
	mountedOn       string
	file            string
}

var humanSuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opts, files := parseArgs(args)
	if msg := validateOpts(opts); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		return 1
	}
	if len(files) == 0 {
		return listAll(opts)
	}
	return listPaths(files, opts)
}

func validateOpts(opts options) string {
	if opts.outputMode && opts.showInodes {
		return "df: options -i and --output are mutually exclusive\n" +
			"Try 'df --help' for more information."
	}
	if opts.outputMode && opts.showType {
		return "df: options -T and --output are mutually exclusive\n" +
			"Try 'df --help' for more information."
	}
	for _, t := range opts.includeTypes {
		if slices.Contains(opts.excludeTypes, t) {
			return fmt.Sprintf("df: file system type '%s' both selected and excluded", t)
		}
	}
	return ""
}

func parseArgs(args []string) (options, []string) {
	var opts options
	var files []string
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		consumed, skip := parseFlag(&opts, arg, args, i)
		if consumed {
			i += skip
		} else {
			files = append(files, arg)
		}
	}
	return opts, files
}

func parseFlag(opts *options, arg string, args []string, i int) (bool, int) {
	switch {
	case arg == "-h" || arg == "--human-readable":
		opts.sizeMode = modeHuman
	case arg == "-H" || arg == "--si":
		opts.sizeMode = modeSI
	case arg == "-T" || arg == "--print-type":
		opts.showType = true
	case arg == "-i" || arg == "--inodes":
		opts.showInodes = true
	case arg == "-a" || arg == "--all":
		opts.showAll = true
	case arg == "-l" || arg == "--local":
		opts.localOnly = true
	case arg == "-t" && i+1 < len(args):
		opts.includeTypes = append(opts.includeTypes, args[i+1])
		return true, 1
	case strings.HasPrefix(arg, "--type="):
		opts.includeTypes = append(opts.includeTypes, arg[7:])
	case arg == "-x" && i+1 < len(args):
		opts.excludeTypes = append(opts.excludeTypes, args[i+1])
		return true, 1
	case strings.HasPrefix(arg, "--exclude-type="):
		opts.excludeTypes = append(opts.excludeTypes, arg[15:])
	case arg == "--output":
		opts.outputMode = true
		opts.outputFields = allOutputFields()
	case strings.HasPrefix(arg, "--output="):
		opts.outputMode = true
		opts.outputFields = strings.Split(arg[9:], ",")
	default:
		return false, 0
	}
	return true, 0
}

func listAll(opts options) int {
	stats, err := getAllMounts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	entries := filterAndDedup(stats, opts)
	printTable(entries, opts)
	return 0
}

func listPaths(paths []string, opts options) int {
	stats, err := getAllMounts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	devMap := buildDevMap(stats)
	exitCode := 0
	entries := make([]fsEntry, 0, len(paths))
	for _, path := range paths {
		entry, err := entryForPath(path, devMap)
		if err != nil {
			fmt.Fprintf(os.Stderr, "df: %s: %s\n", path, capitalizeErr(err))
			exitCode = 1
			continue
		}
		if !matchesTypeFilter(entry.fsType, opts) {
			continue
		}
		entry.file = path
		entries = append(entries, entry)
	}
	if len(entries) > 0 {
		printTable(entries, opts)
	}
	return exitCode
}

func capitalizeErr(err error) string {
	s := err.Error()
	if len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

func getAllMounts() ([]unix.Statfs_t, error) {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	buf := make([]unix.Statfs_t, n)
	n, err = unix.Getfsstat(buf, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func isDummyType(t string) bool {
	return t == "devfs" || t == "autofs"
}

func matchesTypeFilter(fsType string, opts options) bool {
	if len(opts.includeTypes) > 0 && !slices.Contains(opts.includeTypes, fsType) {
		return false
	}
	return !slices.Contains(opts.excludeTypes, fsType)
}

func filterAndDedup(stats []unix.Statfs_t, opts options) []fsEntry {
	seen := make(map[int32]bool)
	entries := make([]fsEntry, 0, len(stats))
	for _, s := range stats {
		fsType := cstring(s.Fstypename[:])
		if !opts.showAll {
			if isDummyType(fsType) {
				continue
			}
			if s.Blocks == 0 {
				continue
			}
		}
		if opts.localOnly && s.Flags&unix.MNT_LOCAL == 0 {
			continue
		}
		if !matchesTypeFilter(fsType, opts) {
			continue
		}
		dev, ok := mountDev(cstring(s.Mntonname[:]))
		if !ok {
			continue
		}
		if seen[dev] {
			continue
		}
		seen[dev] = true
		entry := buildEntry(s)
		entry.file = "-"
		entries = append(entries, entry)
	}
	return entries
}

func buildDevMap(stats []unix.Statfs_t) map[int32]string {
	m := make(map[int32]string, len(stats))
	for _, s := range stats {
		mount := cstring(s.Mntonname[:])
		dev, ok := mountDev(mount)
		if !ok {
			continue
		}
		if _, exists := m[dev]; !exists {
			m[dev] = mount
		}
	}
	return m
}

func entryForPath(path string, devMap map[int32]string) (fsEntry, error) {
	var info syscall.Stat_t
	if err := syscall.Stat(path, &info); err != nil {
		return fsEntry{}, err
	}
	target := path
	if mount, ok := devMap[info.Dev]; ok {
		target = mount
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(target, &stat); err != nil {
		return fsEntry{}, err
	}
	return buildEntry(stat), nil
}

func mountDev(mount string) (int32, bool) {
	var info syscall.Stat_t
	if err := syscall.Stat(mount, &info); err != nil {
		return 0, false
	}
	return info.Dev, true
}

func buildEntry(stat unix.Statfs_t) fsEntry {
	bsize := uint64(stat.Bsize)
	totalBytes := int64(stat.Blocks * bsize)
	freeBytes := int64(stat.Bfree * bsize)
	availBytes := int64(stat.Bavail * bsize)
	total := totalBytes / 1024
	free := freeBytes / 1024
	avail := availBytes / 1024
	inodes := int64(stat.Files)
	inodesFree := int64(stat.Ffree)
	return fsEntry{
		filesystem:      cstring(stat.Mntfromname[:]),
		fsType:          cstring(stat.Fstypename[:]),
		blocks1K:        total,
		used:            total - free,
		available:       avail,
		sizeBytes:       totalBytes,
		usedBytes:       totalBytes - freeBytes,
		availBytes:      availBytes,
		usePercent:      computeUsePercent(total, avail),
		inodes:          inodes,
		inodesUsed:      inodes - inodesFree,
		inodesFree:      inodesFree,
		inodeUsePercent: computeUsePercent(inodes, inodesFree),
		mountedOn:       cstring(stat.Mntonname[:]),
	}
}

func computeUsePercent(total, avail int64) string {
	if total == 0 {
		return "-"
	}
	used := total - avail
	if used <= 0 {
		return "0%"
	}
	pct := int64(math.Ceil(float64(used) * 100 / float64(total)))
	return strconv.FormatInt(pct, 10) + "%"
}

func humanSize(bytes int64, base float64) string {
	if bytes == 0 {
		return "0"
	}
	val := float64(bytes)
	idx := 0
	for idx < len(humanSuffixes)-1 && val >= base {
		val /= base
		idx++
	}
	return formatHuman(val, humanSuffixes[idx])
}

func formatHuman(val float64, suffix string) string {
	if suffix == "" {
		return strconv.FormatInt(int64(math.Ceil(val)), 10)
	}
	ceiled := math.Ceil(val*10) / 10
	if ceiled < 10 {
		return fmt.Sprintf("%.1f%s", ceiled, suffix)
	}
	return fmt.Sprintf("%d%s", int64(math.Ceil(val)), suffix)
}

func formatSize(blocks1K, sizeBytes int64, mode sizeMode) string {
	switch mode {
	case modeHuman:
		return humanSize(sizeBytes, 1024)
	case modeSI:
		return humanSize(sizeBytes, 1000)
	default:
		return strconv.FormatInt(blocks1K, 10)
	}
}

func formatInodeField(count int64, mode sizeMode) string {
	switch mode {
	case modeHuman:
		return humanSize(count, 1024)
	case modeSI:
		return humanSize(count, 1000)
	default:
		return strconv.FormatInt(count, 10)
	}
}

func getHeaders(opts options) []string {
	var h []string
	h = append(h, "Filesystem")
	if opts.showType {
		h = append(h, "Type")
	}
	if opts.showInodes {
		h = append(h, "Inodes", "IUsed", "IFree", "IUse%")
	} else if opts.sizeMode != modeDefault {
		h = append(h, "Size", "Used", "Avail", "Use%")
	} else {
		h = append(h, "1K-blocks", "Used", "Available", "Use%")
	}
	h = append(h, "Mounted on")
	return h
}

func getAlignment(opts options) []bool {
	var a []bool
	a = append(a, false)
	if opts.showType {
		a = append(a, false)
	}
	a = append(a, true, true, true, true)
	a = append(a, false)
	return a
}

func entryStrings(e fsEntry, opts options) []string {
	var cols []string
	cols = append(cols, e.filesystem)
	if opts.showType {
		cols = append(cols, e.fsType)
	}
	if opts.showInodes {
		cols = append(cols,
			strconv.FormatInt(e.inodes, 10),
			strconv.FormatInt(e.inodesUsed, 10),
			strconv.FormatInt(e.inodesFree, 10),
			e.inodeUsePercent,
		)
	} else {
		cols = append(cols,
			formatSize(e.blocks1K, e.sizeBytes, opts.sizeMode),
			formatSize(e.used, e.usedBytes, opts.sizeMode),
			formatSize(e.available, e.availBytes, opts.sizeMode),
			e.usePercent,
		)
	}
	cols = append(cols, e.mountedOn)
	return cols
}

func computeWidths(entries []fsEntry, opts options) []int {
	h := getHeaders(opts)
	w := make([]int, len(h))
	for i, s := range h {
		w[i] = len(s)
	}
	for _, e := range entries {
		cols := entryStrings(e, opts)
		for i := range w {
			if len(cols[i]) > w[i] {
				w[i] = len(cols[i])
			}
		}
	}
	start := 1
	if opts.showType {
		start = 2
	}
	for i := start; i < start+3 && i < len(w); i++ {
		if w[i] < 5 {
			w[i] = 5
		}
	}
	return w
}

func printTable(entries []fsEntry, opts options) {
	if opts.outputMode {
		printOutputTable(entries, opts)
		return
	}
	w := computeWidths(entries, opts)
	align := getAlignment(opts)
	printLine(getHeaders(opts), w, align)
	for _, e := range entries {
		printLine(entryStrings(e, opts), w, align)
	}
}

func allOutputFields() []string {
	return []string{
		"source", "fstype", "itotal", "iused", "iavail", "ipcent",
		"size", "used", "avail", "pcent", "file", "target",
	}
}

func outputHeader(field string, mode sizeMode) string {
	switch field {
	case "source":
		return "Filesystem"
	case "fstype":
		return "Type"
	case "itotal":
		return "Inodes"
	case "iused":
		return "IUsed"
	case "iavail":
		return "IFree"
	case "ipcent":
		return "IUse%"
	case "size":
		if mode != modeDefault {
			return "Size"
		}
		return "1K-blocks"
	case "used":
		return "Used"
	case "avail":
		return "Avail"
	case "pcent":
		return "Use%"
	case "file":
		return "File"
	case "target":
		return "Mounted on"
	}
	return field
}

func outputRightAlign(field string) bool {
	switch field {
	case "source", "fstype", "file", "target":
		return false
	default:
		return true
	}
}

func outputValue(e fsEntry, field string, opts options) string {
	switch field {
	case "source":
		return e.filesystem
	case "fstype":
		return e.fsType
	case "itotal":
		return formatInodeField(e.inodes, opts.sizeMode)
	case "iused":
		return formatInodeField(e.inodesUsed, opts.sizeMode)
	case "iavail":
		return formatInodeField(e.inodesFree, opts.sizeMode)
	case "ipcent":
		return e.inodeUsePercent
	case "size":
		return formatSize(e.blocks1K, e.sizeBytes, opts.sizeMode)
	case "used":
		return formatSize(e.used, e.usedBytes, opts.sizeMode)
	case "avail":
		return formatSize(e.available, e.availBytes, opts.sizeMode)
	case "pcent":
		return e.usePercent
	case "file":
		return e.file
	case "target":
		return e.mountedOn
	}
	return ""
}

func getOutputHeaders(fields []string, mode sizeMode) []string {
	h := make([]string, len(fields))
	for i, f := range fields {
		h[i] = outputHeader(f, mode)
	}
	return h
}

func getOutputAlignment(fields []string) []bool {
	a := make([]bool, len(fields))
	for i, f := range fields {
		a[i] = outputRightAlign(f)
	}
	return a
}

func getOutputValues(e fsEntry, fields []string, opts options) []string {
	vals := make([]string, len(fields))
	for i, f := range fields {
		vals[i] = outputValue(e, f, opts)
	}
	return vals
}

func computeRowWidths(headers []string, rows [][]string) []int {
	w := make([]int, len(headers))
	for i, h := range headers {
		w[i] = len(h)
	}
	for _, row := range rows {
		for i, col := range row {
			if len(col) > w[i] {
				w[i] = len(col)
			}
		}
	}
	return w
}

func printOutputTable(entries []fsEntry, opts options) {
	fields := opts.outputFields
	headers := getOutputHeaders(fields, opts.sizeMode)
	align := getOutputAlignment(fields)
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = getOutputValues(e, fields, opts)
	}
	w := computeRowWidths(headers, rows)
	printLine(headers, w, align)
	for _, row := range rows {
		printLine(row, w, align)
	}
}

func printLine(cols []string, w []int, rightAlign []bool) {
	parts := make([]string, len(cols))
	for i, col := range cols {
		if i == len(cols)-1 {
			parts[i] = col
		} else if rightAlign[i] {
			parts[i] = format.PadLeft(col, w[i])
		} else {
			parts[i] = format.PadRight(col, w[i])
		}
	}
	fmt.Println(strings.Join(parts, " "))
}

func cstring(b []byte) string {
	for i, v := range b {
		if v == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
