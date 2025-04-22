package scope

import (
	"go/ast"
	"go/token"
	"go/types"
	"iter"
	"maps"
	"slices"

	slices2 "github.com/mkch/gg/slices"
	"github.com/mkch/iter2"
	"golang.org/x/tools/go/packages"
)

// Scope is a lexical scope of go source code.
type Scope interface {
	Parent() Scope
	Children() iter.Seq[Scope]
	// ContainsDef returns whether the scope contains pos.
	Contains(pos token.Pos) bool
	// LookupDef returns the position of definition of name in this scope.
	// If the name is not defined in this scope, ([token.NoPos], false) is returned.
	LookupDef(name string) token.Pos
	// LookupDefParent follows the parent chain of scopes starting with scope until it
	// finds a scope where LookupDef(name) returns a valid [token.Pos], and then returns
	// that scope and position. If pos is a valid position, only positions smaller than
	// pos will be considered. If no such scope and name exists, the result is (nil, [token.NoPos]).
	LookupDefParent(name string, pos token.Pos) (Scope, token.Pos)
	// LookupUse returns all the usages of name in this scope.
	// If no usage is found, the result is nil.
	LookupUse(name string) []Pos
	// RenameChildren follows the children chain of scopes starting with scope
	// to rename the definition and all usages of name defined at a specified position.
	RenameChildren(name string, def token.Pos, newName string)
	// Innermost returns the innermost (child) scope containing pos.
	// If pos is not within any scope, the result is nil.
	Innermost(pos token.Pos) Scope
	// Scope returns the scope that references s.
	Scope(s *types.Scope) Scope
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

func (s *scope) Children() iter.Seq[Scope] {
	return iter2.Map(slices.Values(s.children), func(l *local) Scope { return l })
}

func (s *scope) Contains(pos token.Pos) bool {
	return pos >= s.pos && pos < s.end
}

func (s *scope) LookupDef(name string) token.Pos {
	return s.defs[name]
}

func (s *scope) LookupUse(name string) []Pos {
	return s.uses.Lookup(name)
}

func (s *scope) LookupDefParent(name string, pos token.Pos) (Scope, token.Pos) {
	for scope := Scope(s); scope != nil; scope = scope.Parent() {
		if def := scope.LookupDef(name); def.IsValid() {
			if _, local := scope.(Local); !local || !pos.IsValid() || pos < def {
				return scope, def
			}
		}
	}
	return nil, token.NoPos
}

func (s *scope) lookupUseChildren(name string, pos token.Pos) (Scope, token.Pos) {
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
		renameChildren(&child.scope, name, def, newName, defRenamed)
	}
}

func (s *scope) RenameChildren(name string, def token.Pos, newName string) {
	renameChildren(s, name, def, newName, false)
}

func (s *scope) Innermost(pos token.Pos) Scope {
	if !pos.IsValid() || pos <= s.pos || pos > s.end {
		return nil
	}
	for _, child := range s.children {
		if scope := child.Innermost(pos); scope != nil {
			return scope
		}
	}
	return s
}

// file is a file scope.
type file struct {
	scope
}

func (s *file) Contains(pos token.Pos) bool {
	return s.Parent().(*pkg).Contains(pos)
}

func (s *file) contains(pos token.Pos) bool {
	return s.scope.Contains(pos)
}

func (s *file) LookupDef(name string) token.Pos {
	return s.Parent().(*pkg).LookupDef(name)
}

func (s *file) lookupDef(name string) token.Pos {
	return s.scope.LookupDef(name)
}

func (s *file) LookupUse(name string) []Pos {
	return s.Parent().(*pkg).LookupUse(name)
}

func (s *file) lookupUse(name string) []Pos {
	return s.scope.LookupUse(name)
}

// local is a local scope(not universe, package or file).
type local struct {
	scope
}

func (s *local) LookupUseChildren(name string, pos token.Pos) (Scope, token.Pos) {
	return s.lookupUseChildren(name, pos)
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
func (s *pkg) Children() iter.Seq[Scope] {
	return iter2.Map(slices.Values(s.files), func(f *file) Scope { return f })
}
func (s *pkg) Contains(pos token.Pos) bool {
	for _, f := range s.files {
		if f.contains(pos) {
			return true
		}
	}
	return false
}

func (s *pkg) LookupDef(name string) token.Pos {
	if pos := s.defs[name]; pos.IsValid() {
		return pos
	}
	for _, f := range s.files {
		if pos := f.lookupDef(name); pos.IsValid() {
			return pos
		}
	}
	return token.NoPos
}

func (s *pkg) LookupUse(name string) []Pos {
	if pos := s.uses[name]; pos != nil {
		return pos
	}
	for _, f := range s.files {
		if pos := f.lookupUse(name); pos != nil {
			return pos
		}
	}
	return nil
}

func (s *pkg) LookupDefParent(name string, pos token.Pos) (Scope, token.Pos) {
	if pos := s.LookupDef(name); pos.IsValid() {
		return s, pos
	} else if pos := s.parent.LookupDef(name); pos.IsValid() {
		return s.parent, pos
	}
	return nil, token.NoPos
}

func (s *pkg) RenameChildren(name string, def token.Pos, newName string) {
	delete(s.defs, name)
	s.defs[newName] = def
	s.uses.Rename(name, def, newName)
	for _, f := range s.files {
		f.RenameChildren(name, def, newName)
	}
}

func (s *pkg) Innermost(pos token.Pos) Scope {
	for _, f := range s.files {
		if scope := f.Innermost(pos); scope != nil {
			return scope
		}
	}
	return nil
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

func (s *universe) Children() iter.Seq[Scope] {
	return func(yield func(Scope) bool) {
		yield(s.pkg)
	}
}

func (s *universe) Contains(pos token.Pos) bool {
	return s.pkg.Contains(pos)
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

func (s *universe) LookupDefParent(name string, pos token.Pos) (Scope, token.Pos) {
	if pos := s.LookupDef(name); pos.IsValid() {
		return s, pos
	}
	return nil, token.NoPos
}

func (s *universe) RenameChildren(name string, def token.Pos, newName string) {
	s.pkg.RenameChildren(name, def, newName)
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

	scope := p.Types.Scope()
	m := map[*types.Scope]Scope{scope: &pkg}
	collectDefUses(pkg.defs, pkg.uses, scope, &defs, &uses)
	for fileScope := range scope.Children() {
		var file file
		newScope(&file.scope, &pkg, fileScope, &defs, &uses, m)
		m[fileScope] = &file
		pkg.files = append(pkg.files, &file)
	}
	m[types.Universe] = &universe
	m[scope] = &pkg
	return &pkg
}

func newScope(target *scope, parent Scope, src *types.Scope, defs *[]idObject, uses *[]defUse, m map[*types.Scope]Scope) {
	target.m = m
	target.pos = src.Pos()
	target.end = src.End()
	target.parent = parent
	target.defs = make(map[string]token.Pos)
	target.uses = make(Uses)

	collectDefUses(target.defs, target.uses, src, defs, uses)

	for child := range src.Children() {
		var local local
		newScope(&local.scope, target, child, defs, uses, m)
		m[child] = &local
		target.children = append(target.children, &local)
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
