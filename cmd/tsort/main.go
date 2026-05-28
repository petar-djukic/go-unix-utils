// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type node struct {
	str     string
	count   int
	printed bool
	qlink   *node
	top     *successor
}

type successor struct {
	suc  *node
	next *successor
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

func run() int {
	args := os.Args[1:]
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "tsort: extra operand '%s'\n", args[1])
		fmt.Fprintln(os.Stderr, "Try 'tsort --help' for more information.")
		return 1
	}

	file := "-"
	if len(args) == 1 {
		file = args[0]
	}

	var input *os.File
	if file == "-" {
		input = os.Stdin
	} else {
		var err error
		input, err = os.Open(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tsort: %s: %s\n", file, unwrapPathErr(err))
			return 1
		}
		defer input.Close()
	}

	scanner := bufio.NewScanner(input)
	scanner.Split(bufio.ScanWords)

	nodes := make(map[string]*node)
	var names []string

	getOrCreate := func(s string) *node {
		if n, ok := nodes[s]; ok {
			return n
		}
		n := &node{str: s}
		nodes[s] = n
		names = append(names, s)
		return n
	}

	var j, k *node
	for scanner.Scan() {
		k = getOrCreate(scanner.Text())
		if j != nil {
			if j.str != k.str {
				k.count++
				j.top = &successor{suc: k, next: j.top}
			}
			k = nil
		}
		j = k
	}

	if k != nil {
		fmt.Fprintf(os.Stderr, "tsort: %s: input contains an odd number of tokens\n", file)
		return 1
	}

	sort.Strings(names)

	w := bufio.NewWriter(os.Stdout)

	nStrings := len(nodes)
	ok := true

	for nStrings > 0 {
		var head, zeros *node
		for _, name := range names {
			n := nodes[name]
			if n.count == 0 && !n.printed {
				if head == nil {
					head = n
				} else {
					zeros.qlink = n
				}
				zeros = n
			}
		}

		for head != nil {
			fmt.Fprintln(w, head.str)
			head.printed = true
			nStrings--

			for p := head.top; p != nil; p = p.next {
				p.suc.count--
				if p.suc.count == 0 {
					zeros.qlink = p.suc
					zeros = p.suc
				}
			}

			head = head.qlink
		}

		if nStrings > 0 {
			w.Flush()
			fmt.Fprintf(os.Stderr, "tsort: %s: input contains a loop:\n", file)
			ok = false
			detectLoop(nodes, names)
		}
	}

	w.Flush()

	if !ok {
		return 1
	}
	return 0
}

func detectLoop(nodes map[string]*node, names []string) {
	var loop *node

	for {
		for _, name := range names {
			k := nodes[name]
			if k.count <= 0 || k.printed {
				continue
			}

			if loop == nil {
				loop = k
				continue
			}

			pp := &k.top
			for *pp != nil {
				if (*pp).suc == loop {
					if k.qlink != nil {
						for loop != nil {
							tmp := loop.qlink
							fmt.Fprintf(os.Stderr, "tsort: %s\n", loop.str)
							if loop == k {
								edge := *pp
								edge.suc.count--
								*pp = edge.next
								break
							}
							loop.qlink = nil
							loop = tmp
						}
						for loop != nil {
							tmp := loop.qlink
							loop.qlink = nil
							loop = tmp
						}
						return
					}
					k.qlink = loop
					loop = k
					break
				}
				pp = &(*pp).next
			}
		}

		if loop == nil {
			return
		}
	}
}

func unwrapPathErr(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
