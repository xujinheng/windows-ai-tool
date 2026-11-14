package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Keep all application-window messages unique, even when their handlers
// live in separate Go files.
func TestApplicationMessageIDsUnique(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	used := make(map[int64]string)
	count := 0
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(node ast.Node) bool {
			decl, ok := node.(*ast.GenDecl)
			if !ok || decl.Tok != token.CONST {
				return true
			}
			for _, spec := range decl.Specs {
				values := spec.(*ast.ValueSpec)
				for i, name := range values.Names {
					if !strings.HasPrefix(name.Name, "WM_") || i >= len(values.Values) {
						continue
					}
					binary, ok := values.Values[i].(*ast.BinaryExpr)
					if !ok || binary.Op != token.ADD {
						continue
					}
					base, leftOK := binary.X.(*ast.BasicLit)
					offset, rightOK := binary.Y.(*ast.BasicLit)
					if !leftOK || !rightOK {
						continue
					}
					b, be := strconv.ParseInt(base.Value, 0, 64)
					n, ne := strconv.ParseInt(offset.Value, 0, 64)
					if be != nil || ne != nil || b != 0x8000 {
						continue
					}
					id := b + n
					count++
					current := path + ":" + name.Name
					if previous, exists := used[id]; exists {
						t.Errorf("Windows message 0x%X is shared by %s and %s", id, previous, current)
					}
					used[id] = current
				}
			}
			return false
		})
	}
	if count < 15 {
		t.Errorf("expected application message definitions, found only %d", count)
	}
}
