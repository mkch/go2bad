package scope

import (
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"math"
	"slices"

	"github.com/mkch/gg"
	"github.com/mkch/gg/slices2"
	"github.com/mkch/iter2"
)

// Scope is a lexical scope of go source code.
type Scope interface {
	// Parent returns the parent scope of this scope. Nil if this scope is universe.
	Parent() Scope
	// LookupDef returns the position of definition of name in this scope.
	// If the name is not defined in this scope, ([token.NoPos], false) is returned.
	LookupDef(name string) token.Pos
	// LookupUse returns all the usages of name in this scope.
	// If no usage is found, the result is nil.
	LookupUse(name string) []defUsePos
	// RenameChildren follows the children chain of scopes starting with scope
	// to rename the definition and all usages of name defined at a specified position.
	RenameChildren(name string, def token.Pos, newName string)
	// Scope returns the scope corresponding to s.
	Scope(s *types.Scope) Scope
	// CanDef returns whether a new name can be defined at pos in this scope.
	CanDef(name string, pos token.Pos) bool
	// CanUse returns whether a new name can be used at pos in this scope.
	CanUse(name string, pos token.Pos) bool
	// contains returns whether pos is in this scope.
	contains(pos token.Pos) bool
}

// DeleteFunc removes elements from the slice associated with the given name
// for which the predicate function f returns true. If the resulting slice
// is empty, the name is removed from the map.
func (m multiMap[T]) DeleteFunc(name string, f func(pos T) bool) {
	s := slices.DeleteFunc(m.Lookup(name), f)
	if len(s) == 0 {
		delete(m, name)
	} else {
		m[name] = s
	}
}

// defUsePos is the definition and usage positions of a name.
type defUsePos struct {
	Def token.Pos // Position of definition.
	Use token.Pos // Position of usage.
}

// useMap maps names to their usage.
type useMap multiMap[defUsePos]

func (m useMap) Lookup(name string) []defUsePos {
	return multiMap[defUsePos](m).Lookup(name)
}

func (m useMap) Add(name string, pos ...defUsePos) {
	multiMap[defUsePos](m).Add(name, pos...)
}

func (m useMap) Rename(name string, def token.Pos, newName string) {
	uses := m.Lookup(name)
	equalDef := func(pos defUsePos) bool { return pos.Def == def }
	newUses := slices2.Filter(uses, equalDef)
	multiMap[defUsePos](m).DeleteFunc(name, equalDef)
	if len(newUses) > 0 {
		m.Add(newName, newUses...)
	}
}

// scope is a building block of concrete scopes.
type scope struct {
	m        map[*types.Scope]Scope
	pos, end token.Pos // pos and end describe the scope's source code extent [pos, end)
	parent   Scope
	defs     map[string]token.Pos // Definitions in this scope.
	uses     useMap               // Usages in this scope.
	children []*local
}

func (s *scope) Scope(src *types.Scope) Scope {
	return s.m[src]
}

func (s *scope) Parent() Scope {
	return s.parent
}

func (s *scope) LookupDef(name string) token.Pos {
	return s.defs[name]
}

func (s *scope) LookupUse(name string) []defUsePos {
	return s.uses.Lookup(name)
}

func renameChildren(s *scope, name string, def token.Pos, newName string, defRenamed bool) {
	if !defRenamed {
		if pos := s.defs[name]; pos != token.NoPos {
			delete(s.defs, name)
			s.defs[newName] = pos
			defRenamed = true
		}
	}

	s.uses.Rename(name, def, newName)

	for _, child := range s.children {
		renameChildren((*scope)(child), name, def, newName, defRenamed)
	}
}

func (s *scope) RenameChildren(name string, def token.Pos, newName string) {
	renameChildren(s, name, def, newName, false)
}

func (s *scope) contains(pos token.Pos) bool {
	return pos >= s.pos && pos < s.end
}

// file is a file scope.
// Only imported package names are contained in file scope itself.
type file scope

func (s *file) Parent() Scope {
	return (*scope)(s).Parent()
}

func (s *file) LookupDef(name string) token.Pos {
	return (*scope)(s).LookupDef(name)
}

func (s *file) LookupUse(name string) []defUsePos {
	return (*scope)(s).LookupUse(name)
}

func (s *file) RenameChildren(name string, def token.Pos, newName string) {
	(*scope)(s).RenameChildren(name, def, newName)
}

func (s *file) Scope(src *types.Scope) Scope {
	return (*scope)(s).Scope(src)
}

