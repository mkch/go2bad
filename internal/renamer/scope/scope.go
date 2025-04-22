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
	LookupUse(name string) []Pos
	// RenameChildren follows the children chain of scopes starting with scope
	// to rename the definition and all usages of name defined at a specified position.
	RenameChildren(name string, def token.Pos, newName string)
	// Scope returns the scope corresponding to s.
	Scope(s *types.Scope) Scope
	// Innermost returns the innermost (child) scope containing pos.
	// If pos is not within any scope, the result is nil.
	Innermost(pos token.Pos) Scope
	// CanDef returns whether a new name can be defined at pos in this scope.
	CanDef(name string, pos token.Pos) bool
	// CanUse returns whether a new name can be used at pos in this scope.
	CanUse(name string, pos token.Pos) bool
}

// Local is a scope other than the scope of universe, package or file.
type Local interface {
	Scope
	LookupUseChildren(name string, pos token.Pos) (Scope, token.Pos)
}

// MultiMap is a generic map that associates string keys with slices of values of type T.
type MultiMap[T any] map[string][]T

// Lookup returns the values associated with the given name.
func (m MultiMap[T]) Lookup(name string) []T {
	return m[name]
}

// LookupFunc returns a filtered slice of values associated with the given name.
func (m MultiMap[T]) LookupFunc(name string, f func(pos T) bool) []T {
	return slices2.Filter(m.Lookup(name), f)
}

// Add appends one or more values to the slice associated with the given name.
func (m MultiMap[T]) Add(name string, pos ...T) {
	old := m.Lookup(name)
	m[name] = append(old, pos...)
}

// DeleteFunc removes elements from the slice associated with the given name
// for which the predicate function f returns true. If the resulting slice
// is empty, the name is removed from the map.
func (m MultiMap[T]) DeleteFunc(name string, f func(pos T) bool) {
	s := slices.DeleteFunc(m.Lookup(name), f)
	if len(s) == 0 {
		delete(m, name)
	} else {
		m[name] = s
	}
}

// Pos is the definition and usage positions of a name.
type Pos struct {
	Def token.Pos // Position of definition.
	Use token.Pos // Position of usage.
}

// Uses maps names to their usage.
type Uses MultiMap[Pos]

func (m Uses) Lookup(name string) []Pos {
	return MultiMap[Pos](m).Lookup(name)
}

func (m Uses) Add(name string, pos ...Pos) {
	MultiMap[Pos](m).Add(name, pos...)
}

