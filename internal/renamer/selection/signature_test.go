package selection

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"testing"
)

func Test_cmpMethodSignature(t *testing.T) {
	const input = `
package demo

type t1 struct {}
func(t t1) f(a int) int {
	return 0
}

type t2 struct{}
func(t t2) f(a int) string {
	return ""
}

type t3 struct{}
func(t t3) f(a, b int) string {
	return ""
}

type tt[T any] struct{}
func(t tt[T]) f(a byte) int {
	return 0
}

type tt2[T any] struct{}
func(t tt2[T]) f(a T) int {
	return 0
}

type tt3[T any] struct{}
func(t tt3[T]) f(a T) string {
	return ""
}

type tt4[T int|string] struct{}
func(t tt4[T]) f(a T) byte {
	return 0
}

type pair[T1 any, T2 any] struct {E1 T1; E2 T2}

type tt5[T any] struct{}
func(t tt5[T]) f(a pair[string, T]) string {
	return ""
}

type tt6[T any] struct{}
func(t tt6[T]) f(a pair[string, T]) byte {
	return 0
}

`

	pkg := loadPackage(input, "", "")

	t1_f := lookupMethod(pkg, "t1", 0)
	t2_f := lookupMethod(pkg, "t2", 0)
	t3_f := lookupMethod(pkg, "t3", 0)
	tt_f := lookupMethod(pkg, "tt", 0)
	tt2_f := lookupMethod(pkg, "tt2", 0)
	tt3_f := lookupMethod(pkg, "tt3", 0)
	tt4_f := lookupMethod(pkg, "tt4", 0)
	tt5_f := lookupMethod(pkg, "tt5", 0)
	tt6_f := lookupMethod(pkg, "tt6", 0)

	if cmp := cmpMethodSignature(t1_f, t2_f); cmp != 0 {
		t.Fatal(cmp)
	}
	if cmp := cmpMethodSignature(t1_f, tt_f); cmp != 0 {
		t.Fatal(cmp)
	}
	if cmp := cmpMethodSignature(t1_f, tt2_f); cmp != 0 {
		t.Fatal(cmp)
	}
	if cmp := cmpMethodSignature(t1_f, tt3_f); cmp != 0 {
		t.Fatal(cmp)
	}
	if cmp := cmpMethodSignature(t1_f, tt4_f); cmp != 0 {
		t.Fatal(cmp)
	}
	if cmp := cmpMethodSignature(tt2_f, tt3_f); cmp != 0 {
		t.Fatal(cmp)
	}
	if cmp := cmpMethodSignature(tt2_f, tt4_f); cmp != 0 {
		t.Fatal(cmp)
	}
	if cmp := cmpMethodSignature(tt5_f, tt6_f); cmp != 0 {
		t.Fatal(cmp)
	}

	if cmp := cmpMethodSignature(t1_f, t3_f); cmp == 0 {
		t.Fatal(cmp)
	}
	if cmp := cmpMethodSignature(tt6_f, t3_f); cmp == 0 {
		t.Fatal(cmp)
	}
}

func lookupMethod(pkg *types.Package, typeName string, mtdIndex int) *types.Func {
	return pkg.Scope().Lookup(typeName).Type().(*types.Named).Method(mtdIndex)
}

func loadPackage(code, filename, pkgPath string) (pkg *types.Package) {
	if filename == "" {
		filename = "demo.go"
	}
	if pkgPath == "" {
		pkgPath = "path/demo"
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, code, 0)
	if err != nil {
		log.Fatal(err)
	}
	conf := types.Config{Importer: importer.Default()}
	pkg, err = conf.Check(pkgPath, fset, []*ast.File{f}, nil)
	if err != nil {
		log.Fatal(err)
	}
	return
}
