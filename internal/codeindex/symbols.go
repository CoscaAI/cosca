// Package codeindex extrai símbolos de código Go (funções, métodos, tipos)
// para indexação semântica — o "gatilho de caminhos" do código: achar uma
// função pelo que ela faz, não só pelo nome.
package codeindex

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Symbol representa um símbolo de código extraído de um arquivo Go.
type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"` // func, method, type, struct, interface
	File      string `json:"file"`
	Line      int    `json:"line"`
	Signature string `json:"signature,omitempty"`
	Package   string `json:"package"`
	Doc       string `json:"doc,omitempty"`
	// Offset e Length são o byte-range do símbolo no arquivo cru (para
	// byte-offset O(1) retrieval — ~80-99% menos tokens, ADR-020).
	Offset int `json:"offset,omitempty"`
	Length int `json:"length,omitempty"`
}

// ExtractFile extrai os símbolos de um único arquivo Go.
func ExtractFile(path string) ([]Symbol, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	pkgName := f.Name.Name
	var symbols []Symbol

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			kind := "func"
			name := d.Name.Name
			sig := funcSignature(fset, d.Type)
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kind = "method"
				recv := receiverName(d.Recv.List[0].Type)
				name = recv + "." + name
			}
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      kind,
				File:      path,
				Line:      fset.Position(d.Pos()).Line,
				Signature: sig,
				Package:   pkgName,
				Doc:       docText(d.Doc),
				Offset:    fset.Position(d.Pos()).Offset,
				Length:    fset.Position(d.End()).Offset - fset.Position(d.Pos()).Offset,
			})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				kind := "type"
				switch ts.Type.(type) {
				case *ast.StructType:
					kind = "struct"
				case *ast.InterfaceType:
					kind = "interface"
				}
				symbols = append(symbols, Symbol{
					Name:    ts.Name.Name,
					Kind:    kind,
					File:    path,
					Line:    fset.Position(ts.Pos()).Line,
					Package: pkgName,
					Doc:     docText(d.Doc),
					Offset:  fset.Position(ts.Pos()).Offset,
					Length:  fset.Position(ts.End()).Offset - fset.Position(ts.Pos()).Offset,
				})
			}
		}
	}
	return symbols, nil
}

// skipDirs são diretórios de cache/dependências/runtime que nunca contêm
// código-fonte do projeto — pular evita varrer GOMODCACHE, node_modules etc.
var skipDirs = map[string]bool{
	".git":            true,
	".cosca":          true,
	"node_modules":    true,
	"go-build-cache":  true,
	"vendor":          true,
	"lib":             true,
	"lib64":           true,
	"usr":             true,
	"tmp":             true,
	"proc":            true,
	"run":             true,
	"go":              true,
	"dist":            true,
}

// WalkDir extrai símbolos de todos os arquivos Go (exclui _test.go) sob root.
func WalkDir(root string) ([]Symbol, error) {
	var symbols []Symbol
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		syms, err := ExtractFile(path)
		if err != nil {
			return nil
		}
		symbols = append(symbols, syms...)
		return nil
	})
	return symbols, err
}

func funcSignature(fset *token.FileSet, ft *ast.FuncType) string {
	params := make([]string, 0)
	if ft.Params != nil {
		for _, p := range ft.Params.List {
			for range p.Names {
				params = append(params, typeString(p.Type))
			}
			if len(p.Names) == 0 {
				params = append(params, typeString(p.Type))
			}
		}
	}
	results := make([]string, 0)
	if ft.Results != nil {
		for _, r := range ft.Results.List {
			results = append(results, typeString(r.Type))
		}
	}
	ret := ""
	if len(results) == 1 {
		ret = results[0]
	} else if len(results) > 1 {
		ret = "(" + strings.Join(results, ", ") + ")"
	}
	return "(" + strings.Join(params, ", ") + ") " + ret
}

func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return typeString(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return typeString(t.X)
	default:
		return typeString(expr)
	}
}

func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.MapType:
		return "map[" + typeString(t.Key) + "]" + typeString(t.Value)
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.Ellipsis:
		return "..." + typeString(t.Elt)
	case *ast.ChanType:
		return "chan " + typeString(t.Value)
	case *ast.FuncType:
		return "func"
	case *ast.IndexExpr:
		return typeString(t.X) + "[" + typeString(t.Index) + "]"
	case *ast.IndexListExpr:
		elems := make([]string, len(t.Indices))
		for i, e := range t.Indices {
			elems[i] = typeString(e)
		}
		return typeString(t.X) + "[" + strings.Join(elems, ", ") + "]"
	default:
		return "?"
	}
}

func docText(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	return strings.TrimSpace(doc.Text())
}
