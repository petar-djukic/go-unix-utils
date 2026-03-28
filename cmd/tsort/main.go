// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tsort implements topological sort of directed graphs.
// Implements prd102-tsort R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "tsort"

func main() {
	// R2.3: SIGPIPE handling.
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run executes tsort logic, returning the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	r, inputName, err := openInput(args, stdin, stderr)
	if err != nil {
		return 1
	}
	if r != stdin {
		defer r.(io.Closer).Close() //nolint:errcheck // best-effort close
	}

	tokens, err := readTokens(r)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return 1
	}

	// R1.3: odd number of tokens is an error.
	if len(tokens)%2 != 0 {
		fmt.Fprintf(stderr, "%s: %s: input contains an odd number of tokens\n",
			programName, inputName)
		return 1
	}

	g := buildGraph(tokens)
	return kahnSort(g, inputName, stdout, stderr)
}

// openInput returns the reader and source name for tsort input.
// R1.4: reads from FILE argument, or stdin when no argument or "-".
func openInput(args []string, stdin io.Reader, stderr io.Writer) (io.Reader, string, error) {
	if len(args) > 1 {
		fmt.Fprintf(stderr, "%s: extra operand %q\n", programName, args[1])
		return nil, "", fmt.Errorf("extra operand")
	}
	if len(args) == 0 || args[0] == "-" {
		return stdin, "-", nil
	}
	f, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return nil, "", err
	}
	return f, args[0], nil
}

// readTokens reads all whitespace-separated tokens from r.
func readTokens(r io.Reader) ([]string, error) {
	var tokens []string
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}
	return tokens, nil
}

// graph holds the adjacency data for topological sorting.
type graph struct {
	nodes   []string
	succs   map[string][]string
	preds   map[string]int
	removed map[string]bool
}

// buildGraph constructs successor/predecessor maps and an ordered node list.
func buildGraph(tokens []string) *graph {
	g := &graph{
		succs:   make(map[string][]string),
		preds:   make(map[string]int),
		removed: make(map[string]bool),
	}
	seen := make(map[string]bool)

	for i := 0; i < len(tokens); i += 2 {
		from, to := tokens[i], tokens[i+1]
		addNode(g, from, seen)
		addNode(g, to, seen)
		if from != to {
			g.succs[from] = append(g.succs[from], to)
			g.preds[to]++
		}
	}
	return g
}

func addNode(g *graph, n string, seen map[string]bool) {
	if !seen[n] {
		seen[n] = true
		g.nodes = append(g.nodes, n)
		g.preds[n] = 0
	}
}

// kahnSort performs Kahn's algorithm with FIFO queue, matching GNU tsort.
// R1.1: outputs nodes in topological order.
// R1.2: detects cycles, reports on stderr, continues with partial output.
func kahnSort(g *graph, inputName string, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	defer w.Flush() //nolint:errcheck // best-effort flush

	hasCycle := false
	queue := collectZeroPred(g)
	remaining := len(g.nodes)

	for remaining > 0 {
		if len(queue) > 0 {
			queue = drainQueue(g, queue, w)
			remaining = countRemaining(g)
			continue
		}
		// No ready nodes: cycle exists.
		hasCycle = true
		w.Flush() //nolint:errcheck // flush before stderr
		node := findFirstNonRemoved(g)
		reportCycle(node, g, inputName, stderr)
		removeNode(g, node)
		fmt.Fprintln(w, node)
		remaining = countRemaining(g)
		queue = collectZeroPred(g)
	}

	if hasCycle {
		return 1
	}
	return 0
}

// collectZeroPred returns all non-removed nodes with zero predecessors.
func collectZeroPred(g *graph) []string {
	var queue []string
	for _, n := range g.nodes {
		if !g.removed[n] && g.preds[n] == 0 {
			queue = append(queue, n)
		}
	}
	return queue
}

// drainQueue processes the FIFO queue, emitting nodes and enqueuing
// newly-ready successors.
func drainQueue(g *graph, queue []string, w *bufio.Writer) []string {
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if g.removed[node] {
			continue
		}
		fmt.Fprintln(w, node)
		g.removed[node] = true
		for _, s := range g.succs[node] {
			if !g.removed[s] {
				g.preds[s]--
				if g.preds[s] == 0 {
					queue = append(queue, s)
				}
			}
		}
	}
	return queue
}

// removeNode marks a node as removed and decrements predecessor counts.
func removeNode(g *graph, node string) {
	g.removed[node] = true
	for _, s := range g.succs[node] {
		if !g.removed[s] {
			g.preds[s]--
		}
	}
}

func countRemaining(g *graph) int {
	count := 0
	for _, n := range g.nodes {
		if !g.removed[n] {
			count++
		}
	}
	return count
}

// findFirstNonRemoved returns the first node not yet removed.
func findFirstNonRemoved(g *graph) string {
	for _, n := range g.nodes {
		if !g.removed[n] {
			return n
		}
	}
	return ""
}

// reportCycle traces and prints the cycle starting from node.
func reportCycle(start string, g *graph, inputName string, stderr io.Writer) {
	fmt.Fprintf(stderr, "%s: %s: input contains a loop:\n", programName, inputName)
	cycle := traceCycle(start, g)
	for _, n := range cycle {
		fmt.Fprintf(stderr, "%s: %s\n", programName, n)
	}
}

// traceCycle follows successor edges from start to find a cycle.
func traceCycle(start string, g *graph) []string {
	visited := map[string]bool{start: true}
	path := []string{start}
	current := start

	for {
		next := findNonRemovedSucc(g.succs[current], g.removed)
		if next == "" || visited[next] {
			break
		}
		visited[next] = true
		path = append(path, next)
		current = next
	}
	return path
}

// findNonRemovedSucc returns the first non-removed successor.
func findNonRemovedSucc(neighbors []string, removed map[string]bool) string {
	for _, n := range neighbors {
		if !removed[n] {
			return n
		}
	}
	return ""
}
