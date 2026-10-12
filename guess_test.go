package main

import (
	"strings"
	"testing"
)

func TestGuessLangRecognisesClearCode(t *testing.T) {
	for want, code := range map[string]string{
		"go":         "package orders\n\ntype Service struct {\n\tDB *Database\n}\n\nfunc (s *Service) Total(o Order) (float64, error) {\n\tif err := s.DB.Save(o); err != nil {\n\t\treturn 0, err\n\t}\n\treturn 1, nil\n}",
		"python":     "import os\n\nclass Repo:\n    def save(self, order):\n        if order is None:\n            return False\n        return True",
		"javascript": "import fs from 'fs'\n\nconst total = items.map((i) => i.price)\nconsole.log(total)",
		"typescript": "export interface Order {\n  id: string\n  total: number\n}\n\nfunction pay(o: Order): Promise<void> {\n  return Promise.resolve()\n}",
		"json":       "{\n  \"name\": \"wr\",\n  \"version\": 1\n}",
		"bash":       "#!/bin/bash\nset -e\nsudo apt-get install -y chromium\nexport WR_BROWSER=chromium",
		"sql":        "SELECT id, total\nFROM orders\nWHERE total > 10\nORDER BY total;",
		"rust":       "use std::fmt;\n\nfn main() {\n    let mut v = vec![1, 2];\n    println!(\"{:?}\", v);\n}",
		"java":       "import java.util.List;\n\npublic class Orders {\n    public static void main(String[] args) {\n        System.out.println(\"hi\");\n    }\n}",
		"csharp":     "using System;\n\nnamespace Shop {\n    public class Orders {\n        static void Main() { Console.WriteLine(\"hi\"); }\n    }\n}",
	} {
		if got := guessLang(code); got != want {
			t.Errorf("guessLang = %q, want %q for:\n%s", got, want, code)
		}
	}
}

func TestGuessLangStaysSilentWhenUnsure(t *testing.T) {
	for name, code := range map[string]string{
		"prose":      "This is just a paragraph of text that somebody put in a block,\nwith no code in it at all, only words and some commas.",
		"too short":  "x := 1",
		"gdscript":   "extends Node2D\n\nfunc _ready():\n\tvar speed = 10\n\tprint(speed)",
		"ruby":       "class Order\n  def total\n    items.sum(&:price)\n  end\nend",
		"c":          "#include <stdio.h>\n\nint main(void) {\n    printf(\"hi\\n\");\n    return 0;\n}",
		"php":        "<?php\n$total = array_sum($prices);\necho $total;",
		"tree":       "src/\n  main.go\n  render.go\n  fetch.go\ndocs/\n  GUIDE.md",
		"output":     "ok  \tgithub.com/sazardev/wr\t0.9s\nPASS",
		"one signal": "// a comment\nfoo := bar\nbaz",
		"json-ish":   "{ not really json,\n  nor close }",
	} {
		if got := guessLang(code); got != "" {
			t.Errorf("%s: guessed %q, want nothing:\n%s", name, got, code)
		}
	}
}

func TestGuessedBlocksAreHighlightedAndLabelled(t *testing.T) {
	md := "```\npackage orders\n\ntype Service struct {\n\tDB *Database\n}\n\nfunc New() *Service {\n\treturn &Service{}\n}\n```\n"
	d := render(t, md, 80, nil)
	all := strings.Join(d.Plain, "\n")
	if !strings.Contains(all, "go (guessed)") {
		t.Errorf("the guess must be labelled:\n%s", all)
	}
	colored := false
	for _, l := range d.Lines {
		if strings.Contains(l, "\x1b[94m") { // keyword color
			colored = true
		}
	}
	if !colored {
		t.Error("a guessed block must be syntax highlighted")
	}

	d = render(t, md, 80, func(c *Config) { c.GuessCodeLang = false })
	if all := strings.Join(d.Plain, "\n"); strings.Contains(all, "guessed") || !strings.Contains(all, "text") {
		t.Errorf("with the setting off the block stays plain text:\n%s", all)
	}

	d = render(t, "```python\n"+strings.Repeat("x = 1\n", 3)+"```\n", 80, nil)
	if all := strings.Join(d.Plain, "\n"); strings.Contains(all, "guessed") {
		t.Errorf("a declared language must never be marked as guessed:\n%s", all)
	}
}
