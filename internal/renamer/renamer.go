// Package renamer implements id renaming.
package renamer

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"github.com/mkch/go2bad/internal/idgen"
	"github.com/mkch/go2bad/internal/renamer/scope"
	"golang.org/x/tools/go/packages"
)

type defRenamer struct {
	pkgScope scope.Scope
	info     *scope.Info
}

func Rename(pkg *packages.Package, idGen *idgen.Generator, renameExported bool, keep func(pkg, name string) bool) func(id *ast.Ident, usePos token.Pos) {
	var renamer defRenamer
	renamer.pkgScope, renamer.info = scope.PackageScope(pkg)

	renamed := make(map[token.Pos]string)
	var xRenamed map[token.Pos]string // exported IDs renamed
	if renameExported {
		xRenamed = make(map[token.Pos]string)
	}

	for id, def := range pkg.TypesInfo.Defs {
		if id.Name == "." || id.Name == "_" {
			continue
		}
		if keep(pkg.PkgPath, id.Name) {
			continue
		}
		var exported bool
		if def == nil {
			if !renamer.isSymbolic(id) {
				continue
			}
		} else {
			exported = def.Parent() == pkg.Types.Scope() && id.IsExported()
			if isInitFunc(def) {
				continue
			} else if def.Parent() == nil { // methods and struct fields.
				continue
			}
		}
		if exported && !renameExported {
			continue
		}
		var next func() string
		if exported {
			next = idGen.NewExported(nil)
		} else {
			next = idGen.NewUnexported(nil)
		}
		for {
			newName := next()
			if id.Name == newName {
				break
			}
			if renamer.Rename(id, newName) {
				renamed[id.Pos()] = newName
				if exported {
					xRenamed[id.Pos()] = newName
				}
				break
			}
		}
	}

	for id, use := range pkg.TypesInfo.Uses {
		if newName, ok := renamed[use.Pos()]; ok {
			id.Name = newName
		}
	}

	if !renameExported {
		return nil
	}
	return func(id *ast.Ident, usePos token.Pos) {
		if newName, ok := xRenamed[usePos]; ok {
			id.Name = newName
		}
	}
}

func (renamer *defRenamer) canRenameTo(name string, defPos token.Pos, defParent scope.Scope, newName string) bool {
	if !defParent.CanDef(newName, defPos) {
		return false
	}
	for _, use := range renamer.info.Uses.Lookup(name) {
		if use.Def != defPos {
			continue
		}
		if !use.UseScope.CanUse(newName, defPos) {
			return false
		}
	}
	return true
}

// isSymbolic returns whether id denotes to a symbolic variable.
//
// Symbolic variable is the variable t in t := x.(type) of type switch headers.
func (renamer *defRenamer) isSymbolic(id *ast.Ident) (symbolic bool) {
	_, symbolic = renamer.info.NilDefObjects[id].(*types.Var)
	return
}

func (defRenamer *defRenamer) Rename(id *ast.Ident, newName string) bool {
	scope := defRenamer.info.DefScopes[id]
	if !defRenamer.canRenameTo(id.Name, id.Pos(), scope, newName) {
		return false
	}

	scope.RenameChildren(id.Name, id.Pos(), newName)
	defRenamer.info.Uses.Rename(id.Name, id.Pos(), newName)
	defRenamer.info.Defs.Rename(id.Name, id.Pos(), newName)

	id.Name = newName
	return true
}

// TestXxx where Xxx does not start with a lowercase letter
// No id validation.
var reTestFuncName = regexp.MustCompile(`^Test[^\p{Ll}]`)

// isTestFunc returns true if obj is a test function.
func isTestFunc(fset *token.FileSet, obj types.Object) bool {
	if !strings.HasSuffix(fset.PositionFor(obj.Pos(), true).Filename, "_test.go") {
		return false
	}
	f, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	if !reTestFuncName.MatchString(f.Name()) {
		return false
	}
	signature := f.Signature()
	if signature.Recv() != nil {
		return false
	}
	params := signature.Params()
	if params == nil || signature.TypeParams() != nil || signature.Variadic() {
		return false
	}
	argumentType := types.Unalias(params.At(0).Type())
	return argumentType.String() == "*testing.T"

}

// isInitFunc returns true if obj is a package init function.
func isInitFunc(obj types.Object) bool {
	f, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	if f.Name() != "init" {
		return false
	}
	signature := f.Signature()
	if signature.Recv() != nil {
		return false
	}
	return signature.Params() == nil
}
