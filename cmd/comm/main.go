// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/comm — compare two sorted files line by line.
// Implements prd029-comm R1-R4.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const version = "comm (go-unix-utils) 0.1"

type orderMode int

const (
	orderDefault orderMode = iota
	orderCheck
	orderNoCheck
)

type config struct {
	suppress [3]bool
	delim    string
	order    orderMode
	total    bool
}

// prefix returns the delimiter prefix for a column, adjusted for suppressed columns.
func (c *config) prefix(col int) string {
	n := 0
	for i := range col {
		if !c.suppress[i] {
			n++
		}
	}
	return strings.Repeat(c.delim, n)
}

func preprocessArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if len(a) > 2 && a[0] == '-' && a[1] != '-' && allDigitFlags(a[1:]) {
			for _, c := range a[1:] {
				out = append(out, "-"+string(c))
			}
			continue
		}
		out = append(out, a)
	}
	return out
}

func allDigitFlags(s string) bool {
	for _, c := range s {
		if c != '1' && c != '2' && c != '3' {
			return false
		}
	}
	return true
}

func main() {
	sys.InstallSIGPIPEHandler()
	var sup1, sup2, sup3 bool
	var checkOrder, noCheckOrder, showTotal, showVersion bool
	var delim string
	fs := flag.NewFlagSet("comm", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.BoolVar(&sup1, "1", false, "")
	fs.BoolVar(&sup2, "2", false, "")
	fs.BoolVar(&sup3, "3", false, "")
	fs.BoolVar(&checkOrder, "check-order", false, "")
	fs.BoolVar(&noCheckOrder, "nocheck-order", false, "")
	fs.StringVar(&delim, "output-delimiter", "\t", "")
	fs.BoolVar(&showTotal, "total", false, "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: comm [OPTION]... FILE1 FILE2\n"+
			"Compare sorted files FILE1 and FILE2 line by line.\n\n"+
			"  -1              suppress column 1 (lines unique to FILE1)\n"+
			"  -2              suppress column 2 (lines unique to FILE2)\n"+
			"  -3              suppress column 3 (lines that appear in both files)\n"+
			"  --check-order   check that the input is correctly sorted\n"+
			"  --nocheck-order do not check that the input is correctly sorted\n"+
			"  --output-delimiter=STR  separate columns with STR\n"+
			"  --total         output a summary\n"+
			"      --help      display this help and exit\n"+
			"      --version   output version information and exit\n")
	}
	if err := fs.Parse(preprocessArgs(os.Args[1:])); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}
	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}
	args := fs.Args()
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "comm: missing operand\nTry 'comm --help' for more information.\n")
		os.Exit(1)
	}
	cfg := config{suppress: [3]bool{sup1, sup2, sup3}, delim: delim, total: showTotal}
	if noCheckOrder {
		cfg.order = orderNoCheck
	} else if checkOrder {
		cfg.order = orderCheck
	}
	f1, err := openFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: %v\n", err)
		os.Exit(1)
	}
	defer f1.Close()
	f2, err := openFile(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: %v\n", err)
		os.Exit(1)
	}
	defer f2.Close()
	w := bufio.NewWriter(os.Stdout)
	exitCode := run(cfg, f1, f2, w)
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "comm: write error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func openFile(name string) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(name)
}

func run(cfg config, r1, r2 io.Reader, w *bufio.Writer) int {
	s1, s2 := bufio.NewScanner(r1), bufio.NewScanner(r2)
	have1, have2 := s1.Scan(), s2.Scan()
	var l1, l2, prev1, prev2 string
	hasPrev1, hasPrev2 := false, false
	if have1 {
		l1 = s1.Text()
	}
	if have2 {
		l2 = s2.Text()
	}
	var warned [2]bool
	var counts [3]int

	checkOrd := func(idx int, prev, cur string) bool {
		if cfg.order == orderNoCheck || warned[idx] {
			return true
		}
		if prev > cur {
			fmt.Fprintf(os.Stderr, "comm: file %d is not in sorted order\n", idx+1)
			if cfg.order == orderCheck {
				return false
			}
			warned[idx] = true
		}
		return true
	}
	writeLine := func(col int, line string) {
		counts[col]++
		if cfg.suppress[col] {
			return
		}
		fmt.Fprintf(w, "%s%s\n", cfg.prefix(col), line)
	}

	for have1 && have2 {
		if hasPrev1 && !checkOrd(0, prev1, l1) {
			return 1
		}
		if hasPrev2 && !checkOrd(1, prev2, l2) {
			return 1
		}
		switch cmp := strings.Compare(l1, l2); {
		case cmp < 0:
			writeLine(0, l1)
			prev1, hasPrev1 = l1, true
			have1 = s1.Scan()
			if have1 {
				l1 = s1.Text()
			}
		case cmp > 0:
			writeLine(1, l2)
			prev2, hasPrev2 = l2, true
			have2 = s2.Scan()
			if have2 {
				l2 = s2.Text()
			}
		default:
			writeLine(2, l1)
			prev1, prev2, hasPrev1, hasPrev2 = l1, l2, true, true
			have1, have2 = s1.Scan(), s2.Scan()
			if have1 {
				l1 = s1.Text()
			}
			if have2 {
				l2 = s2.Text()
			}
		}
	}
	for have1 {
		if hasPrev1 && !checkOrd(0, prev1, l1) {
			return 1
		}
		writeLine(0, l1)
		prev1, hasPrev1 = l1, true
		have1 = s1.Scan()
		if have1 {
			l1 = s1.Text()
		}
	}
	for have2 {
		if hasPrev2 && !checkOrd(1, prev2, l2) {
			return 1
		}
		writeLine(1, l2)
		prev2, hasPrev2 = l2, true
		have2 = s2.Scan()
		if have2 {
			l2 = s2.Text()
		}
	}
	if cfg.total {
		parts := make([]string, 0, 4)
		for i := range 3 {
			if !cfg.suppress[i] {
				parts = append(parts, fmt.Sprintf("%d", counts[i]))
			}
		}
		parts = append(parts, "total")
		fmt.Fprintln(w, strings.Join(parts, cfg.delim))
	}
	return 0
}
