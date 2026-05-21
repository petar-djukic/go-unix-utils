package main

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	lines, err := readLines(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "shuf: %v\n", err)
		os.Exit(1)
	}

	rand.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})

	w := bufio.NewWriter(os.Stdout)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "shuf: write error: %v\n", err)
		os.Exit(1)
	}
}

func readLines(args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{"-"}
	}

	var lines []string
	for _, name := range args {
		ls, err := readFileLines(name)
		if err != nil {
			return nil, err
		}
		lines = append(lines, ls...)
	}
	return lines, nil
}

func readFileLines(name string) ([]string, error) {
	var f *os.File
	if name == "-" {
		f = os.Stdin
	} else {
		var err error
		f, err = os.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
