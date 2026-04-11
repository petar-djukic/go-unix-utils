// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Filesystem stat mode for cmd/stat.
// Implements srd082 R5.1, R6.1 (filesystem status and format directives).
package main

import (
	"fmt"
	"os"
	"strings"
)

// fsInfo holds filesystem status data from statfs(2).
type fsInfo struct {
	typeName string
	typeHex  uint32
	bsize    int64
	frsize   int64
	blocks   uint64
	bfree    uint64
	bavail   uint64
	files    uint64
	ffree    uint64
	fsidVal  [2]int32
	namelen  int64
}

// fsTerseFormat is the format string for filesystem terse output.
const fsTerseFormat = "%n %i %l %t %s %S %b %f %a %c %d\n"

// processFileFsys performs filesystem stat and prints output.
// R5.1: -f displays filesystem status instead of file status.
func processFileFsys(path string, opts options) error {
	fs, err := statFilesystem(path)
	if err != nil {
		reportFsError(path, err)
		return err
	}
	switch {
	case opts.format != "":
		fmt.Println(expandFsFormat(opts.format, fs, path))
	case opts.terse:
		fmt.Print(expandFsFormat(fsTerseFormat, fs, path))
	default:
		fmt.Print(expandFsFormat(defaultFsFormat(), fs, path))
	}
	return nil
}

// reportFsError prints a filesystem stat error to stderr.
func reportFsError(path string, err error) {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	fmt.Fprintf(os.Stderr,
		"%s: cannot read file system information for '%s': %s\n",
		programName, path, msg)
}

// defaultFsFormat returns the default format for filesystem stat output.
func defaultFsFormat() string {
	return "  File: \"%n\"\n" +
		"    ID: %-8i Namelen: %-7l Type: %T\n" +
		"Block size: %-10s Fundamental block size: %S\n" +
		"Blocks: Total: %-10b Free: %-10f Available: %a\n" +
		"Inodes: Total: %-10c Free: %d\n"
}

// expandFsFormat expands filesystem format directives in the format string.
// R6.1: filesystem directives differ from file directives.
func expandFsFormat(format string, fs *fsInfo, path string) string {
	return expandFormatWith(format, func(spec formatSpec) string {
		return applyFsDirective(spec, fs, path)
	})
}

// applyFsDirective formats a single filesystem directive with width/flags.
func applyFsDirective(
	spec formatSpec, fs *fsInfo, path string,
) string {
	if spec.directive == 'i' {
		return formatFsidSpec(spec, fs.fsidVal)
	}
	// R6.1: %l prints "?" when namelen is unavailable (macOS).
	if spec.directive == 'l' && fs.namelen == 0 {
		prefix := "%" + spec.flags + spec.width + spec.precision
		return fmt.Sprintf(prefix+"s", "?")
	}
	d := spec.directive
	prefix := "%" + spec.flags + spec.width + spec.precision
	if isFsNumericDirective(d) {
		val := fsNumericValue(d, fs)
		if fsDirectiveBase(d) == 16 {
			return fmt.Sprintf(prefix+"x", val)
		}
		return fmt.Sprintf(prefix+"d", val)
	}
	if isFsStringDirective(d) {
		return fmt.Sprintf(prefix+"s", fsStringValue(d, fs, path))
	}
	return "%" + string(d)
}

// formatFsidSpec formats the filesystem ID as a combined 64-bit hex value.
// The two int32 values are combined in little-endian order: val[0] is low
// 32 bits, val[1] is high 32 bits.
func formatFsidSpec(spec formatSpec, val [2]int32) string {
	combined := (uint64(uint32(val[0])) << 32) |
		uint64(uint32(val[1]))
	prefix := "%" + spec.flags + spec.width + spec.precision
	return fmt.Sprintf(prefix+"x", combined)
}

// isFsNumericDirective returns true for filesystem numeric directives.
func isFsNumericDirective(d byte) bool {
	switch d {
	case 'a', 'b', 'c', 'd', 'f', 'l', 's', 'S', 't':
		return true
	}
	return false
}

// isFsStringDirective returns true for filesystem string directives.
func isFsStringDirective(d byte) bool {
	return d == 'n' || d == 'T'
}

// fsDirectiveBase returns the numeric base for a filesystem directive.
func fsDirectiveBase(d byte) int {
	if d == 't' {
		return 16
	}
	return 10
}

// fsNumericValue returns the numeric value for a filesystem directive.
func fsNumericValue(d byte, fs *fsInfo) uint64 {
	switch d {
	case 'a':
		return fs.bavail
	case 'b':
		return fs.blocks
	case 'c':
		return fs.files
	case 'd':
		return fs.ffree
	case 'f':
		return fs.bfree
	case 'l':
		return uint64(fs.namelen)
	case 's':
		return uint64(fs.bsize)
	case 'S':
		return uint64(fs.frsize)
	case 't':
		return uint64(fs.typeHex)
	default:
		return 0
	}
}

// fsStringValue returns the string value for a filesystem directive.
func fsStringValue(d byte, fs *fsInfo, path string) string {
	switch d {
	case 'n':
		return path
	case 'T':
		return fs.typeName
	default:
		return ""
	}
}

// int8ToString converts a null-terminated int8 array to a Go string.
func int8ToString(arr []int8) string {
	var buf strings.Builder
	for _, b := range arr {
		if b == 0 {
			break
		}
		buf.WriteByte(byte(b))
	}
	return buf.String()
}