func (s *file) CanDef(name string, pos token.Pos) bool {
	// name already defined in this file.
	if prev := s.LookupDef(name); prev.IsValid() {
		return false
	}
	// name already defined in this package.
	if prev := s.Parent().LookupDef(name); prev.IsValid() {
		return false
	}
	return true
}

func (s *file) CanUse(name string, pos token.Pos) bool {
	// name is already used in this file, if add a use of name it will
	// not reference the right target.
	if s.LookupUse(name) != nil {
		return false
	}
	// name is already used in this package ...
	if s.Parent().LookupUse(name) != nil {
		return false
	}
	return true
}

func (s *file) contains(pos token.Pos) bool {
	return (*scope)(s).contains(pos)
}

// local is a local scope(not universe, package or file).
type local scope

func (s *local) Parent() Scope {
	return (*scope)(s).Parent()
}

func (s *local) LookupDef(name string) token.Pos {
	return (*scope)(s).LookupDef(name)
}

func (s *local) LookupUse(name string) []defUsePos {
	return (*scope)(s).LookupUse(name)
}

func (s *local) RenameChildren(name string, def token.Pos, newName string) {
	(*scope)(s).RenameChildren(name, def, newName)
}

func (s *local) Scope(src *types.Scope) Scope {
	return (*scope)(s).Scope(src)
}

// LookupUseChildren searches for a usage of the given name that occurs after the specified position
// within the current scope or any of its child scopes.  If no such usage is found, it returns nil.
func (s *local) LookupUseChildren(name string, pos token.Pos) Scope {
	for _, use := range s.uses.Lookup(name) {
		if !pos.IsValid() || use.Use > pos {
			return s
		}
	}

	for _, child := range s.children {
		if scope := child.LookupUseChildren(name, pos); scope != nil {
			return scope
		}
	}
	return nil
}

func (s *local) CanDef(name string, pos token.Pos) bool {
	// name already defined in this scope.
	if prev := s.LookupDef(name); prev.IsValid() {
		return false
	}
	// name is used after pos, a newly added definition will shadow it.
	if s.LookupUseChildren(name, pos) != nil {
		return false
	}
	return true
}

// lookupDefParent looks for a name in recursive parents of s(including s itself).
// If a name is found and the found position is less than pos, the scope contains that
// name is returned.
func (s *local) lookupDefParent(name string, pos token.Pos) Scope {
	if found := s.LookupDef(name); found.IsValid() && found < pos {
		return s
	}
	for s := s.Parent(); s != nil; s = s.Parent() {
		found := s.LookupDef(name)
		if !found.IsValid() {
			continue
		}
		if _, isLocal := s.(*local); isLocal {
			// do not consider pos if s is *file, *pkg or *universe
			return gg.If(found < pos, s, nil)
		}
		return s
	}
	return nil
}

func (s *local) CanUse(name string, pos token.Pos) bool {
	// name is already in use, the newly added will be shadowed.
	if s.LookupUse(name) != nil {
		return false
	}
	// there is already an definition of name before pos,
	// the newly added will be shadowed.
	if s.lookupDefParent(name, pos) != nil {
		return false
	}
	return true
}

func (s *local) contains(pos token.Pos) bool {
	return (*scope)(s).contains(pos)
}

// pkg is a package scope.
type pkg struct {
	parent *universe
	defs   map[string]token.Pos // Definitions in this scope.
	uses   useMap               // Usages in this scope.
	files  []*file
}

func (s *pkg) Scope(src *types.Scope) Scope {
	if len(s.files) > 0 {
		return s.files[0].Scope(src)
	}
	return nil
}

func (s *pkg) Parent() Scope {
	return s.parent
}

func (s *pkg) LookupDef(name string) token.Pos {
	if pos := s.defs[name]; pos.IsValid() {
		return pos
	}
	return token.NoPos
}

func (s *pkg) LookupUse(name string) []defUsePos {
	if pos := s.uses[name]; pos != nil {
		return pos
	}
	return nil
}

func (s *pkg) RenameChildren(name string, def token.Pos, newName string) {
	delete(s.defs, name)
	s.defs[newName] = def
	s.uses.Rename(name, def, newName)
	for _, f := range s.files {
		f.RenameChildren(name, def, newName)
	}
}

func (s *pkg) CanDef(name string, pos token.Pos) bool {
	// name already defined in this package.
	if prev := s.LookupDef(name); prev.IsValid() {
		return false
	}
	for _, file := range s.files {
		// name already defined in current file.
		if prev := file.LookupDef(name); prev.IsValid() {
			return false
		}
	}
	return true
}

