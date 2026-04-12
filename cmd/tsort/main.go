// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/tsort: topological sort of partial orderings.
// Implements srd102-tsort R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "tsort"

type runAction int

const (
	runSort runAction = iota
	runHelp
	runVersion
)

// graph holds the directed graph for topological sorting.
type graph struct {
	order   []string            // nodes in insertion order
	succs   map[string][]string // successors per node (insertion order)
	predCnt map[string]int      // predecessor count per node
	done    map[string]bool     // already output
	file    string              // filename for error messages
}

// queue is a FIFO queue of node names, matching GNU tsort's zeros queue.
type queue struct {
	items []string
	front int
}

// enqueue adds a node to the back of the queue.
func (q *queue) enqueue(s string) { q.items = append(q.items, s) }

// dequeue removes and returns the front node.
func (q *queue) dequeue() string {
	s := q.items[q.front]
	q.front++
	return s
}

// empty returns true if the queue has no elements.
func (q *queue) empty() bool { return q.front >= len(q.items) }

// R2.3: SIGPIPE handler installed at start.
// R1.4: main entry with argument parsing.
func main() {
	sys.InstallSIGPIPEHandler()

	file, act, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		tryHelp()
		os.Exit(1)
	}

	switch act {
	case runHelp:
		printUsage()
		return
	case runVersion:
		printVersion()
		return
	}

	os.Exit(run(file))
}

// tryHelp prints the "Try --help" hint to stderr.
func tryHelp() {
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
}

// printVersion prints version to stdout.
func printVersion() {
	fmt.Println("tsort (go-unix-utils) 0.1.0")
}

// printUsage prints the help message to stdout.
func printUsage() {
	fmt.Print(`Usage: tsort [OPTION] [FILE]
Write totally ordered list consistent with the partial ordering in FILE.

With no FILE, or when FILE is -, read standard input.

      --help     display this help and exit
      --version  output version information and exit
`)
}

// parseArgs parses command-line arguments.
// R1.4: accepts optional FILE argument, --help, --version.
// R2.1: rejects invalid options and extra operands with exit 1.
func parseArgs(args []string) (string, runAction, error) {
	file := "-"
	flagsDone := false
	var operands []string

	for _, arg := range args {
		if flagsDone {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		switch arg {
		case "--help":
			return "", runHelp, nil
		case "--version":
			return "", runVersion, nil
		}
		if strings.HasPrefix(arg, "--") {
			return "", 0, fmt.Errorf("unrecognized option '%s'", arg)
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			return "", 0, fmt.Errorf("invalid option -- '%c'", arg[1])
		}
		operands = append(operands, arg)
	}

	if len(operands) > 1 {
		return "", 0, fmt.Errorf("extra operand '%s'", operands[1])
	}
	if len(operands) == 1 {
		file = operands[0]
	}
	return file, runSort, nil
}

// run opens the input, builds the graph, and performs topological sort.
// R2.1: returns 0 on success. R2.2: returns 1 on cycle, malformed input, or I/O error.
func run(file string) int {
	r, err := openInput(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}
	defer closeInput(r, file)

	tokens, err := readTokens(r)
	if err != nil {
		// R2.2: include filename in read error for parity with gtsort.
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, file, err)
		return 1
	}

	// R1.3: odd number of tokens is an error.
	if len(tokens)%2 != 0 {
		fmt.Fprintf(os.Stderr, "%s: %s: input contains an odd number of tokens\n",
			programName, file)
		return 1
	}

	g := buildGraph(tokens, file)
	if topoSort(g) {
		return 1
	}
	return 0
}

// openInput opens a file for reading; "-" means stdin.
// R2.2: returns error on file open failure.
func openInput(name string) (io.ReadCloser, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", name, unwrapPathErr(err))
	}
	return f, nil
}

// closeInput closes a reader if it's not stdin.
func closeInput(r io.ReadCloser, name string) {
	if name != "-" {
		r.Close() // best-effort close
	}
}

