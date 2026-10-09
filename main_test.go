package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    options
		wantErr bool
	}{
		{"no args", nil, options{}, false},
		{"every flag", []string{"-L", "--md", "--fresh"}, options{noLinks: true, rawMD: true, fresh: true}, false},
		{"words", []string{"example.com", "some", "words"}, options{words: []string{"example.com", "some", "words"}}, false},
		{"flags then words", []string{"--md", "example.com"}, options{rawMD: true, words: []string{"example.com"}}, false},
		{"a dash after a word is a word", []string{"example.com", "--md"}, options{words: []string{"example.com", "--md"}}, false},
		{"a lone dash is a word", []string{"-"}, options{words: []string{"-"}}, false},
		{"unknown option", []string{"--nope"}, options{}, true},
		{"unknown option after a word", []string{"a", "--nope"}, options{words: []string{"a", "--nope"}}, false},
		{"help", []string{"-h"}, options{action: actHelp}, false},
		{"long help", []string{"--help"}, options{action: actHelp}, false},
		{"version", []string{"-V"}, options{action: actVersion}, false},
		{"clear cache", []string{"--clear-cache"}, options{action: actClear}, false},
		{"config", []string{"--config"}, options{action: actConfig}, false},
		{"the first command wins", []string{"--clear-cache", "--help"}, options{action: actClear}, false},
		{"a command stops the scan", []string{"-h", "--nope"}, options{action: actHelp}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs(%q) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseArgs(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}

func TestHelpListsEveryFlag(t *testing.T) {
	plain := helpText(false)
	if strings.ContainsRune(plain, '\x1b') {
		t.Error("the plain help contains an escape character")
	}
	for _, f := range flagDefs {
		if !strings.Contains(plain, f.long) {
			t.Errorf("flag %s is missing from the help", f.long)
		}
		if f.short != "" && !strings.Contains(plain, f.short) {
			t.Errorf("short flag %s is missing from the help", f.short)
		}
	}
}

func TestHelpIsStyledOnATerminal(t *testing.T) {
	styled := helpText(true)
	if !strings.ContainsRune(styled, '\x1b') {
		t.Error("the terminal help has no styling")
	}
	if !strings.Contains(styled, "⣿") {
		t.Error("the terminal help has no brand glyph")
	}
}
