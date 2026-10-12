package main

// Many sites highlight code at build time and ship no language at all (Shiki,
// Torchlight and friends leave only styled spans). wr's rule is never to color
// by a wrong label, so chroma's own Analyse (which called Go "GDScript3" on 13
// of 16 blocks of a real article) is not used. guessLang is deliberately small:
// a handful of languages, each recognised by strong, independent signals, and a
// guess is returned only when one language clearly beats every other. Otherwise
// the block stays plain text. A guessed block is labelled "(guessed)" so the
// reader can tell it from one the page declared.

import (
	"encoding/json"
	"regexp"
	"strings"
)

type signal struct {
	re     *regexp.Regexp
	weight int
}

func sig(weight int, pattern string) signal {
	return signal{regexp.MustCompile(pattern), weight}
}

var guessSignals = map[string][]signal{
	"go": {
		sig(3, `(?m)^package\s+\w+\s*$`),
		sig(3, `(?m)^type\s+\w+\s+(struct|interface)\s*\{`),
		sig(3, `(?m)^func\s*\(\w+\s+\*?\w+(\[[^\]]*\])?\)\s*\w+\(`),
		sig(2, `\w+(,\s*\w+)*\s:=\s`),
		sig(2, `(?m)^import\s*\(\s*$`),
		sig(2, `(?m)^import\s+"[\w./-]+"`),
		sig(2, `\berr\s*!=\s*nil\b`),
		sig(1, `(?m)^func\s+\w+\(`),
		sig(1, `\bfmt\.\w+\(`),
		sig(1, `\[\]\w+\{`),
		sig(1, `\bmap\[\w+\]\w+`),
		sig(1, `(?m)^\s*go\s+func\(`),
	},
	"python": {
		sig(3, `(?m)^\s*def\s+\w+\(.*\)\s*(->\s*[^:]+)?:\s*$`),
		sig(3, `(?m)^\s*class\s+\w+(\(.*\))?:\s*$`),
		sig(3, `(?m)^from\s+[\w.]+\s+import\s+`),
		sig(2, `(?m)^import\s+[\w.]+(\s+as\s+\w+)?\s*$`),
		sig(2, `\bself\.\w+`),
		sig(1, `(?m)^\s*(if|elif|else|for|while|try|except|with)\b.*:\s*$`),
		sig(1, `\b(None|True|False)\b`),
		sig(1, `\bprint\(`),
	},
	"javascript": {
		sig(3, `(?m)^import\s+.+\s+from\s+['"]`),
		sig(2, `\b(const|let)\s+\w+\s*=`),
		sig(2, `(?m)^\s*(async\s+)?function\s*\w*\(`),
		sig(2, `\bconsole\.(log|error)\(`),
		sig(2, `\brequire\(['"]`),
		sig(2, `(?m)^export\s+(default|const|function|class|async)\b`),
		sig(1, `=>`),
		sig(1, `\s===?\s`),
	},
	"typescript": {
		sig(3, `(?m)^(export\s+)?(interface|type)\s+\w+(<[^>]*>)?\s*(=|\{)`),
		sig(2, `\b\w+\??:\s*(string|number|boolean|unknown|void|Promise<[^>]*>)\b`),
		sig(2, `\)\s*:\s*(string|number|boolean|void|Promise<[^>]*>)\s*(\{|=>)`),
		sig(2, `\b(const|let)\s+\w+\s*:\s*\w+`),
		sig(1, `\bas\s+(const|string|number|unknown)\b`),
	},
	"sql": {
		sig(3, `(?im)^\s*select\s.+\sfrom\s`),
		sig(3, `(?im)^\s*insert\s+into\s`),
		sig(3, `(?im)^\s*create\s+(table|index|view)\s`),
		sig(2, `(?im)^\s*(update\s+\w+\s+set|delete\s+from)\s`),
		sig(1, `(?i)\b(where|group\s+by|order\s+by|inner\s+join|left\s+join)\b`),
	},
	"rust": {
		sig(3, `\blet\s+mut\s`),
		sig(3, `(?m)^use\s+(std|crate|super)::`),
		sig(3, `\b(println|vec|format|panic)!\(`),
		sig(2, `(?m)^\s*(pub\s+)?fn\s+\w+(<[^>]*>)?\(.*\)\s*(->\s*[^{]+)?\{`),
		sig(2, `(?m)^\s*impl(<[^>]*>)?\s+\w+`),
		sig(1, `&(mut\s+)?\w+`),
	},
	"java": {
		sig(3, `\bSystem\.out\.print(ln)?\(`),
		sig(3, `(?m)^import\s+java(x)?\.`),
		sig(3, `(?m)^\s*public\s+(static\s+|final\s+|abstract\s+)*(class|interface|enum)\s+\w+`),
		sig(2, `(?m)^\s*(public|private|protected)\s+(static\s+)?[\w<>\[\]]+\s+\w+\(.*\)\s*(throws\s+[\w, ]+)?\{`),
		sig(1, `@Override\b`),
	},
	"csharp": {
		sig(3, `(?m)^using\s+System[\w.]*;`),
		sig(3, `\bConsole\.Write(Line)?\(`),
		sig(2, `(?m)^namespace\s+[\w.]+`),
		sig(2, `(?m)^\s*(public|private|internal)\s+(static\s+)?(async\s+)?(class|record|interface)\s+\w+`),
		sig(1, `\bvar\s+\w+\s*=\s*new\s`),
	},
	"bash": {
		sig(5, `(?m)\A#!/(usr/)?bin/(env\s+)?(ba|z)?sh\b`),
		sig(2, `(?m)^\$\s+\S+`),
		sig(2, `(?m)^\s*(sudo|apt(-get)?|brew|npm|npx|yarn|pnpm|pip3?|cargo|docker|kubectl|curl|wget|chmod|mkdir|export)\s+\S+`),
		sig(1, `(?m)^\s*(cd|ls|git|make|go\s+(run|build|test|get|install))\s`),
	},
}

// guessLang returns the language of an unlabeled code block, or "" when it
// cannot say with confidence.
func guessLang(code string) string {
	code = strings.TrimSpace(code)
	if len(code) < 20 || strings.Count(code, "\n") < 1 && len(code) < 40 {
		return ""
	}
	if (code[0] == '{' || code[0] == '[') && json.Valid([]byte(code)) && strings.Count(code, "\n") >= 1 {
		return "json"
	}
	best, second := "", 0
	bestScore := 0
	for lang, sigs := range guessSignals {
		score := 0
		for _, s := range sigs {
			if s.re.MatchString(code) {
				score += s.weight
			}
		}
		switch {
		case score > bestScore:
			best, second, bestScore = lang, bestScore, score
		case score > second:
			second = score
		}
	}
	// TypeScript is JavaScript plus types: both firing means TypeScript, but
	// only when the type signals alone are convincing.
	if best == "javascript" || best == "typescript" {
		ts, js := scoreOf(code, "typescript"), scoreOf(code, "javascript")
		switch {
		case ts >= 3 && ts+js >= 5:
			best, bestScore, second = "typescript", ts+js, 0
		case best == "typescript":
			return ""
		}
	}
	if bestScore < 4 || bestScore-second < 3 {
		return ""
	}
	return best
}

func scoreOf(code, lang string) int {
	n := 0
	for _, s := range guessSignals[lang] {
		if s.re.MatchString(code) {
			n += s.weight
		}
	}
	return n
}
