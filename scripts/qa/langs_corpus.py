#!/usr/bin/env python3
# Builds the language corpus: one alias per lexer, using chroma's official
# lexers/testdata sample when there is one and a hand-written snippet otherwise.
# Needs qa-out/chroma-lexers.tsv (see probe/lexers). Output: lang-corpus.json
import json
import os
import re
import subprocess
from pathlib import Path

ROOT = Path(os.environ.get("QA_ROOT", "qa-out")).resolve()


def chroma_testdata() -> Path:
    env = os.environ.get("CHROMA_TESTDATA")
    if env:
        return Path(env)
    cache = subprocess.run(["go", "env", "GOMODCACHE"], capture_output=True, text=True).stdout.strip()
    hits = sorted(Path(cache).glob("github.com/alecthomas/chroma/v2@*/lexers/testdata"))
    if not hits:
        raise SystemExit("chroma testdata not found; set CHROMA_TESTDATA")
    return hits[-1]


CHROMA = chroma_testdata()
samples = {p.name[: -len(".actual")]: p for p in CHROMA.glob("*.actual")}
lexers_tsv = Path(os.environ.get("CHROMA_LEXERS", ROOT / "chroma-lexers.tsv"))
if not lexers_tsv.exists():
    raise SystemExit(f"missing {lexers_tsv}; run: go run ./scripts/qa/probe/lexers > {lexers_tsv}")

