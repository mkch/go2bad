// Package renamer implements id renaming.
package renamer

import (
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"regexp"
	"strings"

	"github.com/mkch/goingbad/internal/idgen"
	"github.com/mkch/goingbad/internal/renamer/scope"
	"github.com/mkch/goingbad/internal/renamer/selection"
	"github.com/mkch/iter2"
	"golang.org/x/tools/go/packages"
)

type defRenamer struct {
	pkgScope    scope.Scope
	info        *scope.Info
	sel         *selection.Selection
	methodGroup map[token.Pos][]selection.Method
	// The type of "*testing.T".
	// Used to match the argument of a testing function.
	// nil if "testing" package is not imported by this package.
	asterisk_testing_dot_T types.Type
}

func newDefRenamer(pkg *packages.Package) *defRenamer {
	renamer := &defRenamer{sel: selection.New(pkg)}
	renamer.methodGroup = maps.Collect(iter2.Map2(
		maps.All(selection.GroupMethods(pkg.TypesInfo.Defs)),
		func(k *types.Func, v []selection.Method) (token.Pos, []selection.Method) {
			pos := k.Pos()
			return pos, v
		}))
	renamer.pkgScope, renamer.info = scope.PackageScope(pkg.Types, pkg.TypesInfo)

	for _, imported := range pkg.Types.Imports() {
		if imported.Path() == "testing" {
			renamer.asterisk_testing_dot_T = types.NewPointer(imported.Scope().Lookup("T").Type())
			break
		}
	}
	return renamer
}

func RenameUsedExports(pkg *packages.Package, renamed map[token.Pos]string) {
	for id, use := range pkg.TypesInfo.Uses {
		if newName, ok := renamed[use.Pos()]; ok {
			id.Name = newName
		}
	}
}

func Rename(pkg *packages.Package, idGen *idgen.Generator, renameExported bool, renamedExports map[token.Pos]string, keep func(pkg, name string) bool) {
	var renamer = newDefRenamer(pkg)

	renamed := make(map[token.Pos]string)

	for id, def := range pkg.TypesInfo.Defs {
		if _, alreadyRenamed := renamed[id.Pos()]; alreadyRenamed {
			continue
		}
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
				if isTestFunc(pkg.Fset, renamer.asterisk_testing_dot_T, def) {
					continue // Do not rename test function.
				} else if field, _ := def.(*types.Var); field != nil && field.Embedded() {
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
			if result := rename(id, newName); len(result) > 0 {
				for _, r := range result {
					renamed[r.Pos()] = newName
					if exported {
						renamedExports[r.Pos()] = newName
					}
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
}

func (renamer *defRenamer) canRenameScoped(name string, defPos token.Pos, defScope scope.Scope, newName string) bool {
	if !defScope.CanDef(newName, defPos) {
		return false
	}
	for _, use := range renamer.info.Uses.Lookup(name) {
		if use.Def != defPos {
			continue
		}
		if !use.UseScope.CanUse(newName, use.Use, defScope) {
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
func (renamer *defRenamer) RenameScoped(id *ast.Ident, newName string) (renamed []*ast.Ident) {
	if !renamer.sel.CanRenameEmbedded(id.Pos(), id.Name, newName) {
		return
	}
	// TODO: Here
	scope := renamer.info.DefScopes[id]
	if !renamer.canRenameScoped(id.Name, id.Pos(), scope, newName) {
		return
	}

	scope.RenameChildren(id.Name, id.Pos(), newName)
	renamer.info.Uses.Rename(id.Name, id.Pos(), newName)
	renamer.info.Defs.Rename(id.Name, id.Pos(), newName)
	id.Name = newName
	renamer.sel.RenameEmbedded(id.Pos(), newName) // TODO: can move to above?
	return []*ast.Ident{id}
}

func (renamer *defRenamer) RenameFieldMethod(id *ast.Ident, newName string) (renamed []*ast.Ident) {
	// method
	if methodsImplSame := renamer.methodGroup[id.Pos()]; len(methodsImplSame) > 0 {
		for _, mtd := range methodsImplSame {
			if !renamer.sel.CanRenameFieldMethod(id.Name, mtd.ID.Pos(), newName) {
				return
			}
		}
		for _, mtd := range methodsImplSame {
			renamer.sel.RenameFieldMethod(mtd.ID.Name, mtd.ID.Pos(), newName)
			mtd.ID.Name = newName
			renamed = append(renamed, mtd.ID)
		}
		return
	}
	// field
	if !renamer.sel.CanRenameFieldMethod(id.Name, id.Pos(), newName) {
		return
	}
	renamer.sel.RenameFieldMethod(id.Name, id.Pos(), newName)
	id.Name = newName
	renamed = append(renamed, id)
	return

}

// TestXxx where Xxx does not start with a lowercase letter
// No id validation.
var reTestFuncName = regexp.MustCompile(`^Test[^\p{Ll}]`)

// isTestFunc returns true if obj is a test function.
func isTestFunc(fset *token.FileSet, asterisk_testing_dot_T types.Type, obj types.Object) bool {
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
	result := signature.Results()
	if result.Len() != 0 {
		return false
	}
	argumentType := types.Unalias(params.At(0).Type())
	return types.Identical(argumentType, asterisk_testing_dot_T)

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