func (m Uses) Rename(name string, def token.Pos, newName string) {
	uses := m.Lookup(name)
	equalDef := func(pos Pos) bool { return pos.Def == def }
	newUses := slices2.Filter(uses, equalDef)
	MultiMap[Pos](m).DeleteFunc(name, equalDef)
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
	uses     Uses                 // Usages in this scope.
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

func (s *scope) LookupUse(name string) []Pos {
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

func (s *scope) innermost(pos token.Pos) (Scope, bool) {
	for _, child := range s.children {
		if scope := child.Innermost(pos); scope != nil {
			return scope, false
		}
	}
	return nil, pos >= s.pos && pos < s.end
}

func (s *scope) RenameChildren(name string, def token.Pos, newName string) {
	renameChildren(s, name, def, newName, false)
}

// file is a file scope.
type file scope

func (s *file) Parent() Scope {
	return (*scope)(s).Parent()
}

func (s *file) LookupDef(name string) token.Pos {
	return (*scope)(s).LookupDef(name)
}

func (s *file) LookupUse(name string) []Pos {
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

func (s *file) Innermost(pos token.Pos) Scope {
	if scope, selfMatch := (*scope)(s).innermost(pos); scope != nil {
		return scope
	} else if selfMatch {
		return s
	}
	return nil
}

// local is a local scope(not universe, package or file).
type local scope

func (s *local) Parent() Scope {
	return (*scope)(s).Parent()
}

func (s *local) LookupDef(name string) token.Pos {
	return (*scope)(s).LookupDef(name)
}

func (s *local) LookupUse(name string) []Pos {
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

func (s *local) Innermost(pos token.Pos) Scope {
	if scope, selfMatch := (*scope)(s).innermost(pos); scope != nil {
		return scope
	} else if selfMatch {
		return s
	}
	return nil
}

func (s *local) CanDef(name string, pos token.Pos) bool {
	// name already defined in this scope.
	if prev := s.LookupDef(name); prev.IsValid() {
		return false
	}
	// name is used after pos, if a definition of name added here
	// the reference of that uses will change.
	if scope, _ := s.LookupUseChildren(name, pos); scope != nil {
		return false
	}
	return true
}

func (s *local) CanUse(name string, pos token.Pos) bool {
	// name is already in use, if add a use of name it will
	// not reference the right target.
	if s.LookupUse(name) != nil {
		return false
	}
	// there is already an definition of new before pos,
	// if add a use of name it will not reference the right target.
	if def := s.LookupDef(name); def.IsValid() && def < pos {
		return false
	}
	return true
}

// pkg is a package scope.
type pkg struct {
	parent *universe
	defs   map[string]token.Pos // Definitions in this scope.
	uses   Uses                 // Usages in this scope.
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

func (s *pkg) LookupUse(name string) []Pos {
	if pos := s.uses[name]; pos != nil {
		return pos
	}
	return nil
}

func (s *pkg) Innermost(pos token.Pos) Scope {
	for _, file := range s.files {
		if scope := file.Innermost(pos); scope != nil {
			return scope
		}
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
		// name already defined in one file of this package.
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
		// name is already used in one file of this packages ...
		if file.Parent().LookupUse(name) != nil {
			return false
		}
	}
	return true
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

func (s *universe) LookupUse(name string) []Pos {
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

func (s *universe) Innermost(pos token.Pos) Scope {
	return s.pkg.Innermost(pos)
}

type defUse struct {
	Def types.Object
	Use *ast.Ident
}

type idObject struct {
	ID     *ast.Ident
	Object types.Object
}

// PackageScope creates the package scope of pkg.
func PackageScope(p *packages.Package) Scope {
	var pkg = pkg{
		uses: make(Uses),
		defs: make(map[string]token.Pos),
	}
	universe := universe{&pkg}
	pkg.parent = &universe

	uses := slices.Collect(
		iter2.Values(
			iter2.Map2(maps.All(p.TypesInfo.Uses),
				func(id *ast.Ident, obj types.Object) (struct{}, defUse) {
					return struct{}{}, defUse{Def: obj, Use: id}
				})))
	defs := slices.Collect(iter2.Map2To1(maps.All(p.TypesInfo.Defs), func(id *ast.Ident, obj types.Object) idObject { return idObject{id, obj} }))

	src := p.Types.Scope()
	m := map[*types.Scope]Scope{src: &pkg}
	collectDefUses(pkg.defs, pkg.uses, src, &defs, &uses)
	for fileScope := range src.Children() {
		var file file
		newScope(&file, (*scope)(&file), &pkg, fileScope, &defs, &uses, m)
		m[fileScope] = &file
		pkg.files = append(pkg.files, &file)
	}
	m[types.Universe] = &universe
	m[src] = &pkg
	return &pkg
}

func newScope(target Scope, concreteTarget *scope, parent Scope, src *types.Scope, defs *[]idObject, uses *[]defUse, m map[*types.Scope]Scope) {
	concreteTarget.m = m
	concreteTarget.pos = src.Pos()
	concreteTarget.end = src.End()
	concreteTarget.parent = parent
	concreteTarget.defs = make(map[string]token.Pos)
	concreteTarget.uses = make(Uses)

	collectDefUses(concreteTarget.defs, concreteTarget.uses, src, defs, uses)

	for child := range src.Children() {
		var local local
		newScope(&local, (*scope)(&local), target, child, defs, uses, m)
		m[child] = &local
		concreteTarget.children = append(concreteTarget.children, &local)
	}
}

func collectDefUses(targetDefs map[string]token.Pos, targetUses Uses, src *types.Scope, defs *[]idObject, uses *[]defUse) {
	for i := len(*defs) - 1; i >= 0; i-- {
		if def := (*defs)[i]; def.Object == nil {
			id := def.ID
			if src.Innermost(id.Pos()) == src {
				targetDefs[id.Name] = id.Pos()
				*defs = slices.Delete(*defs, i, i+1)
			}
		} else if obj := def.Object; obj.Parent() == src {
			targetDefs[obj.Name()] = obj.Pos()
			*defs = slices.Delete(*defs, i, i+1)
		}

	}

	for i := len(*uses) - 1; i >= 0; i-- {
		if pos := (*uses)[i].Use.Pos(); src.Innermost(pos) == src {
			use := (*uses)[i]
			targetUses.Add(use.Use.Name, Pos{Def: use.Def.Pos(), Use: use.Use.Pos()})
			*uses = slices.Delete(*uses, i, i+1)
		}
	}
}
