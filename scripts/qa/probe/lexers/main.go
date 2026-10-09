// Lists every lexer chroma knows about: tab-separated name, aliases and
// filename patterns. Used to build the language corpus of the QA harness:
//
//	go run ./scripts/qa/probe/lexers > qa-out/chroma-lexers.tsv
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2/lexers"
)

func main() {
	names := lexers.Names(false)
	sort.Strings(names)
	fmt.Printf("# %d lexers\n", len(names))
	for _, n := range names {
		l := lexers.Get(n)
		if l == nil {
			continue
		}
		cfg := l.Config()
		fmt.Printf("%s\t%s\t%s\n", cfg.Name, strings.Join(cfg.Aliases, ","), strings.Join(cfg.Filenames, ","))
	}
}
