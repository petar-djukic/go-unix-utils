// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tsort implements topological sort (prd102-tsort R1, R2).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "tsort"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the tsort logic and returns the exit code.
// R1.4: reads from FILE argument, stdin when no argument or "-".
// R2.1: exits 0 on success. R2.2: exits 1 on cycle or malformed input.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 1 {
		fmt.Fprintf(stderr, "%s: extra operand '%s'\n", progName, args[1])
		return 1
	}
	filename := "-"
	r := stdin
	if len(args) == 1 && args[0] != "-" {
		filename = args[0]
		f, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			return 1
		}
		defer f.Close() // best-effort close
		r = f
	}
	return toposort(r, filename, stdout, stderr)
}

// toposort reads pairs, builds a DAG, and emits a topological order.
// R1.1: reads pairs "A B" meaning A before B.
// R1.2: detects and reports cycles, continues sorting, exits 1.
// R1.3: exits 1 on odd number of tokens.
func toposort(r io.Reader, filename string, stdout, stderr io.Writer) int {
	tokens, err := readTokens(r)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1
	}
	if len(tokens)%2 != 0 {
		fmt.Fprintf(stderr, "%s: %s: input contains an odd number of tokens\n",
			progName, filename)
		return 1
	}
	g, order := buildGraph(tokens)
	return emitOrder(g, order, filename, stdout, stderr)
}

// readTokens scans all whitespace-delimited tokens from the reader.
func readTokens(r io.Reader) ([]string, error) {
	var tokens []string
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}
	return tokens, scanner.Err()
}

// dagGraph holds adjacency lists and in-degree counts for topological sort.
type dagGraph struct {
	adj    map[string][]string
	indeg  map[string]int
	exists map[string]bool
}

// buildGraph constructs the DAG from token pairs and returns insertion order.
func buildGraph(tokens []string) (*dagGraph, []string) {
	g := &dagGraph{
		adj:    make(map[string][]string),
		indeg:  make(map[string]int),
		exists: make(map[string]bool),
	}
	var order []string
	for i := 0; i < len(tokens); i += 2 {
		a, b := tokens[i], tokens[i+1]
		order = ensureNode(g, a, order)
		order = ensureNode(g, b, order)
		if a != b {
			g.adj[a] = append(g.adj[a], b)
			g.indeg[b]++
		}
	}
	return g, order
}

// ensureNode registers a node if not yet seen, preserving insertion order.
func ensureNode(g *dagGraph, name string, order []string) []string {
	if !g.exists[name] {
		g.exists[name] = true
		if _, ok := g.indeg[name]; !ok {
			g.indeg[name] = 0
		}
		order = append(order, name)
	}
	return order
}

// emitOrder performs Kahn's algorithm matching GNU tsort ordering.
// GNU uses a FIFO queue with alphabetically-sorted initial zeros and
// reverse-insertion-order successor iteration (due to prepended linked list).
// R1.2: reports cycles and continues sorting.
func emitOrder(g *dagGraph, order []string, filename string, stdout, stderr io.Writer) int {
	queue := collectReady(g, order)
	exitCode := 0
	w := bufio.NewWriter(stdout)
	for len(queue) > 0 || len(g.indeg) > 0 {
		if len(queue) == 0 {
			exitCode = 1
			queue = breakCycle(g, order, filename, stderr)
			continue
		}
		node := queue[0]
		queue = queue[1:]
		fmt.Fprintln(w, node)
		queue = decrementSuccessors(g, node, queue)
	}
	w.Flush() // best-effort flush
	return exitCode
}

// collectReady gathers zero in-degree nodes sorted alphabetically.
// GNU tsort uses a BST traversal that produces alphabetical order.
func collectReady(g *dagGraph, order []string) []string {
	var q []string
	for _, n := range order {
		if g.indeg[n] == 0 {
			q = append(q, n)
			delete(g.indeg, n)
		}
	}
	sort.Strings(q)
	return q
}

// decrementSuccessors reduces in-degree for successors in reverse order.
// GNU tsort prepends successors to a linked list, so iteration is in reverse
// insertion order. Our adjacency lists use append, so we iterate backwards
// and append newly ready nodes to the FIFO queue.
func decrementSuccessors(g *dagGraph, node string, queue []string) []string {
	succs := g.adj[node]
	for i := len(succs) - 1; i >= 0; i-- {
		succ := succs[i]
		if _, ok := g.indeg[succ]; !ok {
			continue
		}
		g.indeg[succ]--
		if g.indeg[succ] == 0 {
			queue = append(queue, succ)
			delete(g.indeg, succ)
		}
	}
	delete(g.adj, node)
	return queue
}

// breakCycle finds a cycle, reports it in GNU format, and frees one node.
func breakCycle(g *dagGraph, order []string, filename string, stderr io.Writer) []string {
	start := firstRemaining(g, order)
	cycle := findCycle(g, start)
	reportCycle(cycle, filename, stderr)
	node := cycle[0]
	delete(g.indeg, node)
	return []string{node}
}

// firstRemaining returns the first node in order still in g.indeg.
func firstRemaining(g *dagGraph, order []string) string {
	for _, n := range order {
		if _, ok := g.indeg[n]; ok {
			return n
		}
	}
	return ""
}

// dfsFrame holds a DFS stack entry with the current successor index.
type dfsFrame struct {
	node string
	idx  int
}

// findCycle traces a cycle from start using DFS, returning the cycle path.
func findCycle(g *dagGraph, start string) []string {
	onPath := make(map[string]bool)
	stack := []dfsFrame{{node: start, idx: 0}}
	onPath[start] = true
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		succs := g.adj[top.node]
		if top.idx >= len(succs) {
			onPath[top.node] = false
			stack = stack[:len(stack)-1]
			continue
		}
		succ := succs[top.idx]
		top.idx++
		if _, ok := g.indeg[succ]; !ok {
			continue // already removed
		}
		if onPath[succ] {
			return extractCycle(stack, succ)
		}
		onPath[succ] = true
		stack = append(stack, dfsFrame{node: succ, idx: 0})
	}
	return []string{start}
}

// extractCycle returns the cycle from the DFS stack starting at target.
func extractCycle(stack []dfsFrame, target string) []string {
	var cycle []string
	found := false
	for _, f := range stack {
		if f.node == target {
			found = true
		}
		if found {
			cycle = append(cycle, f.node)
		}
	}
	return cycle
}

// reportCycle writes the GNU-style cycle diagnostic to stderr.
func reportCycle(cycle []string, filename string, stderr io.Writer) {
	fmt.Fprintf(stderr, "%s: %s: input contains a loop:\n", progName, filename)
	for _, n := range cycle {
		fmt.Fprintf(stderr, "%s: %s\n", progName, n)
	}
}
