// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd072-od (contract): cmd/od package stubs with flag definitions
// and main function that exits 0.
package main

import (
	"flag"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// Flag variables matching prd072-od flag specifications.

// R1.2: -t TYPE type specifier (may be given multiple times).
type typeSpecs []string

func (t *typeSpecs) String() string { return "" }

// Set appends a type specifier value.
func (t *typeSpecs) Set(val string) error {
	*t = append(*t, val)
	return nil
}

var formatTypes typeSpecs

// R2.1: -A RADIX address radix (d, o, x, n).
var addressRadix string

// R2.2: -j BYTES skip bytes before formatting.
var skipBytes string

// R2.3: -N BYTES read at most BYTES input bytes.
var readBytes string

// R3.1: -w N output width in bytes per line (default 16).
var outputWidth int

// R3.4: -v output all data, disabling duplicate suppression.
var outputDuplicates bool

// R3.2: Traditional short option flags.
var (
	traditionalB bool // -b: octal bytes (-t o1)
	traditionalC bool // -c: C-style chars (-t c)
	traditionalD bool // -d: unsigned decimal (-t u2)
	traditionalO bool // -o: octal words (-t o2)
	traditionalS bool // -s: signed decimal (-t d2)
	traditionalX bool // -x: hexadecimal (-t x2)
)

func init() {
	flag.Var(&formatTypes, "t", "output format TYPE")
	flag.StringVar(&addressRadix, "A", "o", "address radix: d, o, x, or n")
	flag.StringVar(&skipBytes, "j", "", "skip BYTES input bytes")
	flag.StringVar(&readBytes, "N", "", "read at most BYTES input bytes")
	flag.IntVar(&outputWidth, "w", 16, "output width in bytes per line")
	flag.BoolVar(&outputDuplicates, "v", false, "output duplicates")
	flag.BoolVar(&traditionalB, "b", false, "octal bytes (equivalent to -t o1)")
	flag.BoolVar(&traditionalC, "c", false, "C-style characters (equivalent to -t c)")
	flag.BoolVar(&traditionalD, "d", false, "unsigned decimal (equivalent to -t u2)")
	flag.BoolVar(&traditionalO, "o", false, "octal words (equivalent to -t o2)")
	flag.BoolVar(&traditionalS, "s", false, "signed decimal (equivalent to -t d2)")
	flag.BoolVar(&traditionalX, "x", false, "hexadecimal (equivalent to -t x2)")
}

func main() {
	sys.InstallSIGPIPEHandler()
	flag.Parse()
	// Stub: argument handling reads flag values but does not process them.
	_ = flag.Args()
	os.Exit(0)
}
