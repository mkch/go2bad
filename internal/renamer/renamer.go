// Package renamer implements id renaming.
package renamer

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	slices2 "github.com/mkch/gg/slices"
	"github.com/mkch/go2bad/internal/idgen"
	"github.com/mkch/go2bad/internal/renamer/scope"
	"golang.org/x/tools/go/packages"
)

func createUseMap(uses map[*ast.Ident]types.Object) (result useMap) {
	result = make(useMap, len(uses))
	for id, object := range uses {
		if object.Parent() == nil { // methods and struct fields.
			continue
		}
		result.Add(id.Name, useDef{Use: id.Pos(), Def: object})
	}
	return
}

type defMap scope.MultiMap[token.Pos]

func (m defMap) Lookup(name string) []token.Pos {
	return scope.MultiMap[token.Pos](m).Lookup(name)
}

func (m defMap) LookupFunc(name string, f func(pos token.Pos) bool) []token.Pos {
	return scope.MultiMap[token.Pos](m).LookupFunc(name, f)
}

func (m defMap) Add(name string, pos ...token.Pos) {
	scope.MultiMap[token.Pos](m).Add(name, pos...)
}

func (m defMap) DeleteFunc(name string, f func(pos token.Pos) bool) {
	scope.MultiMap[token.Pos](m).DeleteFunc(name, f)
}

func (m defMap) Rename(name string, def token.Pos, newName string) {
	s := m.Lookup(name)
	newS := slices2.Filter(s, func(pos token.Pos) bool { return pos == def })
	m.DeleteFunc(name, func(pos token.Pos) bool { return pos == def })
	if len(newS) > 0 {
		m.Add(newName, newS...)
	}
}

func createDefMap(defs map[*ast.Ident]types.Object) defMap {
	result := make(defMap)
	for id, object := range defs {
		if object != nil && object.Parent() == nil { // methods and struct fields.
			continue
		}
		result.Add(id.Name, id.Pos())
	}
	return result
}

type useDef struct {
	Use token.Pos
	Def types.Object
}

type useMap scope.MultiMap[useDef]

func (m useMap) Lookup(name string) []useDef {
	return scope.MultiMap[useDef](m).Lookup(name)
}

func (m useMap) Add(name string, obj ...useDef) {
	scope.MultiMap[useDef](m).Add(name, obj...)
}

func (m useMap) Rename(name string, def token.Pos, newName string) {
	uses := m.Lookup(name)
	equalDef := func(obj useDef) bool { return obj.Def.Pos() == def }
	newUses := slices2.Filter(uses, equalDef)
	scope.MultiMap[useDef](m).DeleteFunc(name, equalDef)
	if len(newUses) > 0 {
		m.Add(newName, newUses...)
	}
}

type renamer struct {
	pkgScope scope.Scope
	useMap   useMap
	defMap   defMap
}

func Rename(pkg *packages.Package, idGen *idgen.Generator, renameExported bool, keep func(pkg, name string) bool) func(id *ast.Ident, usePos token.Pos) {
	renamer := &renamer{
		pkgScope: scope.PackageScope(pkg),
		useMap:   createUseMap(pkg.TypesInfo.Uses),
		defMap:   createDefMap(pkg.TypesInfo.Defs),
	}

	renamed := make(map[token.Pos]string)
	var xRenamed map[token.Pos]string // exported IDs renamed
	if renameExported {
		xRenamed = make(map[token.Pos]string)
	}

	for id, def := range pkg.TypesInfo.Defs {
		if id.Name == "." || id.Name == "_" || def == nil || isInitFunc(def) {
			continue
		}
		if keep(pkg.PkgPath, id.Name) {
			continue
		}
		if def.Parent() == nil { // methods and struct fields.
			continue
		}
		var next func() string
		exported := renameExported &&
			def.Parent() == pkg.Types.Scope() && id.IsExported()
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
			if renamer.Rename(id, def, newName) {
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

func (renamer *renamer) canRenameTo(name string, defPos token.Pos, defParent scope.Scope, newName string) bool {
	if !defParent.CanDef(newName, defPos) {
		return false
	}
	for _, use := range renamer.useMap.Lookup(name) {
		if use.Def.Pos() != defPos {
			continue
		}
		useScope := renamer.pkgScope.Innermost(use.Use)
		if !useScope.CanUse(newName, defPos) {
			return false
		}
	}
	return true
}

func (renamer *renamer) Rename(id *ast.Ident, defObj types.Object, newName string) bool {
	if !renamer.canRenameTo(id.Name, id.Pos(), renamer.pkgScope.Scope(defObj.Parent()), newName) {
		return false
	}

	renamer.pkgScope.Scope(defObj.Parent()).RenameChildren(id.Name, defObj.Pos(), newName)
	renamer.useMap.Rename(id.Name, defObj.Pos(), newName)
	renamer.defMap.Rename(id.Name, defObj.Pos(), newName)

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
