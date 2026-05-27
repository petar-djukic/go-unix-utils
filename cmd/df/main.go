// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd106-df R1.1-R1.4.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"golang.org/x/sys/unix"
)

type fsEntry struct {
	filesystem string
	blocks1K   int64
	used       int64
	available  int64
	usePercent string
	mountedOn  string
}

var headers = [6]string{
	"Filesystem", "1K-blocks", "Used", "Available", "Use%", "Mounted on",
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return listAll()
	}
	return listPaths(args)
}

func listAll() int {
	stats, err := getAllMounts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	entries := filterAndDedup(stats)
	printTable(entries)
	return 0
}

func listPaths(paths []string) int {
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
		entries = append(entries, entry)
	}
	if len(entries) > 0 {
		printTable(entries)
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

func filterAndDedup(stats []unix.Statfs_t) []fsEntry {
	seen := make(map[int32]bool)
	entries := make([]fsEntry, 0, len(stats))
	for _, s := range stats {
		if isDummyType(cstring(s.Fstypename[:])) {
			continue
		}
		if s.Blocks == 0 {
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
		entries = append(entries, buildEntry(s))
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
	total := int64(stat.Blocks * bsize / 1024)
	free := int64(stat.Bfree * bsize / 1024)
	avail := int64(stat.Bavail * bsize / 1024)
	return fsEntry{
		filesystem: cstring(stat.Mntfromname[:]),
		blocks1K:   total,
		used:       total - free,
		available:  avail,
		usePercent: computeUsePercent(total, avail),
		mountedOn:  cstring(stat.Mntonname[:]),
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

func printTable(entries []fsEntry) {
	w := computeWidths(entries)
	printLine(headers, w)
	for _, e := range entries {
		printLine(entryStrings(e), w)
	}
}

func entryStrings(e fsEntry) [6]string {
	return [6]string{
		e.filesystem,
		strconv.FormatInt(e.blocks1K, 10),
		strconv.FormatInt(e.used, 10),
		strconv.FormatInt(e.available, 10),
		e.usePercent,
		e.mountedOn,
	}
}

func computeWidths(entries []fsEntry) [6]int {
	w := [6]int{
		len(headers[0]), len(headers[1]), len(headers[2]),
		len(headers[3]), len(headers[4]), len(headers[5]),
	}
	for _, e := range entries {
		cols := entryStrings(e)
		for i := range w {
			if len(cols[i]) > w[i] {
				w[i] = len(cols[i])
			}
		}
	}
	return w
}

func printLine(cols [6]string, w [6]int) {
	fmt.Printf("%s %s %s %s %s %s\n",
		format.PadRight(cols[0], w[0]),
		format.PadLeft(cols[1], w[1]),
		format.PadLeft(cols[2], w[2]),
		format.PadLeft(cols[3], w[3]),
		format.PadLeft(cols[4], w[4]),
		cols[5],
	)
}

func cstring(b []byte) string {
	for i, v := range b {
		if v == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
