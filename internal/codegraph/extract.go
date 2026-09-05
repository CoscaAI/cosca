// Heuristic extraction of imports and top-level symbols per language.
//
// Heuristic v1 — structural precision, not AST. These regexes are tuned to
// capture the SHAPE of a codebase (import graph + top-level symbols), not
// to be a parser. Edge cases WILL slip through; the design is fail-open:
// a file that does not match cleanly is skipped, never fatal. Honest
// limits are documented in the package doc (P13: no fact without check).
package codegraph

import (
	"regexp"
	"strings"

	"github.com/CoscaAI/cosca/internal/codeindex"
)

// ---------------------------------------------------------------------------
// Imports
// ---------------------------------------------------------------------------

// goImportRe matches both `import ("a" "b")` blocks and `import "a"` singles.
// Block content is captured in group 1; a single import path in group 2.
var goImportRe = regexp.MustCompile(`(?s)import\s*(?:\(\s*(.*?)\s*\)|"([^"]+)")`)

// javaImportRe matches `import com.foo.Bar;` and `import static com.foo.Bar;`.
var javaImportRe = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([a-zA-Z0-9_.]+)\s*;`)

// tsImportRe matches static ES imports: `import X from 'm'`, `import {a} from "m"`,
// `import * as X from 'm'` and side-effect `import 'm'`.
var tsImportRe = regexp.MustCompile(`(?m)^\s*import\s+(?:[^'"]*?\s+from\s+)?['"]([^'"]+)['"]`)

// jsRequireRe matches CommonJS `require('m')` and dynamic `import('m')`.
var jsRequireRe = regexp.MustCompile(`(?:require|import)\s*\(\s*['"]([^'"]+)['"]\s*\)`)

// pyImportRe matches `import a.b` and `import a.b as c`.
var pyImportRe = regexp.MustCompile(`(?m)^\s*import\s+([a-zA-Z0-9_]+(?:\.[a-zA-Z0-9_]+)*)`)

// pyFromRe matches `from a.b import x`.
var pyFromRe = regexp.MustCompile(`(?m)^\s*from\s+([a-zA-Z0-9_]+(?:\.[a-zA-Z0-9_]+)*)\s+import\s+`)

// rustUseRe matches `use crate::foo;` and `use std::fmt;` — captures the first
// segment (crate / top module), which is the external dependency boundary.
var rustUseRe = regexp.MustCompile(`(?m)^\s*use\s+([a-zA-Z0-9_]+)`)

// csUsingRe matches `using System.Text;` — group 1 is an optional `static`
// modifier (skipped in extractCSharpImports), group 2 the namespace. Aliases
// (`using X = Y;`) never match because the statement must end at `;`.
var csUsingRe = regexp.MustCompile(`(?m)^\s*using\s+(?:(static)\s+)?([a-zA-Z0-9_.]+)\s*;`)

// cppIncludeRe matches `#include <stdio.h>` and `#include "local.h"`.
var cppIncludeRe = regexp.MustCompile(`(?m)^\s*#include\s*[<"]([^>"]+)[>"]`)

// importExtractor maps a language id to a function returning cleaned import
// targets (no quotes, no semicolons, no aliases).
var importExtractor = map[string]func(content string) []string{
	"go":         extractGoImports,
	"java":       extractWithRE(javaImportRe, 1),
	"typescript": extractTSImports,
	"javascript": extractTSImports,
	"python":     extractPythonImports,
	"rust":       extractWithRE(rustUseRe, 1),
	"csharp":     extractCSharpImports,
	"cpp":        extractWithRE(cppIncludeRe, 1),
	"c":          extractWithRE(cppIncludeRe, 1),
}

// ExtractImports returns the cleaned import targets for a file. Unknown
// languages return nil. Fail-open: unparseable content yields nil, not error.
func ExtractImports(path, content string, lang string) []string {
	fn, ok := importExtractor[lang]
	if !ok {
		return nil
	}
	return fn(content)
}

func extractWithRE(re *regexp.Regexp, group int) func(content string) []string {
	return func(content string) []string {
		seen := map[string]bool{}
		var out []string
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			if group < len(m) && group > 0 && m[group] != "" && !seen[m[group]] {
				seen[m[group]] = true
				out = append(out, m[group])
			}
		}
		return out
	}
}