func (s *pkg) CanUse(name string, pos token.Pos) bool {
	// name is already used in this package, if add a use of name it will
	// not reference the right target.
	if s.LookupUse(name) != nil {
		return false
	}
	for _, file := range s.files {
		if !file.contains(pos) {
			continue
		}
		// name is already used in current file ...
		if file.Parent().LookupUse(name) != nil {
			return false
		}
	}
	return true
}

func (s *pkg) contains(pos token.Pos) bool {
	for _, f := range s.files {
		if in := f.contains(pos); in {
			return true
		}
	}
	return false
}

const universePos = token.Pos(math.MaxInt)

// universeDefs is the definitions in types.Universe.
var universeDefs = make(map[string]token.Pos)

func init() {
	for _, name := range types.Universe.Names() {
		universeDefs[name] = universePos
	}
}

// universe is the parent scope of a certain package.
// Note: it is not the parent scope of every package scopes.
type universe struct {
	pkg *pkg
}

func (s *universe) Scope(src *types.Scope) Scope {
	return s.pkg.Scope(src)
}

func (s *universe) Parent() Scope {
	return nil
}

func (s *universe) LookupDef(name string) token.Pos {
	return universeDefs[name]
}

func (s *universe) LookupUse(name string) []defUsePos {
	return s.pkg.LookupUse(name)
}

func (s *universe) RenameChildren(name string, def token.Pos, newName string) {
	s.pkg.RenameChildren(name, def, newName)
}

func (s *universe) CanDef(name string, pos token.Pos) bool {
	return false
}

func (s *universe) CanUse(name string, pos token.Pos) bool {
	return false
}

func (s *universe) contains(pos token.Pos) bool {
	return s.pkg.contains(pos) || pos == universePos
}

type idObject struct {
	id     *ast.Ident
	object types.Object
}

type Use struct {
	Use      token.Pos
	UseScope Scope
	Def      token.Pos
}

type UseMap multiMap[Use]

func (m UseMap) Lookup(name string) []Use {
	return multiMap[Use](m).Lookup(name)
}

func (m UseMap) Add(name string, obj ...Use) {
	multiMap[Use](m).Add(name, obj...)
}

// Rename updates the mapping of a given name to a new name for all uses defined at the specified position.
func (m UseMap) Rename(name string, def token.Pos, newName string) {
	equalDef := func(e Use) bool { return e.Def == def }
	newUses := slices2.Filter(m.Lookup(name), equalDef)
	multiMap[Use](m).DeleteFunc(name, equalDef)
	if len(newUses) > 0 {
		m.Add(newName, newUses...)
	}
}

type DefMap multiMap[token.Pos]

func (m DefMap) Lookup(name string) []token.Pos {
	return multiMap[token.Pos](m).Lookup(name)
}

func (m DefMap) LookupFunc(name string, f func(pos token.Pos) bool) []token.Pos {
	return multiMap[token.Pos](m).LookupFunc(name, f)
}

func (m DefMap) Add(name string, pos ...token.Pos) {
	multiMap[token.Pos](m).Add(name, pos...)
}

func (m DefMap) DeleteFunc(name string, f func(pos token.Pos) bool) {
	multiMap[token.Pos](m).DeleteFunc(name, f)
}

// Rename changes the name of a definition defined at the specified position to a new name.
func (m DefMap) Rename(name string, def token.Pos, newName string) {
	equalDef := func(pos token.Pos) bool { return pos == def }
	defs := slices2.Filter(m.Lookup(name), equalDef)
	m.DeleteFunc(name, equalDef)
	if len(defs) > 0 {
		m.Add(newName, defs...)
	}
}

type Info struct {
	Defs          DefMap
	Uses          UseMap
	DefScopes     map[*ast.Ident]Scope        // Def ID -> Scope
	DefNonObjects map[*ast.Ident]types.Object // Def ID without types.Object -> Object of it's use.
}

