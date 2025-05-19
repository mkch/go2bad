package scope

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"testing"
)

func Test_Scope_CanUse(t *testing.T) {
	pkg, typesInfo := loadPackage()
	pkgScope, _ := PackageScope(pkg, typesInfo)

	assertCanDef(t, pkg, pkgScope, "f1", "tag", "pkgVar1", false, "shadow package var")
	assertCanDef(t, pkg, pkgScope, "f1", "tag", "fmt", false, "shadow file var")
	assertCanDef(t, pkg, pkgScope, "f1", "tag", "unsafe", true, "unique")

	assertCanDef(t, pkg, pkgScope, "f2", "tag", "pkgVar1", true, "won't shadow package var")
	assertCanDef(t, pkg, pkgScope, "f2", "tag", "b", false, "already defined")

	assertCanDef(t, pkg, pkgScope, "", "tag", "pkgVar1", false, "already defined")
	assertCanDef(t, pkg, pkgScope, "", "tag", "f1", false, "already defined")
	assertCanDef(t, pkg, pkgScope, "", "tag", "f2", false, "already defined")
	assertCanDef(t, pkg, pkgScope, "", "tag", "fmt", false, "already defined in file")
	assertCanDef(t, pkg, pkgScope, "", "tag", "pkgVar2", false, "already defined in another file")

	assertCanDef(t, pkg, pkgScope, "", "pkgVar1", "b", false, "shadowed by parameter")

}
func assertCanDef(t *testing.T, pkg *types.Package, pkgScope Scope, funcName, tagName, name string, want bool, msg string) {
	t.Helper()
	scope, tag := lookupID(pkg, pkgScope, funcName, tagName)
	if got := scope.CanDef(name, tag); got != want {
		t.Errorf("scope.CanDef(%q) want %v got %v, %v", name, want, got, msg)
	}
}

func assertCanUse(t *testing.T, pkg *types.Package, pkgScope Scope, funcName, tagName, name string, want bool, msg string) {
	t.Helper()
	scope, tag := lookupID(pkg, pkgScope, funcName, tagName)
	if got := scope.CanUse(name, tag); got != want {
		t.Errorf("scope.CanUse(%q) want %v got %v, %v", name, want, got, msg)
	}
}

func lookupID(pkg *types.Package, scope Scope, f, id string) (Scope, token.Pos) {
	s := pkg.Scope()
	if f != "" {
		s = s.Lookup(f).(*types.Func).Scope()
	}
	if obj := lookupChildren(s, id); obj != nil {
		return scope.Scope(obj.Parent()), obj.Pos()
	}
	panic("no such tag")
}

func lookupChildren(scope *types.Scope, name string) types.Object {
	if obj := scope.Lookup(name); obj != nil {
		return obj
	}
	for child := range scope.Children() {
		if obj := lookupChildren(child, name); obj != nil {
			return obj
		}
	}
	return nil
}

func loadPackage() (pkg *types.Package, info *types.Info) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "testdata/scope/scope.go", nil, 0)
	if err != nil {
		log.Fatal(err)
	}
	f2, err := parser.ParseFile(fset, "testdata/scope/scope2.go", nil, 0)
	if err != nil {
		log.Fatal(err)
	}
	conf := types.Config{Importer: importer.Default()}
	info = &types.Info{Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object)}
	pkg, err = conf.Check("scope", fset, []*ast.File{f, f2}, info)
	if err != nil {
		log.Fatal(err)
	}
	return
}
