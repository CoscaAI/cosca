package codegraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/codeindex"
)

// ---------------------------------------------------------------------------
// Language detection
// ---------------------------------------------------------------------------

func TestLanguageForFile(t *testing.T) {
	cases := map[string]string{
		"a.go":      "go",
		"a.java":    "java",
		"a.ts":      "typescript",
		"a.tsx":     "typescript",
		"a.js":      "javascript",
		"a.jsx":     "javascript",
		"a.py":      "python",
		"a.rs":      "rust",
		"a.cs":      "csharp",
		"a.cpp":     "cpp",
		"a.cc":      "cpp",
		"a.hpp":     "cpp",
		"a.c":       "c",
		"a.h":       "c",
		"a.go.tmp":  "", // unsupported
		"README.md": "", // unsupported
		"data.json": "", // unsupported
	}
	for path, want := range cases {
		got, ok := LanguageForFile(path)
		if want == "" {
			if ok {
				t.Errorf("LanguageForFile(%s) = %q, want !ok", path, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("LanguageForFile(%s) = %q, %v; want %q, true", path, got, ok, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Imports
// ---------------------------------------------------------------------------

func assertImports(t *testing.T, lang, content string, want []string) {
	t.Helper()
	got := ExtractImports("", content, lang)
	if len(got) != len(want) {
		t.Fatalf("ExtractImports(%s) = %v, want %v", lang, got, want)
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ExtractImports(%s) = %v, missing %q", lang, got, w)
		}
	}
}

func TestExtractImportsGo(t *testing.T) {
	assertImports(t, "go", `package main

import (
	"fmt"
	alias "example.com/foo"
)

import "os"
`, []string{"fmt", "example.com/foo", "os"})

	assertImports(t, "go", `package a
import "io/ioutil"`, []string{"io/ioutil"})
}

func TestExtractImportsJava(t *testing.T) {
	assertImports(t, "java", `import java.util.List;
import static java.util.Map.Entry;
import com.foo.Bar;
`, []string{"java.util.List", "java.util.Map.Entry", "com.foo.Bar"})
}

func TestExtractImportsTSJS(t *testing.T) {
	assertImports(t, "typescript", `import React from 'react';
import {useState} from "react";
import './app.css';
import * as utils from './utils';
`, []string{"react", "./app.css", "./utils"})

	assertImports(t, "javascript", `const _ = require('lodash');
const m = import('./mod.js');
`, []string{"lodash", "./mod.js"})
}

func TestExtractImportsPython(t *testing.T) {
	assertImports(t, "python", `import os
import os.path
from collections import defaultdict
from a.b.c import x
`, []string{"os", "os.path", "collections", "a.b.c"})
}

func TestExtractImportsRust(t *testing.T) {
	assertImports(t, "rust", `use crate::foo::bar;
use std::collections::HashMap;
use serde::{Deserialize, Serialize};
`, []string{"crate", "std", "serde"})
}

func TestExtractImportsCSharp(t *testing.T) {
	assertImports(t, "csharp", `using System;
using System.Text;
using static System.Math;
using Alias = System.IO;
`, []string{"System", "System.Text"})
}

func TestExtractImportsCPPAndC(t *testing.T) {
	assertImports(t, "cpp", `#include <stdio.h>
#include "local.h"
#include <vector>
`, []string{"stdio.h", "local.h", "vector"})

	assertImports(t, "c", `#include <stdio.h>
#include "util.h"
`, []string{"stdio.h", "util.h"})
}

// ---------------------------------------------------------------------------
// Symbols
// ---------------------------------------------------------------------------

func assertSymbols(t *testing.T, lang, content string, want map[string]string) {
	t.Helper()
	got := ExtractSymbols(content, lang)
	seen := map[string]bool{} // "name|kind"
	for _, s := range got {
		if s.Name == "" {
			continue
		}
		seen[s.Name+"|"+s.Kind] = true
	}
	for name, kind := range want {
		if !seen[name+"|"+kind] {
			t.Errorf("ExtractSymbols(%s): missing symbol %s (kind %s); have %d symbols", lang, name, kind, len(got))
		}
	}
}

func TestExtractSymbolsJava(t *testing.T) {
	content := `package com.foo;
public class Greeter {
    private String name;
    public String greet(String who) { return "hi"; }
}
interface Runner {}
enum Color { RED }
record Point(int x) {}
`
	assertSymbols(t, "java", content, map[string]string{
		"Greeter": "class",
		"Runner":  "interface",
		"Color":   "enum",
		"Point":   "record",
		"greet":   "method",
	})
}

func TestExtractSymbolsTS(t *testing.T) {
	content := `export interface User { id: number }
export class ApiClient {
  fetch(): Promise<any> { return null; }
}
type ID = string;
enum Mode { A }
export function build(): ApiClient { return null; }
const run = (x: number) => x;
const asyncRun = async () => {};
`
	assertSymbols(t, "typescript", content, map[string]string{
		"User":      "interface",
		"ApiClient": "class",
		"ID":        "type",
		"Mode":      "enum",
		"build":     "function",
		"run":       "function",
		"asyncRun":  "function",
	})
}

func TestExtractSymbolsPython(t *testing.T) {
	content := `class Animal:
    def speak(self):
        pass

def top_level(x):
    return x

async def fetch():
    pass
`
	assertSymbols(t, "python", content, map[string]string{
		"Animal":    "class",
		"top_level": "function",
		"fetch":     "function",
		"speak":     "function",
	})
}

func TestExtractSymbolsRust(t *testing.T) {
	content := `pub struct Point { x: f64 }
enum Direction { N, S }
trait Drawable { fn draw(&self); }
impl Drawable for Point {}
pub fn distance(a: &Point) -> f64 { 0.0 }
`
	assertSymbols(t, "rust", content, map[string]string{
		"Point":     "struct",
		"Direction": "enum",
		"Drawable":  "trait",
		"distance":  "fn",
	})

	// `impl Drawable for Point {}` is its own symbol (kind impl), same name.
	implFound := false
	for _, s := range ExtractSymbols(content, "rust") {
		if s.Name == "Drawable" && s.Kind == "impl" {
			implFound = true
		}
	}
	if !implFound {
		t.Error("missing rust impl symbol for Drawable")
	}
}

func TestExtractSymbolsCSharp(t *testing.T) {
	content := `namespace App {
    public class Service {
        public string Get() { return ""; }
    }
    internal record Config;
    struct Pair;
    enum Level { Low }
}
`
	assertSymbols(t, "csharp", content, map[string]string{
		"Service": "class",
		"Config":  "record",
		"Pair":    "struct",
		"Level":   "enum",
		"Get":     "method",
	})
}

func TestExtractSymbolsCPPAndC(t *testing.T) {
	content := `#include <vector>
class Shape {
public:
    void draw() {}
};
struct Node {
    int value;
};
int add(int a, int b) {
    return a + b;
}
`
	assertSymbols(t, "cpp", content, map[string]string{
		"Shape": "class",
		"Node":  "struct",
		"add":   "function",
	})

	assertSymbols(t, "c", `#include <stdio.h>
struct Point { int x; };
void init(struct Point* p) {}
`, map[string]string{
		"Point": "struct",
		"init":  "function",
	})
}

// ---------------------------------------------------------------------------
// BuildGraph
// ---------------------------------------------------------------------------

func TestBuildGraph(t *testing.T) {
	root := t.TempDir()

	mustWrite := func(rel, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("main.go", `package main

import (
	"greeter"
	"fmt"
)

func main() {
	_ = greeter.Hello()
}
`)
	mustWrite("greeter.go", `package greeter

func Hello() string { return "hello" }
`)
	mustWrite("app.ts", `import {hello} from './lib';
import React from 'react';
`)
	mustWrite("lib.ts", `export function hello() { return 1; }
`)
	mustWrite("utils.py", `import os

def helper():
    pass
`)
	mustWrite("build/ignored.go", "package ignored\n")

	g, err := BuildGraph(root)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Files + external modules present.
	for _, id := range []string{"main.go", "greeter.go", "app.ts", "lib.ts", "utils.py"} {
		if _, ok := g.GetNode(id); !ok {
			t.Errorf("missing file node %q", id)
		}
	}
	if n, ok := g.GetNode("main.go"); !ok || n.Type != "file" {
		t.Errorf("main.go node type = %v, want file", n.Type)
	}

	// External modules.
	for _, id := range []string{"module:fmt", "module:react", "module:os"} {
		n, ok := g.GetNode(id)
		if !ok {
			t.Errorf("missing module node %q", id)
			continue
		}
		if n.Type != "module" {
			t.Errorf("%s type = %q, want module", id, n.Type)
		}
	}

	// build/ dir was skipped — no node for ignored.go.
	if _, ok := g.GetNode("build/ignored.go"); ok {
		t.Error("expected build/ignored.go to be skipped")
	}

	// Import edges.
	mainNeighbors, err := g.GetNeighbors("main.go")
	if err != nil {
		t.Fatal(err)
	}
	mainSet := map[string]bool{}
	for _, n := range mainNeighbors {
		mainSet[n.ID] = true
	}
	if !mainSet["greeter.go"] || !mainSet["module:fmt"] {
		t.Errorf("main.go neighbors = %v, want greeter.go + module:fmt", mainNeighbors)
	}

	appNeighbors, err := g.GetNeighbors("app.ts")
	if err != nil {
		t.Fatal(err)
	}
	appSet := map[string]bool{}
	for _, n := range appNeighbors {
		appSet[n.ID] = true
	}
	if !appSet["lib.ts"] || !appSet["module:react"] {
		t.Errorf("app.ts neighbors = %v, want lib.ts + module:react", appNeighbors)
	}

	utilsNeighbors, err := g.GetNeighbors("utils.py")
	if err != nil {
		t.Fatal(err)
	}
	if len(utilsNeighbors) != 1 || utilsNeighbors[0].ID != "module:os" {
		t.Errorf("utils.py neighbors = %v, want [module:os]", utilsNeighbors)
	}

	// Sanity on totals: 5 files + 3 modules, 5 import edges.
	if got := g.GetNodeCount(); got != 8 {
		t.Errorf("node count = %d, want 8", got)
	}
	if got := g.GetEdgeCount(); got != 5 {
		t.Errorf("edge count = %d, want 5", got)
	}

	// File metadata carries extracted symbols (Go via AST, others heuristic).
	mainNode, _ := g.GetNode("main.go")
	syms, _ := mainNode.Metadata["symbols"].([]string)
	if len(syms) != 1 || syms[0] != "main" {
		t.Errorf("main.go symbols = %v, want [main]", syms)
	}
}

// Symbol struct compatibility is asserted implicitly by reuse; this guards
// against a future rename breaking the wire type.
func TestSymbolReuse(t *testing.T) {
	s := codeindex.Symbol{Name: "x", Kind: "class"}
	if s.Name != "x" {
		t.Fatal("codeindex.Symbol changed")
	}
}
