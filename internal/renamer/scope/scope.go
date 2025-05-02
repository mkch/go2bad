package scope

import (
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"slices"

	slices2 "github.com/mkch/gg/slices"
	"github.com/mkch/iter2"
	"golang.org/x/tools/go/packages"
)

// Scope is a lexical scope of go source code.
type Scope interface {
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

// Local is a scope other than the scope of universe, package or file.
type Local interface {
	Scope
	LookupUseChildren(name string, pos token.Pos) (Scope, token.Pos)
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

func (s *local) LookupUseChildren(name string, pos token.Pos) (Scope, token.Pos) {
	for _, use := range s.uses.Lookup(name) {
		if !pos.IsValid() || use.Use > pos {
			return s, use.Use
		}
	}

	for _, child := range s.children {
		if scope, pos := child.LookupUseChildren(name, pos); scope != nil {
			return scope, pos
		}
	}
	return nil, token.NoPos
}

func (s *local) CanDef(name string, pos token.Pos) bool {
	// name already defined in this scope.
	if prev := s.LookupDef(name); prev.IsValid() {
		return false
	}
	// name is used after pos, a newly added definition will shadow it.
	if scope, _ := s.LookupUseChildren(name, pos); scope != nil {
		return false
	}
	return true
}

func (s *local) CanUse(name string, pos token.Pos) bool {
	// name is already in use, the newly added will be shadowed.
	if s.LookupUse(name) != nil {
		return false
	}
	// there is already an definition of name before pos,
	// the newly added will be shadowed.
	if def := s.LookupDef(name); def.IsValid() && def < pos {
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

var universeDefs = make(map[string]token.Pos)

func init() {
	for _, name := range types.Universe.Names() {
		universeDefs[name] = token.NoPos
	}
}

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
	if pos := universeDefs[name]; pos.IsValid() {
		return pos
	}
	return s.pkg.LookupDef(name)
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
	return s.pkg.contains(pos)
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

func (m UseMap) Rename(name string, def token.Pos, newName string) {
	uses := m.Lookup(name)
	equalDef := func(e Use) bool { return e.Def == def }
	newUses := slices2.Filter(uses, equalDef)
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

func (m DefMap) Rename(name string, def token.Pos, newName string) {
	s := m.Lookup(name)
	newS := slices2.Filter(s, func(pos token.Pos) bool { return pos == def })
	m.DeleteFunc(name, func(pos token.Pos) bool { return pos == def })
	if len(newS) > 0 {
		m.Add(newName, newS...)
	}
}

type Info struct {
	Defs          DefMap
	Uses          UseMap
	DefScopes     map[*ast.Ident]Scope        // Def ID -> Scope
	DefNonObjects map[*ast.Ident]types.Object // Def ID without types.Object -> Object of it's use.
}

// PackageScope creates the package scope of pkg.
func PackageScope(p *packages.Package) (Scope, *Info) {
	var info = Info{Defs: make(DefMap),
		Uses:      make(UseMap),
		DefScopes: make(map[*ast.Ident]Scope),
	}
	var pkg = pkg{}
	universe := universe{&pkg}
	pkg.parent = &universe

	uses := slices.Collect(iter2.Map2To1(
		maps.All(p.TypesInfo.Uses),
		func(id *ast.Ident, obj types.Object) idObject {
			return idObject{id, obj}
		}))
	defs := slices.Collect(iter2.Map2To1(
		maps.All(p.TypesInfo.Defs),
		func(id *ast.Ident, obj types.Object) idObject {
			return idObject{id, obj}
		}))
	info.DefNonObjects = filterDefs(&defs, uses)
	src := p.Types.Scope()
	m := map[*types.Scope]Scope{src: &pkg}
	pkg.defs, pkg.uses = scopeDefUses(src, &pkg, &defs, &uses, &info)
	for fileScope := range src.Children() {
		var file file
		newScope(&file, (*scope)(&file), &pkg, fileScope, &defs, &uses, m, &info)
		m[fileScope] = &file
		pkg.files = append(pkg.files, &file)
	}
	m[types.Universe] = &universe
	m[src] = &pkg
	return &pkg, &info
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

// filterDefs filters out non-renamable identifiers("." and "_"), and returns non-object declarations and fields/methods.
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
				nonObjects[id] = object
			}
			// Do not delete def from defs.
		} else if object.Parent() == nil {
			// field or method
			// if _, isField := object.(*types.Var); isField {
			// 	fields[id] = object
			// } else {
			// 	methods.Add(object.(*types.Func))
			// }
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
