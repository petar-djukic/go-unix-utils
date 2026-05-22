// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/fmt implements srd070-fmt: simple text formatter that reformats
// paragraphs to fit within a specified line width.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var (
	flagWidth   int
	flagGoal    int
	flagSplit   bool
	flagTagged  bool
	flagPrefix  string
	flagUniform bool
)

func init() {
	flagWidth = 75
	flagGoal = 0
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := parseFlags(os.Args[1:])
	if len(args) == 0 {
		args = []string{"-"}
	}

	if flagGoal == 0 {
		flagGoal = flagWidth * 93 / 100
	}

	exitCode := 0
	for _, name := range args {
		if err := fmtFile(name); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "fmt: %s\n", formatErr(err))
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func parseFlags(args []string) []string {
	var files []string
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || arg == "" || arg == "-" || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if len(arg) > 2 && arg[:2] == "--" {
			i = parseLongFlag(args, i)
			continue
		}
		i = parseShortFlags(args, i)
	}
	return files
}

func parseLongFlag(args []string, i int) int {
	arg := args[i]
	switch {
	case matchLong(arg, "--width"):
		flagWidth = parseLongIntArg(arg, "--width")
	case matchLong(arg, "--goal"):
		flagGoal = parseLongIntArg(arg, "--goal")
	case arg == "--split-only":
		flagSplit = true
	case arg == "--tagged-paragraph":
		flagTagged = true
	case matchLong(arg, "--prefix"):
		flagPrefix = parseLongStrArg(arg, "--prefix")
	case arg == "--uniform-spacing":
		flagUniform = true
	default:
		fmt.Fprintf(os.Stderr, "fmt: unrecognized option '%s'\n", arg)
		os.Exit(1)
	}
	return i
}

func matchLong(arg, name string) bool {
	return arg == name || (len(arg) > len(name) && arg[:len(name)+1] == name+"=")
}

func parseLongIntArg(arg, name string) int {
	eq := len(name) + 1
	if len(arg) <= len(name) || arg[len(name)] != '=' {
		fmt.Fprintf(os.Stderr, "fmt: option '%s' requires an argument\n", name)
		os.Exit(1)
	}
	v, err := strconv.Atoi(arg[eq:])
	if err != nil || v < 1 {
		fmt.Fprintf(os.Stderr, "fmt: invalid width '%s'\n", arg[eq:])
		os.Exit(1)
	}
	return v
}

func parseLongStrArg(arg, name string) string {
	eq := len(name) + 1
	if len(arg) <= len(name) || arg[len(name)] != '=' {
		fmt.Fprintf(os.Stderr, "fmt: option '%s' requires an argument\n", name)
		os.Exit(1)
	}
	return arg[eq:]
}

func parseShortFlags(args []string, i int) int {
	arg := args[i]
	for j := 1; j < len(arg); j++ {
		c := arg[j]
		switch c {
		case 'w':
			flagWidth = consumeIntArg(args, arg, &i, j)
			return i
		case 'g':
			flagGoal = consumeIntArg(args, arg, &i, j)
			return i
		case 'p':
			flagPrefix = consumeStrArg(args, arg, &i, j)
			return i
		case 's':
			flagSplit = true
		case 't':
			flagTagged = true
		case 'u':
			flagUniform = true
		default:
			fmt.Fprintf(os.Stderr, "fmt: invalid option -- '%c'\n", c)
			os.Exit(1)
		}
	}
	return i
}

func consumeIntArg(args []string, arg string, i *int, j int) int {
	rest := arg[j+1:]
	if rest != "" {
		v, err := strconv.Atoi(rest)
		if err != nil || v < 1 {
			fmt.Fprintf(os.Stderr, "fmt: invalid width '%s'\n", rest)
			os.Exit(1)
		}
		return v
	}
	*i++
	if *i >= len(args) {
		fmt.Fprintf(os.Stderr, "fmt: option requires an argument -- '%c'\n", arg[j])
		os.Exit(1)
	}
	v, err := strconv.Atoi(args[*i])
	if err != nil || v < 1 {
		fmt.Fprintf(os.Stderr, "fmt: invalid width '%s'\n", args[*i])
		os.Exit(1)
	}
	return v
}

func consumeStrArg(args []string, arg string, i *int, j int) string {
	rest := arg[j+1:]
	if rest != "" {
		return rest
	}
	*i++
	if *i >= len(args) {
		fmt.Fprintf(os.Stderr, "fmt: option requires an argument -- '%c'\n", arg[j])
		os.Exit(1)
	}
	return args[*i]
}

func fmtFile(name string) error {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}
	return formatInput(r)
}

func formatInput(r io.Reader) error {
	paragraphs, err := detectParagraphs(r)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(os.Stdout)
	for i, para := range paragraphs {
		if i > 0 {
			if werr := writeByte(w, '\n'); werr != nil {
				return werr
			}
		}
		if err := formatParagraph(w, para); err != nil {
			return err
		}
	}
	return w.Flush()
}

// TODO: implement paragraph detection (R2.1, R3.1)
func detectParagraphs(r io.Reader) ([][]string, error) {
	fmt.Fprintln(os.Stderr, "fmt: paragraph detection not implemented")
	os.Exit(1)
	return nil, nil
}

// TODO: implement paragraph formatting (R1.1, R7.1, R8.1)
func formatParagraph(w *bufio.Writer, lines []string) error {
	fmt.Fprintln(os.Stderr, "fmt: paragraph formatting not implemented")
	os.Exit(1)
	return nil
}

// TODO: implement line filling to goal/max width (R5.1, R6.1, R7.1)
func fillLines(words []string, indent string, goal, max int) []string {
	fmt.Fprintln(os.Stderr, "fmt: line filling not implemented")
	os.Exit(1)
	return nil
}

// TODO: implement split-only mode (R9.1)
func splitLongLines(lines []string, max int) []string {
	fmt.Fprintln(os.Stderr, "fmt: split-only mode not implemented")
	os.Exit(1)
	return nil
}

// TODO: implement tagged-paragraph handling (R12.1)
func handleTaggedParagraph(lines []string) (string, string, []string) {
	fmt.Fprintln(os.Stderr, "fmt: tagged-paragraph handling not implemented")
	os.Exit(1)
	return "", "", nil
}

// TODO: implement prefix filtering (R11.1)
func filterPrefix(lines []string, prefix string) ([]string, []bool) {
	fmt.Fprintln(os.Stderr, "fmt: prefix filtering not implemented")
	os.Exit(1)
	return nil, nil
}

// TODO: implement uniform spacing normalization (R10.1)
func normalizeSpacing(line string) string {
	fmt.Fprintln(os.Stderr, "fmt: uniform spacing not implemented")
	os.Exit(1)
	return ""
}

func writeByte(w *bufio.Writer, b byte) error {
	return w.WriteByte(b)
}

func formatErr(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Sprintf("%s: %s", pe.Path, pe.Err)
	}
	return err.Error()
}
