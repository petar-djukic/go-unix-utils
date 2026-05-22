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
	"strings"
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
	if err != nil || v < 0 {
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
		if err != nil || v < 0 {
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
	if err != nil || v < 0 {
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
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	w := bufio.NewWriter(os.Stdout)
	var werr error
	if flagPrefix != "" {
		werr = formatWithPrefix(w, lines)
	} else {
		werr = writeFormatted(w, lines)
	}
	if werr != nil {
		return werr
	}
	return w.Flush()
}

func writeFormatted(w *bufio.Writer, lines []string) error {
	blocks := groupParagraphs(lines)
	for _, block := range blocks {
		if len(block) == 0 {
			if _, err := w.WriteString("\n"); err != nil {
				return err
			}
			continue
		}
		if err := formatParagraph(w, block); err != nil {
			return err
		}
	}
	return nil
}

func groupParagraphs(lines []string) [][]string {
	var result [][]string
	var current []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				result = append(result, current)
				current = nil
			}
			result = append(result, nil)
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		result = append(result, current)
	}
	return result
}

func formatParagraph(w *bufio.Writer, lines []string) error {
	if flagSplit {
		return writeSplitOnly(w, lines)
	}
	if len(lines) == 1 && len(lines[0]) <= flagWidth {
		return writeLines(w, lines)
	}
	firstIndent, bodyIndent := getIndents(lines)
	words := extractWords(lines)
	if len(words) == 0 {
		return nil
	}
	filled := fillLines(words, firstIndent, bodyIndent, flagGoal, flagWidth)
	return writeLines(w, filled)
}

func getIndents(lines []string) (string, string) {
	first := extractIndent(lines[0])
	if len(lines) > 1 {
		return first, extractIndent(lines[1])
	}
	return first, first
}

func extractIndent(s string) string {
	trimmed := strings.TrimLeft(s, " \t")
	return s[:len(s)-len(trimmed)]
}

func extractWords(lines []string) []string {
	var words []string
	for _, line := range lines {
		words = append(words, strings.Fields(line)...)
	}
	return words
}

func fillLines(words []string, firstIndent, bodyIndent string, goal, max int) []string {
	if len(words) == 0 {
		return nil
	}
	if goal > max {
		goal = max
	}
	var result []string
	line := firstIndent + words[0]
	for i := 1; i < len(words); i++ {
		sep := wordSep(words[i-1])
		trial := line + sep + words[i]
		if len(trial) > max || pastGoal(len(line), len(trial), goal) {
			result = append(result, line)
			line = bodyIndent + words[i]
		} else {
			line = trial
		}
	}
	return append(result, line)
}

func pastGoal(lineLen, trialLen, goal int) bool {
	if trialLen <= goal {
		return false
	}
	over := trialLen - goal
	under := goal - lineLen
	if under <= 0 {
		return true
	}
	return over > under
}

func wordSep(prev string) string {
	if endsWithSentence(prev) {
		return "  "
	}
	return " "
}

func endsWithSentence(word string) bool {
	if len(word) == 0 {
		return false
	}
	last := word[len(word)-1]
	return last == '.' || last == '!' || last == '?'
}

func writeLines(w *bufio.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := w.WriteString(line); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return nil
}

func writeSplitOnly(w *bufio.Writer, lines []string) error {
	for _, line := range lines {
		if len(line) <= flagWidth {
			if _, err := w.WriteString(line); err != nil {
				return err
			}
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
			continue
		}
		indent := extractIndent(line)
		words := strings.Fields(line)
		filled := fillLines(words, indent, indent, flagGoal, flagWidth)
		if err := writeLines(w, filled); err != nil {
			return err
		}
	}
	return nil
}

func formatWithPrefix(w *bufio.Writer, allLines []string) error {
	var group []string
	for _, line := range allLines {
		stripped, has := stripPrefix(line)
		if has {
			group = append(group, stripped)
			continue
		}
		if err := flushPrefixGroup(w, group); err != nil {
			return err
		}
		group = nil
		if _, err := w.WriteString(line); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return flushPrefixGroup(w, group)
}

func flushPrefixGroup(w *bufio.Writer, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	blocks := groupParagraphs(lines)
	for _, block := range blocks {
		if err := writePrefixBlock(w, block); err != nil {
			return err
		}
	}
	return nil
}

func writePrefixBlock(w *bufio.Writer, block []string) error {
	if len(block) == 0 {
		_, err := w.WriteString(flagPrefix + "\n")
		return err
	}
	firstIndent, bodyIndent := getIndents(block)
	words := extractWords(block)
	if len(words) == 0 {
		return nil
	}
	pLen := len(flagPrefix)
	filled := fillLines(words, firstIndent, bodyIndent,
		max(1, flagGoal-pLen), max(1, flagWidth-pLen))
	for _, fl := range filled {
		if _, err := w.WriteString(flagPrefix + fl + "\n"); err != nil {
			return err
		}
	}
	return nil
}

func stripPrefix(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, flagPrefix) {
		return trimmed[len(flagPrefix):], true
	}
	return line, false
}

func formatErr(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Sprintf("%s: %s", pe.Path, pe.Err)
	}
	return err.Error()
}