# Snippets propios para lexers sin muestra (sintaxis realista).
HAND = {
    "C": '#include <stdio.h>\n\n// comment\nint main(void) {\n    char *s = "hello";\n    printf("%s %d\\n", s, 42);\n    return 0;\n}\n',
    "C++": '#include <iostream>\n\n// comment\ntemplate <typename T>\nT add(T a, T b) { return a + b; }\n\nint main() {\n    std::cout << add(1, 2) << std::endl;\n}\n',
    "C#": 'using System;\n\n// comment\nnamespace Demo {\n    class Program {\n        static void Main() {\n            string s = "hello";\n            Console.WriteLine($"<{s}> {42}");\n        }\n    }\n}\n',
    "Java": 'import java.util.List;\n\n// comment\npublic class Main {\n    public static void main(String[] args) {\n        List<String> xs = List.of("a", "b");\n        System.out.println(xs.size() + 42);\n    }\n}\n',
    "Kotlin": '// comment\nfun main() {\n    val xs = listOf("a", "b")\n    println(xs.size + 42)\n}\n\nclass Greeter(val name: String) {\n    fun greet() = "hello $name"\n}\n',
    "Swift": 'import Foundation\n\n// comment\nstruct Point {\n    var x: Double\n    var y: Double\n}\n\nlet p = Point(x: 1.5, y: 2.0)\nprint("point \\(p.x) \\(42)")\n',
    "Objective-C": '#import <Foundation/Foundation.h>\n\n// comment\nint main() {\n    @autoreleasepool {\n        NSString *s = @"hello";\n        NSLog(@"%@ %d", s, 42);\n    }\n    return 0;\n}\n',
    "PHP": '<?php\n// comment\nfunction greet(string $name): string {\n    return "hello $name";\n}\n\necho greet("world") . 42;\n',
    "Perl": "#!/usr/bin/perl\nuse strict;\nuse warnings;\n\n# comment\nmy $s = 'hello';\nmy @xs = (1, 2, 3);\nprintf \"%s %d\\n\", $s, scalar @xs;\n",
    "Lua": '-- comment\nlocal function fib(n)\n    if n < 2 then return n end\n    return fib(n - 1) + fib(n - 2)\nend\n\nprint(fib(10))\n',
    "SQL": "-- comment\nSELECT u.name, COUNT(*) AS orders\nFROM users u\nJOIN orders o ON o.user_id = u.id\nWHERE u.created_at > '2024-01-01'\nGROUP BY u.name\nORDER BY orders DESC\nLIMIT 10;\n",
    "PL/pgSQL": "CREATE OR REPLACE FUNCTION add_one(x integer) RETURNS integer AS $$\nBEGIN\n    -- comment\n    RETURN x + 1;\nEND;\n$$ LANGUAGE plpgsql;\n",
    "PL/SQL": "DECLARE\n    v_count NUMBER := 0; -- comment\nBEGIN\n    SELECT COUNT(*) INTO v_count FROM users WHERE active = 1;\n    DBMS_OUTPUT.PUT_LINE('count: ' || v_count);\nEND;\n/\n",
    "Transact-SQL": "-- comment\nSELECT TOP 10 name, COUNT(*) AS c\nFROM users WITH (NOLOCK)\nWHERE created_at > GETDATE() - 30\nGROUP BY name\nORDER BY c DESC;\nGO\n",
    "MySQL": "-- comment\nSELECT `name`, COUNT(*) AS `c`\nFROM `users`\nWHERE `created_at` > '2024-01-01'\nGROUP BY `name`\nORDER BY `c` DESC\nLIMIT 10;\n",
    "SQLite3": "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL);\n-- comment\nINSERT INTO users (name) VALUES ('ana');\nSELECT name FROM users WHERE id = 1;\n",
    "Elm": "module Main exposing (main)\n\nimport Html exposing (text)\n\n-- comment\ngreet : String -> String\ngreet name =\n    \"hello \" ++ name\n\nmain =\n    text (greet \"world\")\n",
    "F#": "// comment\nlet rec fib n =\n    if n < 2 then n\n    else fib (n - 1) + fib (n - 2)\n\n[<EntryPoint>]\nlet main _ =\n    printfn \"%d\" (fib 10)\n    0\n",
    "Clojure": "; comment\n(ns demo.core)\n\n(defn greet [name]\n  (str \"hello \" name))\n\n(defn -main []\n  (println (greet \"world\") (reduce + [1 2 3])))\n",
    "Common Lisp": "; comment\n(defun fib (n)\n  (if (< n 2) n\n      (+ (fib (- n 1)) (fib (- n 2)))))\n\n(format t \"~a~%\" (fib 10))\n",
    "Scheme": "; comment\n(define (fib n)\n  (if (< n 2) n\n      (+ (fib (- n 1)) (fib (- n 2)))))\n\n(display (fib 10))\n(newline)\n",
    "Racket": "#lang racket\n; comment\n(define (fib n)\n  (if (< n 2) n (+ (fib (- n 1)) (fib (- n 2)))))\n(displayln (fib 10))\n",
    "Prolog": "% comment\nfib(0, 0).\nfib(1, 1).\nfib(N, F) :- N > 1, N1 is N - 1, N2 is N - 2,\n             fib(N1, F1), fib(N2, F2), F is F1 + F2.\n",
    "Erlang": "% comment\n-module(demo).\n-export([fib/1]).\n\nfib(N) when N < 2 -> N;\nfib(N) -> fib(N - 1) + fib(N - 2).\n",
    "Fortran": "program main\n  implicit none\n  integer :: i\n  ! comment\n  do i = 1, 10\n    print *, 'i =', i ** 2\n  end do\nend program main\n",
    "Pascal": "program Demo;\n{ comment }\nvar\n  i: Integer;\nbegin\n  for i := 1 to 10 do\n    WriteLn('i = ', i * i);\nend.\n",
    "Ada": "with Ada.Text_IO; use Ada.Text_IO;\n\n-- comment\nprocedure Demo is\n   X : Integer := 42;\nbegin\n   Put_Line (\"value:\" & Integer'Image (X));\nend Demo;\n",
    "Groovy": "// comment\nclass Greeter {\n    String name\n    String greet() { \"hello $name\" }\n}\n\ndef g = new Greeter(name: 'world')\nprintln g.greet()\n",
    "Crystal": "# comment\nclass Greeter\n  def initialize(@name : String)\n  end\n\n  def greet\n    \"hello #{@name}\"\n  end\nend\n\nputs Greeter.new(\"world\").greet\n",
    "Nim": "# comment\nproc fib(n: int): int =\n  if n < 2: n\n  else: fib(n - 1) + fib(n - 2)\n\necho fib(10)\n",
    "V": "// comment\nfn fib(n int) int {\n\tif n < 2 { return n }\n\treturn fib(n - 1) + fib(n - 2)\n}\n\nfn main() {\n\tprintln(fib(10))\n}\n",
    "D": "import std.stdio;\n\n// comment\nvoid main() {\n    auto xs = [1, 2, 3];\n    writeln(\"sum: \", xs.sum, 42);\n}\n",
    "Haxe": "// comment\nclass Main {\n    static function main() {\n        var xs = [1, 2, 3];\n        trace('sum: ' + xs.length);\n    }\n}\n",
    "Zig": "const std = @import(\"std\");\n\n// comment\npub fn main() !void {\n    const stdout = std.io.getStdOut().writer();\n    try stdout.print(\"hello {d}\\n\", .{42});\n}\n",
    "Odin": "package main\n\nimport \"core:fmt\"\n\n// comment\nmain :: proc() {\n\txs := []int{1, 2, 3}\n\tfmt.println(\"len:\", len(xs))\n}\n",
    "Assembly (x86-64 ASM)": ".section .data\nmsg: .asciz \"hello\"\n\n# comment\n.global _start\n_start:\n    mov $1, %rax\n    mov $1, %rdi\n    lea msg(%rip), %rsi\n    mov $5, %rdx\n    syscall\n",
    "MASM": ".data\nmsg db \"hello\", 0\n\n; comment\n.code\nmain PROC\n    mov eax, 42\n    ret\nmain ENDP\nEND\n",
    "NASM": "section .data\nmsg db 'hello', 10\n\n; comment\nsection .text\nglobal _start\n_start:\n    mov rax, 1\n    mov rdi, 1\n    mov rsi, msg\n    mov rdx, 6\n    syscall\n",
    "RISC-V Assembly": ".text\n.globl main\n# comment\nmain:\n    li a0, 42\n    li a7, 93\n    ecall\n",
    "VHDL": "-- comment\nlibrary IEEE;\nuse IEEE.STD_LOGIC_1164.ALL;\n\nentity demo is\n    Port ( clk : in STD_LOGIC; q : out STD_LOGIC );\nend demo;\n\narchitecture Behavioral of demo is\nbegin\n    q <= clk;\nend Behavioral;\n",
    "Verilog": "// comment\nmodule counter(input clk, output reg [7:0] q);\n    always @(posedge clk) begin\n        q <= q + 1'b1;\n    end\nendmodule\n",
    "SystemVerilog": "// comment\nmodule counter(input logic clk, output logic [7:0] q);\n    always_ff @(posedge clk) q <= q + 8'd1;\nendmodule\n",
    "TOML": "[package]\nname = \"demo\"\nversion = \"1.0.0\"\n\n[dependencies]\nserde = { version = \"1\", features = [\"derive\"] }\n\n# comment\n",
    "INI": "[section]\nkey = value\n; comment\nnumber = 42\nflag = true\n",
    "XML": '<?xml version="1.0" encoding="UTF-8"?>\n<!-- comment -->\n<catalog>\n  <book id="1"><title>Demo</title></book>\n</catalog>\n',
    "DTD": '<!ELEMENT catalog (book*)>\n<!ELEMENT book (title)>\n<!ATTLIST book id ID #REQUIRED>\n',
    "Protobuf": 'syntax = "proto3";\n\npackage demo;\n\n// comment\nmessage User {\n  int64 id = 1;\n  string name = 2;\n  repeated string tags = 3;\n}\n\nservice Users {\n  rpc Get(User) returns (User);\n}\n',
    "Thrift": "namespace go demo\n\n// comment\nstruct User {\n  1: i64 id,\n  2: string name,\n}\n\nservice Users {\n  User get(1: i64 id),\n}\n",
    "GraphQL": "# comment\nquery User($id: ID!) {\n  user(id: $id) {\n    name\n    posts(first: 10) { title }\n  }\n}\n\ntype User {\n  id: ID!\n  name: String!\n}\n",
    "Docker": "FROM golang:1.24 AS build\nWORKDIR /src\n\n# comment\nCOPY . .\nRUN go build -o /out/app ./cmd/app\n\nFROM gcr.io/distroless/static\nCOPY --from=build /out/app /app\nENTRYPOINT [\"/app\"]\n",
    "Makefile": "CC = gcc\nCFLAGS = -Wall -O2\n\n# comment\nall: app\n\napp: main.o util.o\n\t$(CC) $(CFLAGS) -o $@ $^\n\n%.o: %.c\n\t$(CC) $(CFLAGS) -c $<\n\nclean:\n\trm -f *.o app\n",
    "Nix": "{ pkgs ? import <nixpkgs> {} }:\n\n# comment\npkgs.stdenv.mkDerivation {\n  pname = \"demo\";\n  version = \"1.0\";\n  src = ./.;\n  buildInputs = [ pkgs.go pkgs.git ];\n}\n",
    "Puppet": "# comment\nclass nginx {\n  package { 'nginx':\n    ensure => installed,\n  }\n  service { 'nginx':\n    ensure => running,\n  }\n}\n",
    "Salt": "nginx:\n  pkg.installed: []\n  # comment\n  service.running:\n    - enable: True\n",
    "Ansible": "- name: Configure web servers\n  hosts: webservers\n  become: true\n  tasks:\n    # comment\n    - name: Install nginx\n      apt:\n        name: nginx\n        state: present\n",
    "HCL": 'resource "aws_instance" "web" {\n  ami           = "ami-123456"\n  instance_type = "t3.micro"\n\n  # comment\n  tags = {\n    Name = "web"\n  }\n}\n',
    "JSONata": '(\n  $users := [{"name": "ana", "age": 30}];\n  /* comment */\n  $users[age > 18].name\n)\n',
    "Bicep": "param location string = resourceGroup().location\n\n// comment\nresource stg 'Microsoft.Storage/storageAccounts@2023-01-01' = {\n  name: 'demo'\n  location: location\n  sku: { name: 'Standard_LRS' }\n  kind: 'StorageV2'\n}\n",
    "Cap'n Proto": "@0xabcdef1234567890;\n\n# comment\nstruct User {\n  id @0 :Int64;\n  name @1 :Text;\n}\n\ninterface Users {\n  get @0 (id :Int64) -> (user :User);\n}\n",
    "AWK": "#!/usr/bin/awk -f\n# comment\nBEGIN { FS = \",\" }\nNR > 1 { total += $3 }\nEND { printf \"total: %d\\n\", total }\n",
    "Sed": "# comment\ns/foo/bar/g\ns/^[ \\t]*//\n1,10d\n",
    "Batchfile": "@echo off\nREM comment\nsetlocal\nset NAME=world\necho hello %NAME%\nif exist out.txt del out.txt\nendlocal\n",
    "PowerShell": "# comment\nparam([string]$Name = 'world')\n$xs = 1..10 | Where-Object { $_ % 2 -eq 0 }\nWrite-Output \"hello $Name: $($xs.Count)\"\n",
    "Objective-J": "// comment\n@import <Foundation/CPString.j>\n\n@implementation Greeter : CPObject\n- (CPString)greet { return @\"hello\"; }\n@end\n",
    "Forth": "\\ comment\n: fib ( n -- n )\n  dup 2 < if exit then\n  dup 1- recurse swap 2 - recurse + ;\n\n10 fib . cr\n",
    "Coq": "(* comment *)\nRequire Import Arith.\n\nFixpoint fib (n : nat) : nat :=\n  match n with\n  | 0 => 0\n  | 1 => 1\n  | S (S n' as m) => fib m + fib n'\n  end.\n",
    "Lean": "-- comment\nimport Mathlib.Data.Nat.Basic\n\ndef fib : Nat -> Nat\n  | 0 => 0\n  | 1 => 1\n  | n + 2 => fib (n + 1) + fib n\n",
    "Agda": "-- comment\nmodule Demo where\n\nopen import Data.Nat\n\nfib : ℕ → ℕ\nfib 0 = 0\nfib 1 = 1\nfib (suc (suc n)) = fib (suc n) + fib n\n",
    "EmacsLisp": ";; comment\n(defun fib (n)\n  (if (< n 2) n\n      (+ (fib (- n 1)) (fib (- n 2)))))\n\n(message \"%d\" (fib 10))\n",
    "Gnuplot": "# comment\nset terminal png\nset output 'plot.png'\nplot sin(x) with lines title 'sin', cos(x) title 'cos'\n",
    "Zsh": "#!/bin/zsh\n# comment\ntypeset -a xs=(1 2 3)\nfor x in $xs; do\n  print -- \"$x\"\ndone\n",
    "Fish": "#!/usr/bin/env fish\n# comment\nset -l xs 1 2 3\nfor x in $xs\n    echo $x\nend\n",
    "Tcsh": "#!/bin/tcsh\n# comment\nset xs = (1 2 3)\nforeach x ( $xs )\n  echo $x\nend\n",
    "ShellSession": "$ go version\ngo version go1.24 linux/amd64\n$ echo \"hello\" | wc -c\n6\n",
    "Typst": "#set page(width: 10cm)\n// comment\n= Heading\n\nThis is *strong* and _emphasized_.\n\n#let x = 42\n",
    "AsciiDoc": "= Document Title\n:author: Me\n\n== Section\n\nA *bold* word and `code`.\n\n* item one\n* item two\n",
    "RST": "Title\n=====\n\n.. comment\n\nSection\n-------\n\nSome **bold** and ``code`` text.\n\n* item\n",
    "Org Mode": "#+TITLE: Demo\n\n* Heading\n\nSome text with *bold* and =code=.\n\n- item one\n- item two\n",
    "reStructuredText": "Demo\n====\n\n.. note::\n   A note here.\n\n::\n\n    indented code\n",
    "Gettext": 'msgid ""\nmsgstr ""\n"Content-Type: text/plain; charset=UTF-8\\n"\n\n#. comment\nmsgid "hello"\nmsgstr "hola"\n',
    "Pkl": "amends \"pkl:base\"\n\n/// comment\nname = \"demo\"\nport = 8080\n",
    "CUE": "package demo\n\n// comment\n#User: {\n\tname: string\n\tage:  int\n}\n\nuser: #User & {name: \"ana\", age: 30}\n",
    "Dhall": "-- comment\nlet User = { name : Text, age : Natural }\nin { name = \"ana\", age = 30 } : User\n",
    "Jsonnet": "// comment\nlocal greet(name) = 'hello ' + name;\n{\n  message: greet('world'),\n  count: std.length([1, 2, 3]),\n}\n",
    "KDL": "// comment\nperson \"ana\" {\n  age 30\n  email \"ana@example.com\"\n}\n",
    "Protocol Buffer Text Format": 'name: "ana"\nage: 30\ntags: ["a", "b"]\n',
    "CSV": 'name,age,city\nana,30,madrid\nluis,25,seville\n',
    "Diff": "--- a/main.go\n+++ b/main.go\n@@ -1,5 +1,5 @@\n package main\n \n-func main() { println(\"old\") }\n+func main() { println(\"new\") }\n",
    "Hexdump": "00000000  7f 45 4c 46 02 01 01 00  00 00 00 00 00 00 00 00  |.ELF............|\n00000010  03 00 3e 00 01 00 00 00  50 10 00 00 00 00 00 00  |..>.........P...|\n",
    "GAS": ".text\n.globl main\n# comment\nmain:\n    movq $42, %rax\n    ret\n",
    "Groff": ".TH DEMO 1\n.SH NAME\ndemo \\- show things\n.SH SYNOPSIS\n.B demo\n[\\fIoptions\\fR]\n",
    "Markdown": "# Title\n\n**bold** and `code` and [link](https://example.com).\n\n- item one\n- item two\n\n```go\nfmt.Println(\"hi\")\n```\n",
    "Text only": "Just some plain text.\nSecond line with punctuation: yes, sir!\n",
    "Bash": "#!/usr/bin/env bash\nset -euo pipefail\n\n# comment\nfor f in *.txt; do\n  echo \"$f\"\ndone\n",
    "Ksh": "#!/bin/ksh\n# comment\nfor f in *.txt; do\n  print -- \"$f\"\ndone\n",
    "Nu": "# comment\nls | where size > 10kb | sort-by size --reverse | first 5\n",
    "Haskell": "-- comment\nfib :: Int -> Int\nfib n\n  | n < 2 = n\n  | otherwise = fib (n - 1) + fib (n - 2)\n\nmain :: IO ()\nmain = print (fib 10)\n",
    "OCaml": "(* comment *)\nlet rec fib n =\n  if n < 2 then n else fib (n - 1) + fib (n - 2)\n\nlet () = Printf.printf \"%d\\n\" (fib 10)\n",
    "Scala": "// comment\nobject Main extends App {\n  def fib(n: Int): Int = if (n < 2) n else fib(n - 1) + fib(n - 2)\n  println(fib(10))\n}\n",
    "Eiffel": "class DEMO\n\n-- comment\nfeature\n\n  greet (name: STRING): STRING\n    do\n      Result := \"hello \" + name\n    end\nend\n",
    "Smalltalk": "\"comment\"\n| xs sum |\nxs := #(1 2 3).\nsum := xs inject: 0 into: [:a :b | a + b].\nTranscript show: sum printString.\n",
    "Tcl": "# comment\nproc fib {n} {\n    if {$n < 2} { return $n }\n    return [expr {[fib [expr {$n-1}]] + [fib [expr {$n-2}]]}]\n}\nputs [fib 10]\n",
    "AutoHotkey": "; comment\n#Requires AutoHotkey v2.0\nF1::MsgBox \"hello\"\n^j::Send \"world\"\n",
    "AutoIt": "; comment\n#include <MsgBoxConstants.au3>\n\nFunc Greet($name)\n    MsgBox($MB_OK, \"demo\", \"hello \" & $name)\nEndFunc\n",
    "AppleScript": "-- comment\ntell application \"Finder\"\n    set theFiles to every file of desktop\n    return count of theFiles\nend tell\n",
    "Groovy (Gradle)": "plugins {\n    id 'java'\n}\n\n// comment\ndependencies {\n    implementation 'com.google.guava:guava:33.0.0-jre'\n    testImplementation 'junit:junit:4.13.2'\n}\n",
    "Maven POM": '<?xml version="1.0"?>\n<project>\n  <!-- comment -->\n  <groupId>com.example</groupId>\n  <artifactId>demo</artifactId>\n  <version>1.0</version>\n</project>\n',
    "CMake": "cmake_minimum_required(VERSION 3.20)\nproject(demo C)\n\n# comment\nadd_executable(demo main.c)\ntarget_compile_options(demo PRIVATE -Wall -O2)\n",
    "Meson": "project('demo', 'c')\n\n# comment\nexecutable('demo', 'main.c',\n  c_args: ['-Wall', '-O2'])\n",
    "Bazel": "load(\"@rules_go//go:def.bzl\", \"go_binary\")\n\n# comment\ngo_binary(\n    name = \"app\",\n    srcs = [\"main.go\"],\n)\n",
    "Buck": "cxx_binary(\n  name = \"app\",\n  # comment\n  srcs = [\"main.cpp\"],\n  deps = [\":lib\"],\n)\n",
    "Terraform": 'resource "aws_s3_bucket" "b" {\n  bucket = "my-bucket"\n\n  # comment\n  tags = {\n    Environment = "dev"\n  }\n}\n',
    "PEG": "start <- expr\nexpr  <- term ('+' term)*\nterm  <- [0-9]+\n",
    "EBNF": "expr   = term, { \"+\", term } ;\nterm   = factor, { \"*\", factor } ;\nfactor = number | \"(\", expr, \")\" ;\n",
    "ABNF": "request = method SP request-target SP HTTP-version CRLF\nmethod  = \"GET\" / \"POST\" / \"PUT\"\nSP      = %x20\n",
    "BNF": "<expr> ::= <term> | <expr> \"+\" <term>\n<term> ::= <factor> | <term> \"*\" <factor>\n<factor> ::= <number> | \"(\" <expr> \")\"\n",
    "Mermaid": "graph TD\n    A[Start] --> B{OK?}\n    B -->|yes| C[Done]\n    B -->|no| D[Retry]\n",
    "PlantUML": "@startuml\nAlice -> Bob: hello\nBob --> Alice: hi\n@enduml\n",
    "Graphviz DOT": "digraph G {\n    rankdir=LR;\n    a -> b [label=\"edge\"];\n    b -> c;\n    c -> a;\n}\n",
    "SQL (SQLite)": "SELECT name, age FROM users WHERE age > 18 ORDER BY age DESC;\n",
    "TeX": "\\documentclass{article}\n\\begin{document}\n% comment\nHello, \\textbf{world}. $E = mc^2$\n\\end{document}\n",
    "LaTeX": "\\section{Introduction}\n% comment\nAs shown in \\cite{smith2024}, $\\alpha + \\beta = \\gamma$.\n",
    "BibTeX": "@article{smith2024,\n  author = {Smith, John},\n  title = {A Title},\n  journal = {J. Demo},\n  year = {2024},\n  volume = {42},\n}\n",
    "Typst Math": "$ integral_0^1 x^2 dif x = 1/3 $\n",
    "VimL": '" comment\nfunction! Fib(n) abort\n  if a:n < 2 | return a:n | endif\n  return Fib(a:n - 1) + Fib(a:n - 2)\nendfunction\nnnoremap <leader>f :echo Fib(10)<CR>\n',
    "Vim Ex": ":set number\n:e main.go\n:%s/foo/bar/g\n:wq\n",
    "Kakoune": "# comment\ndef fib -params 1 %{\n  %arg{1} 2 <select> %{}\n}\n",
    "Nginx": "server {\n    listen 80;\n    server_name example.com;\n\n    # comment\n    location / {\n        root /var/www/html;\n        try_files $uri $uri/ /index.html;\n    }\n}\n",
    "ApacheConf": "<VirtualHost *:80>\n    ServerName example.com\n    # comment\n    DocumentRoot /var/www/html\n    <Directory /var/www/html>\n        Require all granted\n    </Directory>\n</VirtualHost>\n",
    "Caddyfile": "example.com {\n\t# comment\n\troot * /var/www/html\n\tfile_server\n\treverse_proxy /api localhost:8080\n}\n",
    "HAProxy": "global\n    maxconn 2048\n\ndefaults\n    timeout connect 5s\n\n# comment\nfrontend http\n    bind *:80\n    default_backend app\n",
    "Systemd": "[Unit]\nDescription=Demo service\n\n[Service]\n# comment\nExecStart=/usr/local/bin/demo --flag\nRestart=on-failure\n\n[Install]\nWantedBy=multi-user.target\n",
    "INI (Desktop Entry)": "[Desktop Entry]\nType=Application\nName=Demo\nExec=demo %F\nIcon=demo\nCategories=Utility;\n",
    "Rego": "package authz\n\n# comment\ndefault allow = false\n\nallow {\n    input.user == \"admin\"\n}\n",
    "Cue (JSON)": '{"name": "demo", "port": 8080}\n',
    "Kustomize": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n\n# comment\nresources:\n  - deployment.yaml\npatches:\n  - path: patch.yaml\n",
    "Kubernetes YAML": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: demo\nspec:\n  replicas: 3\n  template:\n    spec:\n      containers:\n        - name: app\n          image: demo:1.0\n",
    "OpenAPI": "openapi: 3.0.3\ninfo:\n  title: Demo\n  version: 1.0.0\npaths:\n  /users:\n    get:\n      responses:\n        '200':\n          description: OK\n",
    "AsyncAPI": "asyncapi: 3.0.0\ninfo:\n  title: Demo\n  version: 1.0.0\nchannels:\n  user.created:\n    messages:\n      UserCreated:\n        payload:\n          type: object\n",
    "Solidity": "// SPDX-License-Identifier: MIT\npragma solidity ^0.8.20;\n\n// comment\ncontract Counter {\n    uint256 public count;\n\n    function inc() external {\n        count += 1;\n    }\n}\n",
    "Move": "module 0x1::counter {\n    // comment\n    struct Counter has key { value: u64 }\n\n    public fun increment(c: &mut Counter) {\n        c.value = c.value + 1;\n    }\n}\n",
    "Cairo": "# comment\nfunc fib(n: felt) -> (res: felt) {\n    if n == 0 {\n        return (res=0);\n    }\n    return (res=n);\n}\n",
    "Vyper": "# @version ^0.4.0\n# comment\ncount: public(uint256)\n\n@external\ndef increment():\n    self.count += 1\n",
    "Rust (no sample)": "// comment\nfn main() {\n    let xs = vec![1, 2, 3];\n    println!(\"sum: {}\", xs.iter().sum::<i32>());\n}\n",
    "Go (no sample)": "package main\n\nimport \"fmt\"\n\n// comment\nfunc main() {\n\txs := []int{1, 2, 3}\n\tfmt.Println(len(xs))\n}\n",
    "Python (no sample)": "# comment\nfrom dataclasses import dataclass\n\n@dataclass\nclass Point:\n    x: float\n    y: float\n\nprint(sum([1, 2, 3]))\n",
    "JavaScript (no sample)": "// comment\nconst add = (a, b) => a + b;\nconsole.log(add(1, 2));\n\nasync function main() {\n  return await fetch('/api').then(r => r.json());\n}\n",
    "TypeScript (no sample)": "// comment\ninterface User { id: number; name: string }\n\nconst greet = (u: User): string => `hello ${u.name}`;\n\nconsole.log(greet({ id: 1, name: 'ana' }));\n",
    "Java (no sample)": "// comment\npublic record User(long id, String name) {}\n",
    "Kotlin DSL": "plugins {\n    kotlin(\"jvm\") version \"2.0.0\"\n}\n\n// comment\ndependencies {\n    implementation(\"org.jetbrains.kotlinx:kotlinx-coroutines-core:1.9.0\")\n}\n",
    "Svelte": "<script>\n  // comment\n  let count = $state(0);\n</script>\n\n<button onclick={() => count++}>\n  clicks: {count}\n</button>\n\n<style>\n  button { color: red; }\n</style>\n",
    "Vue": "<script setup>\n// comment\nimport { ref } from 'vue';\nconst count = ref(0);\n</script>\n\n<template>\n  <button @click=\"count++\">clicks: {{ count }}</button>\n</template>\n\n<style scoped>\nbutton { color: red; }\n</style>\n",
    "JSX": "// comment\nfunction App({ name }) {\n  return <h1 className=\"title\">hello {name}</h1>;\n}\n\nexport default App;\n",
    "TSX": "// comment\ntype Props = { name: string };\n\nexport const App = ({ name }: Props) => <h1>hello {name}</h1>;\n",
    "HTML": "<!doctype html>\n<!-- comment -->\n<html lang=\"en\">\n  <head><title>Demo</title><meta charset=\"utf-8\"></head>\n  <body>\n    <h1 class=\"title\">Hello</h1>\n    <script>console.log(\"hi\")</script>\n  </body>\n</html>\n",
    "CSS": "/* comment */\n:root { --accent: #3b82f6; }\n\n.card {\n  display: flex;\n  gap: 1rem;\n  color: var(--accent);\n}\n\n@media (max-width: 640px) {\n  .card { flex-direction: column; }\n}\n",
    "SCSS": "// comment\n$accent: #3b82f6;\n\n@mixin card($pad: 1rem) {\n  padding: $pad;\n  border: 1px solid darken($accent, 10%);\n}\n\n.card {\n  @include card(1.5rem);\n  &:hover { color: $accent; }\n}\n",
    "Sass": "// comment\n$accent: #3b82f6\n\n.card\n  padding: 1rem\n  &:hover\n    color: $accent\n",
    "Less": "// comment\n@accent: #3b82f6;\n\n.card {\n  padding: 1rem;\n  &:hover { color: @accent; }\n}\n",
    "Stylus": "// comment\naccent = #3b82f6\n\n.card\n  padding 1rem\n  &:hover\n    color accent\n",
    "PostCSS": "/* comment */\n.card {\n  color: color-mix(in srgb, red 40%, blue);\n}\n",
    "Tailwind CSS (directives)": "@tailwind base;\n@tailwind components;\n/* comment */\n@tailwind utilities;\n\n@layer components {\n  .btn { @apply rounded px-3 py-1 bg-blue-600 text-white; }\n}\n",
    "Handlebars": "<!-- comment -->\n<h1>{{title}}</h1>\n<ul>\n  {{#each items}}\n    <li>{{this.name}}</li>\n  {{/each}}\n</ul>\n",
    "Mustache": "Hello {{name}},\nYou have {{count}} new messages.\n{{#items}}* {{.}}{{/items}}\n",
    "Jinja": "{% extends \"base.html\" %}\n{# comment #}\n{% block content %}\n  <ul>\n  {% for item in items %}\n    <li>{{ item.name|e }}</li>\n  {% endfor %}\n  </ul>\n{% endblock %}\n",
    "Django": "{% block title %}Demo{% endblock %}\n{# comment #}\n{% for item in items %}\n  <li>{{ item.name }}</li>\n{% empty %}\n  <li>none</li>\n{% endfor %}\n",
    "ERB": "<%# comment %>\n<ul>\n  <% @items.each do |item| %>\n    <li><%= item.name %></li>\n  <% end %>\n</ul>\n",
    "EJS": "<%# comment %>\n<ul>\n  <% items.forEach(function(item) { %>\n    <li><%= item.name %></li>\n  <% }); %>\n</ul>\n",
    "Liquid": "{% comment %}comment{% endcomment %}\n<ul>\n  {% for item in items %}\n    <li>{{ item.name | escape }}</li>\n  {% endfor %}\n</ul>\n",
    "Twig": "{# comment #}\n<ul>\n  {% for item in items %}\n    <li>{{ item.name|e }}</li>\n  {% endfor %}\n</ul>\n",
    "Blade": "{{-- comment --}}\n<ul>\n  @foreach ($items as $item)\n    <li>{{ $item->name }}</li>\n  @endforeach\n</ul>\n",
    "Astro": "---\n// comment\nconst { title } = Astro.props;\n---\n\n<h1>{title}</h1>\n\n<style>\n  h1 { color: teal; }\n</style>\n",
    "Marko": "// comment\n$ const name = 'world';\n<h1>hello ${name}</h1>\n",
    "Nunjucks": "{# comment #}\n{% for item in items %}\n  <li>{{ item.name }}</li>\n{% endfor %}\n",
    "Pug": "//- comment\nul\n  each item in items\n    li= item.name\n",
    "Slim": "/ comment\nul\n  - items.each do |item|\n    li = item.name\n",
    "Haml": "-# comment\n%ul\n  - items.each do |item|\n    %li= item.name\n",
    "DjHTML": "<div class=\"{% if active %}active{% endif %}\">{{ value }}</div>\n",
    "Mako": "<%doc>comment</%doc>\n<ul>\n% for item in items:\n  <li>${item.name}</li>\n% endfor\n</ul>\n",
    "Genshi": "<ul xmlns:py=\"http://genshi.edgewall.org/\">\n  <li py:for=\"item in items\">${item.name}</li>\n</ul>\n",
    "Cheetah": "#comment\n#for $item in $items\n  $item.name\n#end for\n",
    "Velocity": "#* comment *#\n#foreach($item in $items)\n  $item.name\n#end\n",
    "FreeMarker": "<#-- comment -->\n<#list items as item>\n  ${item.name}\n</#list>\n",
    "Thymeleaf": "<ul th:if=\"${#lists.size(items) > 0}\">\n  <li th:each=\"item : ${items}\" th:text=\"${item.name}\">x</li>\n</ul>\n",
    "Razor": "@* comment *@\n<ul>\n@foreach (var item in Model.Items) {\n  <li>@item.Name</li>\n}\n</ul>\n",
    "JSX (Preact)": "// comment\nconst App = () => <main><h1>hello</h1></main>;\n",
    "React": "// comment\nexport function Button({ label, onClick }) {\n  return <button onClick={onClick}>{label}</button>;\n}\n",
    "Angular2": "// comment\n@Component({\n  selector: 'app-root',\n  template: '<h1>{{title}}</h1>',\n})\nexport class AppComponent {\n  title = 'demo';\n}\n",
    "HTML+PHP": '<!-- comment -->\n<ul>\n<?php foreach ($items as $item): ?>\n  <li><?= htmlspecialchars($item->name) ?></li>\n<?php endforeach; ?>\n</ul>\n',
    "Regular Expressions": "^[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$\n(?:foo|bar)+?(?=baz)\n",
    "JSON (JSON5)": "{\n  // comment\n  name: 'demo',\n  port: 8080,\n  tags: ['a', 'b',],\n}\n",
    "YAML (multi-doc)": "---\n# comment\nname: demo\nports: [80, 443]\n---\nname: other\n",
    "JSON Lines": '{"event": "start", "ts": 1735689600}\n{"event": "stop", "ts": 1735689605}\n',
    "NDJSON": '{"id": 1, "ok": true}\n{"id": 2, "ok": false}\n',
    "Log File": "2026-10-08T12:00:00Z INFO  starting server on :8080\n2026-10-08T12:00:01Z WARN  slow query: 1.2s\n2026-10-08T12:00:02Z ERROR connection refused\n",
    "Iptables": "# comment\niptables -A INPUT -p tcp --dport 22 -j ACCEPT\niptables -A INPUT -j DROP\n",
    "Nasm (variant)": "section .text\nglobal _start\n_start:\n    xor eax, eax\n    ret\n",
    "Objective C++": "// comment\n#include <string>\nint main() {\n    std::string s = @\"hello\";\n    return s.size();\n}\n",
    "MATLAB": "% comment\nx = linspace(0, 2*pi, 100);\ny = sin(x) + 0.1 * randn(size(x));\nplot(x, y, 'LineWidth', 1.5);\n",
    "Octave": "% comment\nA = [1 2; 3 4];\nb = [5; 6];\nx = A \\ b;\nprintf('x = [%f, %f]\\n', x(1), x(2));\n",
    "Scilab": "// comment\nA = [1 2; 3 4];\nb = [5; 6];\nx = A \\ b;\ndisp(x);\n",
    "R": "# comment\nlibrary(ggplot2)\ndf <- data.frame(x = 1:10, y = (1:10)^2)\nggplot(df, aes(x, y)) + geom_line() + theme_minimal()\n",
    "Julia": "# comment\nusing LinearAlgebra\nA = [1.0 2.0; 3.0 4.0]\nx = A \\ [5.0, 6.0]\nprintln(sum(x))\n",
    "Wolfram": "(* comment *)\nTable[Fibonacci[n], {n, 1, 10}]\nPlot[Sin[x], {x, 0, 2 Pi}]\n",
    "Maxima": "/* comment */\nf(x) := x^2 + 2*x + 1;\ndiff(f(x), x);\n",
    "SAS": "/* comment */\ndata work.demo;\n  set sashelp.class;\n  bmi = weight / (height/100)**2;\nrun;\nproc print data=work.demo (obs=10); run;\n",
    "Stata": "* comment\nsysuse auto, clear\nregress price mpg weight\nsummarize price if foreign == 1\n",
    "SPSS": "* comment.\nGET FILE='demo.sav'.\nFREQUENCIES VARIABLES=age gender.\n",
    "GAMS": "$comment\nSet i /1*3/;\nParameter p(i) /1 10, 2 20, 3 30/;\nVariable x(i);\n",
    "Modelica": "// comment\nmodel Demo\n  Real x(start = 1.0);\n  parameter Real k = 2.0;\nequation\n  der(x) = -k * x;\nend Demo;\n",
    "Dyalog APL": "⍝ comment\na ← 1 2 3 4 5\n+/ a\n2 ×⍳ 5\n",
    "J": "NB. comment\n+/ 1 2 3 4 5\n2 * i. 5\n",
    "Q (kdb+)": "/ comment\nq)t:([] sym:`a`b; px:1.5 2.5)\nq)select avg px by sym from t\n",
    "K": "/ comment\nfib:{$[x<2;x;fib[x-1]+fib[x-2]]}\nfib 10\n",
    "Raku": "# comment\nsub fib(Int $n) {\n    return $n if $n < 2;\n    fib($n - 1) + fib($n - 2)\n}\nsay fib(10);\n",
    "Prolog (SWI)": "% comment\n:- use_module(library(lists)).\n\nsum_list([], 0).\nsum_list([H|T], S) :- sum_list(T, S0), S is S0 + H.\n",
    "Mercury": "% comment\n:- module demo.\n:- interface.\n:- import_module io.\n:- pred main(io::di, io::uo) is det.\n",
    "Oz": "% comment\ndecl\nfun {Fib N}\n   if N < 2 then N else {Fib N-1} + {Fib N-2} end\nend\n{Show {Fib 10}}\n",
    "Standard ML": "(* comment *)\nfun fib n = if n < 2 then n else fib (n - 1) + fib (n - 2)\nval () = print (Int.toString (fib 10) ^ \"\\n\")\n",
    "CoffeeScript": "# comment\nsquare = (x) -> x * x\nnums = (square n for n in [1..10])\nconsole.log nums.reduce (a, b) -> a + b\n",
    "FSharp": "// comment\nlet rec fib n = if n < 2 then n else fib (n-1) + fib (n-2)\n[<EntryPoint>]\nlet main _ = printfn \"%d\" (fib 10); 0\n",
    "GLSL": "#version 330 core\n// comment\nlayout(location = 0) in vec3 pos;\nuniform mat4 mvp;\nvoid main() { gl_Position = mvp * vec4(pos, 1.0); }\n",
    "COBOL": "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. DEMO.\n      * comment\n       DATA DIVISION.\n       WORKING-STORAGE SECTION.\n       01 WS-NUM PIC 9(3) VALUE 42.\n       PROCEDURE DIVISION.\n           DISPLAY 'NUM: ' WS-NUM.\n           STOP RUN.\n",
    "ActionScript": "// comment\npackage {\n    public class Greeter {\n        public function greet(name:String):String {\n            return \"hello \" + name;\n        }\n    }\n}\n",
    "APL": "⍝ comment\na ← 1 2 3 4 5\n+/ a\n2 ×⍳ 5\n",
    "Mathematica": "(* comment *)\nfib[n_] := If[n < 2, n, fib[n-1] + fib[n-2]]\nTable[fib[n], {n, 1, 10}]\n",
    "SPARQL": "# comment\nPREFIX foaf: <http://xmlns.com/foaf/0.1/>\nSELECT ?name WHERE {\n  ?p a foaf:Person ; foaf:name ?name .\n} LIMIT 10\n",
    "Turtle": "@prefix foaf: <http://xmlns.com/foaf/0.1/> .\n# comment\n<#ana> a foaf:Person ;\n    foaf:name \"Ana\" .\n",
    "Pig": "-- comment\nusers = LOAD 'users.csv' USING PigStorage(',') AS (id:int, name:chararray);\nactive = FILTER users BY id > 100;\nSTORE active INTO 'out' USING PigStorage(',');\n",
    "SourcePawn": "// comment\npublic void OnPluginStart() {\n    PrintToServer(\"hello %d\", 42);\n}\n",
    "ANTLR": "// comment\ngrammar Expr;\nexpr : term (('+'|'-') term)* ;\nterm : INT ;\nINT : [0-9]+ ;\nWS : [ \\t\\r\\n]+ -> skip ;\n",
    "Cython": "# cython: language_level=3\n# comment\ndef fib(int n):\n    cdef int i\n    a, b = 0, 1\n    for i in range(n):\n        a, b = b, a + b\n    return a\n",
    "Dax": "// comment\nEVALUATE\nSUMMARIZECOLUMNS(\n    Sales[Region],\n    \"Total\", SUM(Sales[Amount])\n)\n",
    "Go Template": "{{/* comment */}}\n<h1>{{ .Title }}</h1>\n<ul>\n{{ range .Items }}\n  <li>{{ .Name | html }}</li>\n{{ end }}\n</ul>\n",
    "PostScript": "%!PS-Adobe-3.0\n% comment\n/Helvetica findfont 24 scalefont setfont\n72 720 moveto (Hello) show\nshowpage\n",
    "Z80 Assembly": "; comment\n    org $8000\nstart:\n    ld a, 42\n    ld hl, $9000\n    ld (hl), a\n    ret\n",
    "LLVM": "; comment\ndefine i32 @add(i32 %a, i32 %b) {\nentry:\n  %sum = add i32 %a, %b\n  ret i32 %sum\n}\n",
    "Mojo": "# comment\nfn fib(n: Int) -> Int:\n    if n < 2:\n        return n\n    return fib(n - 1) + fib(n - 2)\n\nfn main():\n    print(fib(10))\n",
    "HolyC": "// comment\nU0 Main() {\n  \"Hello World\\n\";\n  I64 x = 42;\n}\nMain;\n",
    "Hy": "; comment\n(defn greet [name]\n  (print f\"hello {name}\"))\n(greet \"world\")\n",
    "Caddyfile Directives": "# comment\nroot * /srv\nencode gzip\nfile_server browse\n",
    "properties": "# comment\nspring.datasource.url=jdbc:postgresql://localhost/demo\nspring.jpa.hibernate.ddl-auto=validate\nserver.port=8080\n",
    "RPMSpec": "Name:           demo\nVersion:        1.0\nRelease:        1%{?dist}\nSummary:        Demo package\n\n%description\nA demo.\n\n%prep\n%setup -q\n\n%build\nmake\n\n%changelog\n* Thu Oct 08 2026 Me <me@example.com> - 1.0-1\n- Initial\n",
    "Sieve": "# comment\nrequire [\"fileinto\", \"imap4flags\"];\nif header :contains \"subject\" \"[SPAM]\" {\n    fileinto \"Junk\";\n    stop;\n}\n",
    "Smarty": "{* comment *}\n<ul>\n{foreach $items as $item}\n  <li>{$item.name|escape}</li>\n{/foreach}\n</ul>\n",
    "VimL (Vim script)": "\" comment\nset number\nlet g:mapleader = \",\"\nnnoremap <leader>w :w<CR>\n",
    "Rexx": "/* comment */\nparse arg name\nsay 'hello' name\ndo i = 1 to 10\n  say i * i\nend\n",
    "Promela": "// comment\nproctype Counter() {\n    byte x = 0;\n    do\n    :: x < 10 -> x++\n    :: else -> break\n    od\n}\ninit { run Counter() }\n",
    "QBasic": "' comment\nDIM i AS INTEGER\nFOR i = 1 TO 10\n    PRINT i * i\nNEXT i\n",
    "MiniZinc": "% comment\nint: n = 8;\narray[1..n] of var 1..n: q;\nsolve satisfy;\n",
    "Idris": "-- comment\nfib : Nat -> Nat\nfib 0 = 0\nfib 1 = 1\nfib (S (S n)) = fib (S n) + fib n\n",
    "Isabelle": "(* comment *)\ntheory Demo imports Main begin\n\ntheorem \"P --> P\"\n  by simp\n\nend\n",
}

