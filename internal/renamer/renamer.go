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
	"github.com/mkch/go2bad/internal/renamer/selection"
	"golang.org/x/tools/go/packages"
)

type defRenamer struct {
	pkgScope scope.Scope
	info     *scope.Info
	sel      *selection.Selection
}

func newDefRenamer(pkg *packages.Package) *defRenamer {
	renamer := &defRenamer{sel: selection.New(pkg)}
	renamer.pkgScope, renamer.info = scope.PackageScope(pkg)
	return renamer
}

func Rename(pkg *packages.Package, idGen *idgen.Generator, renameExported bool, keep func(pkg, name string) bool) func(id *ast.Ident, usePos token.Pos) {
	var renamer = newDefRenamer(pkg)

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
		var rename = renamer.RenameScoped
		if def == nil { // symbolic or package name in package clause.
			if !renamer.isSymbolic(id) {
				continue
			}
		} else {
			if isInitFunc(def) {
				continue
			} else if def.Parent() == nil { // methods and struct fields.
				if field, _ := def.(*types.Var); field != nil && field.Embedded() {
					continue // Do not rename embedded fields. They are renamed with their types.
				}
				rename = renamer.RenameFieldMethod
				exported = id.IsExported()
			} else {
				// Non-field and non-method identifier:
				// Exported identifier is declared in package scope and starts with
				// an upper-case letter.
				exported = def.Parent() == pkg.Types.Scope() && id.IsExported()
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
			if rename(id, newName) {
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

func (renamer *defRenamer) canRenameScoped(name string, defPos token.Pos, defParent scope.Scope, newName string) bool {
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

// isSymbolic returns whether a definition id denotes to a symbolic variable.
//
// Symbolic variable is the variable t in t := x.(type) of type switch headers.
func (renamer *defRenamer) isSymbolic(def *ast.Ident) (symbolic bool) {
	_, symbolic = renamer.info.DefNonObjects[def].(*types.Var)
	return
}

// RenameScoped renames an scoped identifier to new name.
//
// Scoped identifiers are identifiers that are not fields nor methods.
func (renamer *defRenamer) RenameScoped(id *ast.Ident, newName string) bool {
	if !renamer.sel.CanRenameEmbedded(id.Pos(), id.Name, newName) {
		return false
	}
	scope := renamer.info.DefScopes[id]
	if !renamer.canRenameScoped(id.Name, id.Pos(), scope, newName) {
		return false
	}

	scope.RenameChildren(id.Name, id.Pos(), newName)
	renamer.info.Uses.Rename(id.Name, id.Pos(), newName)
	renamer.info.Defs.Rename(id.Name, id.Pos(), newName)
	id.Name = newName
	renamer.sel.RenameEmbedded(id.Pos(), newName)
	return true
}

func (renamer *defRenamer) RenameFieldMethod(id *ast.Ident, newName string) bool {
	if !renamer.sel.Rename(id.Name, id.Pos(), newName) {
		return false
	}
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
