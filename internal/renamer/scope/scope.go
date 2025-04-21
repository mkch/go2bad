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

// scopeMaps maps from *types.Scope to *Scope and vice versa.
type scopeMaps struct {
	To   map[*types.Scope]*Scope
	From map[*Scope]*types.Scope
}

type Scope struct {
	m        *scopeMaps
	parent   *Scope
	defs     map[string]token.Pos // Definitions in this scope. Package declarations are defined in file scope.
	uses     Uses                 // Usages in this scope. Usages in package declarations are in file scope.
	children []*Scope
	universe bool
}

var universeDefs = make(map[string]token.Pos)
var universeUses = make(Uses)

func init() {
	for _, name := range types.Universe.Names() {
		universeDefs[name] = token.NoPos
	}
}

func universe(m *scopeMaps) *Scope {
	return &Scope{
		m:        m,
		defs:     universeDefs,
		uses:     universeUses,
		universe: true,
	}
}

// PackageScope creates the package scope of pkg.
func PackageScope(pkg *packages.Package) *Scope {
	var m = scopeMaps{make(map[*types.Scope]*Scope), make(map[*Scope]*types.Scope)}
	uses := slices.Collect(
		iter2.Values(
			iter2.Map2(maps.All(pkg.TypesInfo.Uses),
				func(k *ast.Ident, v types.Object) (struct{}, defUseID) {
					return struct{}{}, defUseID{Def: v.Pos(), Use: k}
				})))
	defs := slices.Collect(maps.Keys(pkg.TypesInfo.Defs))
	universe := universe(&m)
	ret := newScope(universe, pkg.Types.Scope(), &defs, &uses, &m)
	universe.children = []*Scope{ret}
	m.From[universe] = types.Universe
	m.To[types.Universe] = universe
	return ret
}

type defUseID struct {
	Def token.Pos
	Use *ast.Ident
}

func newScope(parent *Scope, src *types.Scope, defs *[]*ast.Ident, uses *[]defUseID, m *scopeMaps) *Scope {
	ret := Scope{
		m:      m,
		parent: parent,
		defs:   make(map[string]token.Pos),
		uses:   make(Uses),
	}
	m.From[&ret] = src
	m.To[src] = &ret

	for i := len(*defs) - 1; i >= 0; i-- {
		if pos := (*defs)[i].Pos(); src.Innermost(pos) == src {
			ret.defs[(*defs)[i].Name] = pos
			*defs = slices.Delete(*defs, i, i+1)
		}
	}

	for i := len(*uses) - 1; i >= 0; i-- {
		if pos := (*uses)[i].Use.Pos(); src.Innermost(pos) == src {
			name := (*uses)[i].Use.Name
			def := (*uses)[i].Def
			use := (*uses)[i].Use.Pos()
			ret.uses.Add(name, Pos{Def: def, Use: use})
			*uses = slices.Delete(*uses, i, i+1)
		}
	}

	for i := range src.NumChildren() {
		ret.children = append(ret.children, newScope(&ret, src.Child(i), defs, uses, m))
	}
	return &ret
}

func (scope *Scope) Parent() *Scope {
	return scope.parent
}

// ContainsDef returns whether the scope contains a name.
func (scope *Scope) ContainsDef(name string) bool {
	return scope.LookupDef(name) != token.NoPos
}

// LookupDef returns the position of definition of name in this scope.
// If the name is not defined in this scope, [token.NoPos] is returned
func (scope *Scope) LookupDef(name string) token.Pos {
	return scope.defs[name]
}

// LookupDefParent follows the parent chain of scopes starting with scope until it
// finds a scope where LookupDef(name) returns a valid [token.Pos], and then returns
// that scope and position. If no such scope and name exists, the result is (nil, [token.NoPos]).
func (scope *Scope) LookupDefParent(name string) (*Scope, token.Pos) {
	for s := scope; s != nil; s = s.parent {
		if pos := s.LookupDef(name); pos != token.NoPos {
			return s, pos
		}
	}
	return nil, token.NoPos
}

// LookupDefChildren follows the children chain of scopes starting with scope until it
// finds a scope where LookupDef(name) returns a valid [token.Pos], and then returns
// that pos. If no such scope and name exists, the result is [token.NoPos].
func (scope *Scope) LookupDefChildren(name string) token.Pos {
	if pos := scope.LookupDef(name); pos != token.NoPos {
		return pos
	}
	for _, child := range scope.children {
		if pos := child.LookupDefChildren(name); pos != token.NoPos {
			return pos
		}
	}
	return token.NoPos
}

// ContainsUseChildren follows the children chain of scopes starting with scope until it
// finds a scope which contains the usage of name, and then returns true.
// tIf no such scope and name exists, the result is false.
func (scope *Scope) ContainsUseChildren(name string, pos token.Pos) bool {
	uses := scope.uses.Lookup(name)
	if pos == token.NoPos {
		if len(uses) > 0 {
			return true
		}
	} else if slices.ContainsFunc(uses, func(e Pos) bool { return e.Use > pos }) {
		return true
	}
	for _, child := range scope.children {
		if child.ContainsUseChildren(name, pos) {
			return true
		}
	}
	return false
}

// Scope returns the corresponding Scope of src.
func (scope *Scope) Scope(src *types.Scope) *Scope {
	return scope.m.To[src]
}

// Innermost returns the innermost (child) scope containing pos.
// If pos is not within any scope, the result is nil.
func (scope *Scope) Innermost(pos token.Pos) *Scope {
	if s := scope.m.From[scope].Innermost(pos); s == nil {
		return nil
	} else {
		return scope.m.To[s]
	}
}

func (scope *Scope) renameChildren(name string, def token.Pos, newName string, defRenamed bool) {
	if !defRenamed {
		if pos := scope.defs[name]; pos != token.NoPos {
			delete(scope.defs, name)
			scope.defs[newName] = pos
			defRenamed = true
		}
	}

	scope.uses.Rename(name, def, newName)

	for _, child := range scope.children {
		child.renameChildren(name, def, newName, defRenamed)
	}
}

// RenameChildren follows the children chain of scopes starting with scope
// to rename the definition and all usages of name defined at a specified position.
func (scope *Scope) RenameChildren(name string, def token.Pos, newName string) {
	if scope.universe {
		panic("readonly")
	}
	scope.renameChildren(name, def, newName, false)
}

func (scope *Scope) IsUniverse() bool {
	return scope.universe
}