func extractGoImports(content string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range goImportRe.FindAllStringSubmatch(content, -1) {
		if m[2] != "" {
			addUniqueImport(&out, seen, m[2])
			continue
		}
		if m[1] == "" {
			continue
		}
		// Block form: pull every quoted path inside, ignoring aliases.
		for _, sm := range quotedStringRe.FindAllStringSubmatch(m[1], -1) {
			addUniqueImport(&out, seen, sm[1])
		}
	}
	return out
}

var quotedStringRe = regexp.MustCompile(`"([^"]+)"`)

func extractTSImports(content string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range tsImportRe.FindAllStringSubmatch(content, -1) {
		if m[1] != "" {
			addUniqueImport(&out, seen, m[1])
		}
	}
	for _, m := range jsRequireRe.FindAllStringSubmatch(content, -1) {
		if m[1] != "" {
			addUniqueImport(&out, seen, m[1])
		}
	}
	return out
}

func extractPythonImports(content string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range pyImportRe.FindAllStringSubmatch(content, -1) {
		if m[1] != "" {
			addUniqueImport(&out, seen, m[1])
		}
	}
	for _, m := range pyFromRe.FindAllStringSubmatch(content, -1) {
		if m[1] != "" {
			addUniqueImport(&out, seen, m[1])
		}
	}
	return out
}

func extractCSharpImports(content string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range csUsingRe.FindAllStringSubmatch(content, -1) {
		if m[1] == "static" || m[2] == "" {
			continue // `using static X;` imports members, not a namespace
		}
		addUniqueImport(&out, seen, m[2])
	}
	return out
}

func addUniqueImport(out *[]string, seen map[string]bool, s string) {
	s = strings.TrimSpace(s)
	if s != "" && !seen[s] {
		seen[s] = true
		*out = append(*out, s)
	}
}

// ---------------------------------------------------------------------------
// Symbols
// ---------------------------------------------------------------------------

// javaClassRe matches `class Foo`, `public interface Bar`, `enum E`, `record R`.
var javaClassRe = regexp.MustCompile(`(?m)(?:\b(?:public|private|protected|static|final|abstract)\s+)*(class|interface|enum|record)\s+(\w+)`)

// javaMethodRe matches `public ReturnType method(` declarations.
var javaMethodRe = regexp.MustCompile(`(?m)\b(?:public|private|protected)\s+(?:static\s+|final\s+|abstract\s+|synchronized\s+)*[\w<>\[\],\s]+\s+(\w+)\s*\(`)

