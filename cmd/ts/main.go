// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ts: prepend timestamps to stdin lines.
// Implements srd004-ts R1.1-R1.4.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "ts"

// defaultStrftimeFmt is the default strftime format per R1.2.
const defaultStrftimeFmt = "%b %d %H:%M:%S"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and timestamps stdin.
// R1.1-R1.4: core ts behavior.
func run(args []string) int {
	format, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return timestampStdin(format)
}

// parseArgs extracts the optional format string from positional args.
// Unrecognized flags produce an error per R7.2.
func parseArgs(args []string) (string, error) {
	format := defaultStrftimeFmt
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			return "", fmt.Errorf("%s: unrecognized option '%s'", progName, arg)
		}
		format = arg
	}
	return format, nil
}

// timestampStdin reads stdin line by line and prepends a timestamp.
// R1.1: read stdin line by line, prepend timestamp + space.
// R1.2: format evaluated at the time each line is received.
// R1.3: flush stdout after each line.
// R1.4: preserve original newline, do not add extra.
func timestampStdin(format string) int {
	goFmt := strftimeToGo(format)
	scanner := bufio.NewScanner(os.Stdin)
	w := bufio.NewWriter(os.Stdout)
	for scanner.Scan() {
		now := time.Now()
		ts := now.Format(goFmt)
		fmt.Fprintf(w, "%s %s\n", ts, scanner.Text())
		w.Flush()
	}
	return 0
}

// strftimeConversions maps strftime specifiers to Go time.Format tokens.
var strftimeConversions = map[byte]string{
	'a': "Mon", 'A': "Monday",
	'b': "Jan", 'B': "January", 'h': "Jan",
	'd': "02", 'e': "_2",
	'H': "15", 'I': "03",
	'j': "002",
	'm': "01", 'M': "04",
	'p': "PM",
	'S': "05",
	'y': "06", 'Y': "2006",
	'z': "-0700", 'Z': "MST",
	'n': "\n", 't': "\t",
	'T': "15:04:05",
	'R': "15:04",
	'D': "01/02/06",
	'F': "2006-01-02",
	'r': "03:04:05 PM",
}

// strftimeToGo converts a strftime format string to a Go time.Format layout.
func strftimeToGo(format string) string {
	var b strings.Builder
	b.Grow(len(format) * 2)
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			b.WriteByte(format[i])
			continue
		}
		i++
		if format[i] == '%' {
			b.WriteByte('%')
			continue
		}
		if goToken, ok := strftimeConversions[format[i]]; ok {
			b.WriteString(goToken)
		} else {
			b.WriteByte('%')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}