LEXERS = []
for line in lexers_tsv.read_text().splitlines():
    if line.startswith("#") or not line.strip():
        continue
    name, aliases, files = line.split("\t")
    al = [a.strip() for a in aliases.split(",") if a.strip()]
    if not al:
        al = [name.lower().replace(" ", "-")]
    LEXERS.append((name, al))

corpus = []
for name, aliases in LEXERS:
    alias, code, src = None, None, None
    for a in aliases:
        if a in samples:
            alias, code, src = a, samples[a].read_text("utf-8", "replace"), "chroma-sample"
            break
    if code is None:
        hand_ci = {k.casefold(): v for k, v in HAND.items()}
        hand_ci.update({k.replace(" (no sample)", "").casefold(): v for k, v in HAND.items()})
        for key in [name] + aliases:
            if key.casefold() in hand_ci:
                alias, code, src = aliases[0], hand_ci[key.casefold()], "handwritten"
                break
    if code is None:
        # snippet genérico: comentario + string + número, para medir tokenizado
        alias = aliases[0]
        code = f"// {name} demo\nname = \"demo\"\ncount = 42\nend\n"
        src = "generic"
    corpus.append({"lexer": name, "alias": alias, "source": src, "code": code})

# alias extra que wr mapea (langMap)
extra = []
for a, c in [("sh", "#!/bin/sh\necho hello\n"), ("golang", "package main\nfunc main() {}\n"),
             ("py", "print('hello')\n"), ("js", "console.log('hi')\n"), ("ts", "let x: number = 1\n"),
             ("yml", "a: 1\n"), ("docker", "FROM alpine\nRUN echo hi\n"), ("console", "$ ls\nfile.txt\n"),
             ("plaintext", "plain\n"), ("txt", "plain\n"), ("zsh", "echo $ZSH_VERSION\n")]:
    corpus.append({"lexer": f"alias:{a}", "alias": a, "source": "alias-map", "code": c})

(ROOT / "lang-corpus.json").write_text(json.dumps(corpus, indent=1, ensure_ascii=False))
from collections import Counter
print("lenguajes:", len(corpus), Counter(x["source"] for x in corpus))
print("únicos por lexer:", len({x["lexer"] for x in corpus}))