// unwrapPathErr extracts the inner error from a PathError.
func unwrapPathErr(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// readTokens reads all whitespace-separated tokens from the reader.
// R1.1: tokens are separated by any whitespace (spaces, tabs, newlines).
func readTokens(r io.Reader) ([]string, error) {
	var tokens []string
	sc := bufio.NewScanner(r)
	sc.Split(bufio.ScanWords)
	for sc.Scan() {
		tokens = append(tokens, sc.Text())
	}
	return tokens, sc.Err()
}

// buildGraph constructs a directed graph from pairs of tokens.
// R1.1: each pair "A B" means A must come before B.
func buildGraph(tokens []string, file string) *graph {
	g := &graph{
		succs:   make(map[string][]string),
		predCnt: make(map[string]int),
		done:    make(map[string]bool),
		file:    file,
	}
	nodeSet := make(map[string]bool)

	for i := 0; i < len(tokens); i += 2 {
		a, b := tokens[i], tokens[i+1]
		addNode(g, a, nodeSet)
		addNode(g, b, nodeSet)
		if a != b {
			addEdge(g, a, b)
		}
	}
	return g
}

// addNode registers a node if not already present.
func addNode(g *graph, name string, nodeSet map[string]bool) {
	if nodeSet[name] {
		return
	}
	nodeSet[name] = true
	g.order = append(g.order, name)
}

// addEdge adds a directed edge from a to b.
// Skips if the last successor of a is already b (matching GNU behavior).
func addEdge(g *graph, a, b string) {
	succs := g.succs[a]
	if len(succs) > 0 && succs[len(succs)-1] == b {
		return
	}
	g.succs[a] = append(succs, b)
	g.predCnt[b]++
}

// topoSort performs topological sort with cycle detection.
// Returns true if any cycle was detected.
// R1.2: outputs one node per line. Reports cycles to stderr.
func topoSort(g *graph) bool {
	q := initZeros(g)
	w := bufio.NewWriter(os.Stdout)
	hasCycle := false
	remaining := len(g.order)

	for remaining > 0 {
		if q.empty() {
			hasCycle = true
			node := findAndReportCycle(g)
			q.enqueue(node)
		}
		node := q.dequeue()
		if g.done[node] {
			continue
		}
		g.done[node] = true
		remaining--
		fmt.Fprintln(w, node)
		freeSuccessors(g, node, q)
	}
	w.Flush()
	return hasCycle
}

// initZeros builds the initial FIFO queue of zero-predecessor nodes.
// Scans in insertion order so first-inserted nodes are dequeued first.
func initZeros(g *graph) *queue {
	q := &queue{}
	for _, n := range g.order {
		if g.predCnt[n] == 0 {
			q.enqueue(n)
		}
	}
	return q
}

// freeSuccessors decrements predecessor counts for node's successors.
// Iterates in reverse insertion order to match GNU's prepend-based ordering.
func freeSuccessors(g *graph, node string, q *queue) {
	succs := g.succs[node]
	for i := len(succs) - 1; i >= 0; i-- {
		s := succs[i]
		if g.done[s] {
			continue
		}
		g.predCnt[s]--
		if g.predCnt[s] == 0 {
			q.enqueue(s)
		}
	}
}

// findAndReportCycle finds a cycle and reports it to stderr.
// R1.2: prints "input contains a loop:" followed by nodes in the cycle.
func findAndReportCycle(g *graph) string {
	fmt.Fprintf(os.Stderr, "%s: %s: input contains a loop:\n",
		programName, g.file)
	start := findCycleStart(g)
	walkCycle(g, start)
	return start
}

// findCycleStart finds a node in a cycle: one with pred count > 0
// that has at least one successor with pred count > 0.
// Scans in insertion order to match GNU behavior.
func findCycleStart(g *graph) string {
	for _, n := range g.order {
		if g.done[n] || g.predCnt[n] == 0 {
			continue
		}
		if hasUndoneSuccWithPred(g, n) {
			return n
		}
	}
	// Fallback: any undone node.
	for _, n := range g.order {
		if !g.done[n] {
			return n
		}
	}
	return ""
}

// hasUndoneSuccWithPred returns true if n has a successor with pred count > 0.
func hasUndoneSuccWithPred(g *graph, n string) bool {
	for _, s := range g.succs[n] {
		if !g.done[s] && g.predCnt[s] > 0 {
			return true
		}
	}
	return false
}

// walkCycle follows the cycle path and prints each node to stderr.
func walkCycle(g *graph, start string) {
	node := start
	for {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, node)
		next := firstUndoneSuccWithPred(g, node)
		if next == "" || next == start {
			break
		}
		node = next
	}
}

// firstUndoneSuccWithPred returns the first successor of node (scanning
// in insertion order) that is not done and has pred count > 0.
func firstUndoneSuccWithPred(g *graph, node string) string {
	for _, s := range g.succs[node] {
		if !g.done[s] && g.predCnt[s] > 0 {
			return s
		}
	}
	return ""
}
