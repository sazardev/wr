// Comprueba ansi.Hardwrap con la misma llamada que render.go.
package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func main() {
	for _, n := range []int{100, 180, 266, 500} {
		s := `x = "` + strings.Repeat("a", n) + `"`
		out := ansi.Hardwrap(s, 98, true)
		segs := strings.Split(out, "\n")
		total := 0
		for _, seg := range segs {
			total += len(seg)
		}
		fmt.Printf("n=%4d -> segmentos=%d  suma=%d  ok=%v\n", n, len(segs), total, total == len(s))
		for i, seg := range segs {
			fmt.Printf("   seg%d len=%d tail=%q\n", i, len(seg), seg[max(0, len(seg)-12):])
		}
	}
	s := `x = ` + "\x1b[33m\"" + strings.Repeat("a", 260) + "\"\x1b[0m"
	out := ansi.Hardwrap(s, 98, true)
	fmt.Println("con ANSI:", len(strings.Split(out, "\n")), "segmentos")
}