// PackageScope creates the package scope of pkg.
func PackageScope(p *types.Package, typesInfo *types.Info) (Scope, *Info) {
	var info = Info{Defs: make(DefMap),
		Uses:      make(UseMap),
		DefScopes: make(map[*ast.Ident]Scope),
	}
	var pkgScope = pkg{}
	universe := universe{&pkgScope}
	pkgScope.parent = &universe

	uses := slices.Collect(iter2.Map2To1(
		maps.All(typesInfo.Uses),
		func(id *ast.Ident, obj types.Object) idObject {
			return idObject{id, obj}
		}))
	defs := slices.Collect(iter2.Map2To1(
		maps.All(typesInfo.Defs),
		func(id *ast.Ident, obj types.Object) idObject {
			return idObject{id, obj}
		}))
	info.DefNonObjects = filterDefs(&defs, uses)
	src := p.Scope()
	m := map[*types.Scope]Scope{src: &pkgScope}
	pkgScope.defs, pkgScope.uses = scopeDefUses(src, &pkgScope, &defs, &uses, &info)
	for fileScope := range src.Children() {
		var file file
		var imports = make(map[string]token.Pos)
		// Find out all imported package names.
		for _, use := range uses {
			if !fileScope.Contains(use.object.Pos()) {
				continue // imported package names only used in the file imports it.
			}
			if _, isPkgName := use.object.(*types.PkgName); isPkgName {
				imports[use.id.Name] = use.object.Pos()
			}
		}
		newScope(&file, (*scope)(&file), &pkgScope, fileScope, &defs, &uses, m, &info)
		// Imported packages with explicit names are also added to file.defs by
		// scopeDefUses(called in newScope). But it' OK because file.defs is a map.
		maps.Insert(file.defs, maps.All(imports))
		m[fileScope] = &file
		pkgScope.files = append(pkgScope.files, &file)
	}
	m[types.Universe] = &universe
	m[src] = &pkgScope
	return &pkgScope, &info
}

func newScope(target Scope, concreteTarget *scope, parent Scope, src *types.Scope, defs *[]idObject, uses *[]idObject, m map[*types.Scope]Scope, info *Info) {
	concreteTarget.m = m
	concreteTarget.pos = src.Pos()
	concreteTarget.end = src.End()
	concreteTarget.parent = parent
	concreteTarget.defs, concreteTarget.uses = scopeDefUses(src, target, defs, uses, info)

	for child := range src.Children() {
		var local local
		newScope(&local, (*scope)(&local), target, child, defs, uses, m, info)
		m[child] = &local
		concreteTarget.children = append(concreteTarget.children, &local)
	}
}

// filterDefs filters out non-renamable identifiers("." and "_") and fields and methods. The return value is the non-object declarations.
func filterDefs(defs *[]idObject, uses []idObject) (nonObjects map[*ast.Ident]types.Object) {
	nonObjects = make(map[*ast.Ident]types.Object)
	for i := len(*defs) - 1; i >= 0; i-- {
		id, object := (*defs)[i].id, (*defs)[i].object
		if id.Name == "." || id.Name == "_" {
			*defs = slices.Delete(*defs, i, i+1)
			continue
		}
		if object == nil {
			// package name a in "package a" clause
			// or symbolic name t in "t := x.(type)" of type switch header.
			object := findUse(uses, id.Pos())
			if object != nil {
				nonObjects[id] = object // symbolic
			} else {
				*defs = slices.Delete(*defs, i, i+1) // package name
			}
			// Do not delete def from defs.
		} else if object.Parent() == nil {
			*defs = slices.Delete(*defs, i, i+1)
		}
	}
	return
}

// scopeDefUses finds the definitions and usages that belong to the source scope and group them by name.
// The found definitions and usages are deleted form defs and uses.
func scopeDefUses(src *types.Scope, target Scope,
	defs *[]idObject, uses *[]idObject,
	info *Info) (resultDefs map[string]token.Pos, resultUses useMap) {
	resultDefs = make(map[string]token.Pos)
	resultUses = make(useMap)
	for i := len(*defs) - 1; i >= 0; i-- {
		def := (*defs)[i]
		id, obj := def.id, def.object
		if obj != nil && obj.Parent() == src || src.Innermost(id.Pos()) == src {
			resultDefs[id.Name] = id.Pos()
			*defs = slices.Delete(*defs, i, i+1)
			info.Defs.Add(id.Name, id.Pos())
			info.DefScopes[id] = target
		}
	}

	for i := len(*uses) - 1; i >= 0; i-- {
		use := (*uses)[i]
		if pos := use.id.Pos(); src.Innermost(pos) == src {
			resultUses.Add(use.id.Name, defUsePos{Def: use.object.Pos(), Use: use.id.Pos()})
			*uses = slices.Delete(*uses, i, i+1)
			info.Uses.Add(use.id.Name, Use{Use: use.id.Pos(), Def: use.object.Pos(), UseScope: target})
		}
	}

	return
}

// findUse find an usage of definition in uses.
func findUse(uses []idObject, def token.Pos) types.Object {
	for _, use := range uses {
		if use.object.Pos() == def {
			return use.object
		}
	}
	return nil
}