// tsTypeRe matches `class Foo`, `interface I`, `type T = ...`, `enum E`
// optionally exported. Group 1 = kind, group 2 = name.
var tsTypeRe = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(class|interface|type|enum)\s+(\w+)`)

// tsFuncRe matches `function foo(` / `export function foo(`.
var tsFuncRe = regexp.MustCompile(`(?m)^\s*(?:export\s+)?function\s+(\w+)\s*\(`)

// tsConstFuncRe matches arrow-function constants: `const f = (...) => ...`.
var tsConstFuncRe = regexp.MustCompile(`(?m)^\s*(?:export\s+)?const\s+(\w+)\s*=\s*(?:\(|async)`)

// pyClassRe matches top-level `class Foo:`.
var pyClassRe = regexp.MustCompile(`(?m)^class\s+(\w+)`)

// pyFuncRe matches top-level `def foo(` / `async def foo(`.
var pyFuncRe = regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+(\w+)\s*\(`)

// rustItemRe matches `pub fn foo`, `struct S`, `enum E`, `trait T`, `impl Type`.
// Group 1 = kind, group 2 = name.
var rustItemRe = regexp.MustCompile(`(?m)^(?:pub\s+)?(fn|struct|enum|trait|impl)\s+(\w+)`)

// csTypeRe matches `public class Foo`, `internal record R`, `struct S`, etc.
// Group 1 = kind, group 2 = name.
var csTypeRe = regexp.MustCompile(`(?m)\b(?:public|private|internal|protected)?\s*(?:abstract\s+|sealed\s+|static\s+|partial\s+)*(class|interface|record|struct|enum)\s+(\w+)`)

// csMethodRe matches `public Type Method(` declarations.
var csMethodRe = regexp.MustCompile(`(?m)\b(?:public|private|internal)\s+(?:static\s+|virtual\s+|override\s+|async\s+)*[\w<>\[\],\s]+\s+(\w+)\s*\(`)

// cppTypeRe matches `struct Foo`, `class Foo`, `enum Foo`, `union Foo`.
// Group 1 = kind, group 2 = name.
var cppTypeRe = regexp.MustCompile(`(?m)^\s*(?:typedef\s+)?(struct|class|enum|union)\s+(\w+)`)

// cppFuncRe matches `ReturnType name(args) {` (brace must follow — excludes
// forward declarations and macros). `#define NAME(...)` lines are filtered out
// via the keyword guard below.
var cppFuncRe = regexp.MustCompile(`(?m)^\s*[\w:*&]+(?:\s+[\w:*&]+)*\s+(\w+)\s*\([^;{}]*\)\s*\{`)

// symbolExtractor maps a language id to its top-level symbol extraction.
var symbolExtractor = map[string]func(content string) []codeindex.Symbol{
	"java":       extractJavaSymbols,
	"typescript": extractTSSymbols,
	"javascript": extractTSSymbols,
	"python":     extractPythonSymbols,
	"rust":       extractRustSymbols,
	"csharp":     extractCSharpSymbols,
	"cpp":        extractCppSymbols,
	"c":          extractCppSymbols,
}

// ExtractSymbols returns top-level structural symbols (classes, functions,
// types) for a file, reusing codeindex.Symbol. Go files are handled by
// codeindex.ExtractFile (AST-based) and are intentionally absent here.
func ExtractSymbols(content string, lang string) []codeindex.Symbol {
	fn, ok := symbolExtractor[lang]
	if !ok {
		return nil
	}
	return fn(content)
}

func extractJavaSymbols(content string) []codeindex.Symbol {
	var out []codeindex.Symbol
	for _, m := range javaClassRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[2], m[1]))
	}
	for _, m := range javaMethodRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[1], "method"))
	}
	return out
}

func extractTSSymbols(content string) []codeindex.Symbol {
	var out []codeindex.Symbol
	for _, m := range tsTypeRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[2], m[1]))
	}
	for _, m := range tsFuncRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[1], "function"))
	}
	for _, m := range tsConstFuncRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[1], "function"))
	}
	return out
}

func extractPythonSymbols(content string) []codeindex.Symbol {
	var out []codeindex.Symbol
	for _, m := range pyClassRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[1], "class"))
	}
	for _, m := range pyFuncRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[1], "function"))
	}
	return out
}

func extractRustSymbols(content string) []codeindex.Symbol {
	var out []codeindex.Symbol
	for _, m := range rustItemRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[2], m[1]))
	}
	return out
}

func extractCSharpSymbols(content string) []codeindex.Symbol {
	var out []codeindex.Symbol
	for _, m := range csTypeRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[2], m[1]))
	}
	for _, m := range csMethodRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[1], "method"))
	}
	return out
}

func extractCppSymbols(content string) []codeindex.Symbol {
	var out []codeindex.Symbol
	for _, m := range cppTypeRe.FindAllStringSubmatch(content, -1) {
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[2], m[1]))
	}
	for _, m := range cppFuncRe.FindAllStringSubmatch(content, -1) {
		if cppKeywordGuard[m[1]] {
			continue
		}
		out = append(out, symbolAt(content, strings.Index(content, m[0]), m[1], "function"))
	}
	return out
}

// cppKeywordGuard rejects common C/C++ control-flow words that the function
// regex can occasionally snag (e.g. `#define IF(x) {` style macros).
var cppKeywordGuard = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true,
	"catch": true, "return": true, "sizeof": true, "define": true,
}

func symbolAt(content string, matchStart int, name, kind string) codeindex.Symbol {
	if matchStart < 0 {
		matchStart = 0
	}
	return codeindex.Symbol{
		Name: name,
		Kind: kind,
		Line: lineNumber(content, matchStart),
	}
}

func lineNumber(content string, pos int) int {
	if pos < 0 || pos > len(content) {
		return 1
	}
	return strings.Count(content[:pos], "\n") + 1
}
