// Package selection manages fields and methods of types.
package selection

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/mkch/gg"
	"github.com/mkch/iter2"
	"golang.org/x/tools/go/packages"
)

type typ interface {
	// field returns the shallowest depth of a field in selection T.name.
	field(name string, visited gg.Set[typ]) (depth int)
	// ptrField returns the shallowest depth of a field in selection (&T).Field.
	ptrField(name string, visited gg.Set[typ]) (depth int)
	// Field returns the shallowest depth of a method in selection T.name.
	method(name string, visited gg.Set[typ]) (depth int)
	// ptrMethod returns the shallowest depth of a field in selection (&T).Method.
	ptrMethod(name string, visited gg.Set[typ]) (depth int)
}

// defined is a defined [typ].
type defined struct {
	methods    gg.Set[string] // methods declared with receiver T
	ptrMethods gg.Set[string] // methods declared with receiver *T
	underlying typ            // nil if underlying is not is not one of the types in this package
}

func newDefined(bound typ) *defined {
	return &defined{
		make(gg.Set[string]),
		make(gg.Set[string]),
		bound,
	}
}

func (t *defined) field(name string, visited gg.Set[typ]) int {
	if t.underlying == nil {
		return -1
	}
	return t.underlying.field(name, visited)
}

func (t *defined) ptrField(name string, visited gg.Set[typ]) int {
	if t.underlying == nil {
		return -1
	}
	return t.underlying.ptrField(name, visited)
}

func (t *defined) isInterface() bool {
	_, isIface := t.underlying.(*iface)
	return isIface
}

func (t *defined) isPointer() bool {
	_, isIface := t.underlying.(*ptr)
	return isIface
}

func (t *defined) method(name string, visited gg.Set[typ]) int {
	// The method set of an interface type is the intersection of the method sets of each type in the interface's type set
	if iface, _ := t.underlying.(*iface); iface != nil {
		return iface.method(name, visited)
	}
	// The method set of a defined type T consists of all methods declared with receiver type T
	if t.methods.Contains(name) {
		return 0
	}
	// promoted
	if st, _ := t.underlying.(*st); st != nil {
		return st.method(name, visited)
	}
	return -1
}

func (t *defined) ptrMethod(name string, visited gg.Set[typ]) int {
	if t.isPointer() {
		panic("bad receiver")
	}
	if i := t.method(name, visited); i > -1 {
		return i
	}
	if t.ptrMethods.Contains(name) {
		return 0
	}
	return t.underlying.ptrMethod(name, visited)
}

func (t *defined) SetUnderlying(u typ) {
	if _, isDefined := u.(*defined); isDefined {
		panic("invalid underlying type")
	}
	t.underlying = u
}

// AddMethod add a method with receiver T.
func (t *defined) AddMethod(name string) {
	if t.isInterface() || t.isPointer() {
		panic("bad receiver")
	}
	t.methods.Add(name)
}

// AddPtrMethod add a method with receiver *T.
func (t *defined) AddPtrMethod(name string) {
	if t.isInterface() || t.isPointer() {
		panic("bad receiver")
	}
	t.ptrMethods.Add(name)
}

type typeName struct {
	t    typ
	name string
}

func cmpTypeName(t typeName, name string) int {
	return strings.Compare(t.name, name)
}

// st is a (not defined) struct [typ].
type st struct {
	fields   gg.Set[string]
	embedded []typeName
}

func newStruct() *st {
	return &st{
		make(gg.Set[string]),
		nil,
	}
}

// AddField add a field to t.
func (t *st) AddField(name string) {
	t.fields.Add(name)
}

func (t *st) field(name string, visited gg.Set[typ]) (depth int) {
	if t.fields.Contains(name) {
		return 0
	}
	if _, ok := slices.BinarySearchFunc(t.embedded, name, cmpTypeName); ok {
		return 0
	}
	return t.promoted(func(embed typeName, visited gg.Set[typ]) int { return embed.t.field(name, visited) }, visited)
}

func (t *st) ptrField(name string, visited gg.Set[typ]) (depth int) {
	return t.field(name, visited)
}

func (t *st) method(name string, visited gg.Set[typ]) (depth int) {
	return t.promoted(func(embed typeName, visited gg.Set[typ]) int { return embed.t.method(name, visited) }, visited)
}

// promoted returns the shallowest depth of a field or method in t.embedded that matches f.
func (t *st) promoted(f func(embed typeName, visited gg.Set[typ]) int, visited gg.Set[typ]) (depth int) {
	if visited.Contains(t) {
		return -1
	}
	visited.Add(t)
	defer visited.Delete(t)
	var depths = slices.Collect(
		iter2.Filter(
			iter2.Map(
				slices.Values(t.embedded),
				func(embed typeName) int { return f(embed, visited) },
			),
			func(i int) bool { return i > -1 },
		),
	)
	slices.Sort(depths)
	if len := len(depths); len == 0 {
		return -1
	} else if len > 1 && depths[0] == depths[1] {
		return -1 // more than one shallowest paths.
	} else {
		return depths[0] + 1
	}
}

func (t *st) ptrMethod(name string, visited gg.Set[typ]) (depth int) {
	return t.promoted(
		func(e typeName, visited gg.Set[typ]) int {
			var p *ptr
			if d, _ := e.t.(*defined); d != nil {
				p = newPtr(d)
			} else {
				p, _ = e.t.(*ptr)
			}
			return p.method(name, visited)
		},
		visited)
}

// AddEmbedded adds a embed field to t.
// Embed must be a defined type or a pointer to defined type.
func (t *st) AddEmbedded(name string, embed typ) {
	if _, isDefined := embed.(*defined); !isDefined {
		if ptr, isPtr := embed.(*ptr); !isPtr {
			panic("invalid embed")
		} else if _, isDefined := ptr.base.(*defined); !isDefined {
			panic("invalid embed ")
		}
	}
	i, _ := slices.BinarySearchFunc(t.embedded, name, cmpTypeName)
	t.embedded = slices.Insert(t.embedded, i, typeName{embed, name})
}

// iface is an interface [typ].
type iface struct {
	methods  gg.Set[string]
	embedded []*iface
}

func newIface() *iface {
	return &iface{
		make(gg.Set[string]),
		nil,
	}
}

func (t *iface) AddMethod(name string) {
	t.methods.Add(name)
}

func (t *iface) AddEmbedded(e typ) {
	var i *iface
	if defined, _ := e.(*defined); defined == nil {
		i, _ = e.(*iface)
	} else {
		i, _ = defined.underlying.(*iface)
	}
	if i == nil {
		panic("invalid embed")
	}
	t.embedded = append(t.embedded, i)
}

func (t *iface) ptrField(name string, visited gg.Set[typ]) (depth int) {
	return -1
}

func (t *iface) field(name string, visited gg.Set[typ]) (depth int) {
	return -1
}

func (t *iface) ptrMethod(name string, visited gg.Set[typ]) (depth int) {
	return -1
}

func (t *iface) method(name string, visited gg.Set[typ]) (depth int) {
	if t.methods.Contains(name) {
		return 0
	}
	i := slices.IndexFunc(t.embedded, func(e *iface) bool { return e.method(name, visited) > -1 })
	if i == -1 {
		return -1
	}
	return 0 // depth of iface is alway 0
}

// ptr is a pointer [typ].
type ptr struct {
	base typ
}

func newPtr(base typ) *ptr {
	return &ptr{base}
}

func (t *ptr) ptrField(name string, visited gg.Set[typ]) (depth int) {
	return -1
}

func (t *ptr) field(name string, visited gg.Set[typ]) (depth int) {
	return t.base.ptrField(name, visited)
}

func (t *ptr) ptrMethod(name string, visited gg.Set[typ]) (depth int) {
	return -1
}

func (t *ptr) method(name string, visited gg.Set[typ]) (depth int) {
	return t.base.ptrMethod(name, visited)
}

// typeKey is the key type of typeMap.
type typeKey struct {
	Pos token.Pos // Definition position of type T
	Ptr bool      // Whether *T
}

// typeMap is a map from definition position to the defined [typ].
type typeMap map[typeKey]*chainedType

// fieldMethodMap is a map from identifier position of field or method to the belonging Type.
type fieldMethodMap map[token.Pos]*chainedType

// chainedType is a type with it's embeders.
type chainedType struct {
	t        typ
	embeders []*chainedType // The types has t as their embedded fields.
}

// Type returns the [typ] of t.
// The returned value is nil if t is nil.
func (t *chainedType) Type() typ {
	if t == nil {
		return nil
	}
	return t.t
}

func addType(tm typeMap, cm compositeMap, fmm fieldMethodMap, t types.Type) *chainedType {
	switch t := t.(type) {
	case *types.Named:
		k := typeKey{Pos: t.Obj().Pos()}
		if t, exists := tm[k]; exists {
			return t
		}
		chainType := newDefined(nil)
		ret := &chainedType{t: chainType}
		tm[k] = ret
		name := t.Obj()
		chainType.SetUnderlying(addType(tm, cm, fmm, name.Type().Underlying()).Type())
		return ret
	case *types.Pointer:
		k := typeKey{Ptr: true}
		elem := t.Elem()
		switch elem := elem.(type) {
		case *types.Named:
			k.Pos = elem.Obj().Pos()
		case *types.Struct, *types.Interface:
			k.Pos = cm[elem]
		default:
			panic("invalid base type")
		}
		if t, exists := tm[k]; exists {
			return t
		}
		chainType := newPtr(nil)
		ret := &chainedType{t: chainType}
		tm[k] = ret
		chainType.base = addType(tm, cm, fmm, elem).Type()
		return ret
	case *types.Struct:
		k := typeKey{Pos: cm[t]}
		if t, exists := tm[k]; exists {
			return t
		}
		chainType := newStruct()
		ret := &chainedType{t: chainType}
		tm[k] = ret
		for f := range t.Fields() {
			t := f.Type()
			if f.Embedded() {
				var name string
				switch t := t.(type) {
				case *types.Named:
					name = t.Obj().Name()
				case *types.Pointer:
					name = t.Elem().(*types.Named).Obj().Name()
				default:
					panic("invalid embed")
				}
				embedded := addType(tm, cm, fmm, t)
				if embedded == nil {
					continue
				}
				chainType.AddEmbedded(name, embedded.Type())
				embedded.embeders = append(embedded.embeders, ret)
			} else {
				chainType.AddField(f.Name())
			}
			fmm[f.Pos()] = ret
		}
		return ret
	case *types.Interface:
		k := typeKey{Pos: cm[t]}
		if t, exists := tm[k]; exists {
			return t
		}
		chainType := newIface()
		ret := &chainedType{t: chainType}
		tm[k] = ret
		for mtd := range t.ExplicitMethods() {
			chainType.AddMethod(mtd.Name())
			fmm[mtd.Pos()] = ret
		}
		for embed := range t.EmbeddedTypes() {
			embedded := addType(tm, cm, fmm, embed)
			if embedded == nil {
				continue
			}
			chainType.AddEmbedded(embedded.Type())
			embedded.embeders = append(embedded.embeders, ret)
			switch embed := embed.(type) {
			case *types.Named:
				fmm[embed.Obj().Pos()] = ret
			case *types.Interface:
				fmm[cm[embed]] = ret
			default:
				panic("invalid embed")
			}
		}
		return ret
	}
	return nil
}

// compositeMap is a map from the [types.Type] of composite literals to it's definition position.
type compositeMap map[types.Type]token.Pos

// compositeLiterals returns all the composite literals of struct and interface.
func compositeLiterals(ts map[ast.Expr]types.TypeAndValue) (typePos compositeMap) {
	typePos = make(compositeMap)
	for expr, tv := range ts {
		switch expr.(type) {
		case *ast.StructType, *ast.InterfaceType:
			typePos[tv.Type] = expr.Pos()
		}
	}
	return
}

// Selection manages fields and methods of types.
type Selection struct {
	tm  typeMap
	fmm fieldMethodMap
}

// New creates a new [Selection] of a package.
func New(pkg *packages.Package) *Selection {
	cm := compositeLiterals(pkg.TypesInfo.Types)
	tm := make(typeMap)
	fmm := make(fieldMethodMap)
	for t := range cm {
		addType(tm, cm, fmm, t)
	}
	for _, def := range pkg.TypesInfo.Defs {
		if def, _ := def.(*types.Func); def != nil { // methods
			recv := def.Signature().Recv()
			if recv == nil {
				continue
			}
			t := addType(tm, cm, fmm, recv.Type())
			fmm[def.Pos()] = t
			switch t := t.Type().(type) {
			case *defined:
				// interface methods are already added to it's literal.
				if !t.isInterface() {
					t.AddMethod(def.Name())
				}
			case *ptr:
				t.base.(*defined).AddPtrMethod(def.Name())
			}
		}
	}

	return &Selection{tm, fmm}
}

func field(t typ, name string) (depth int) {
	return t.field(name, make(gg.Set[typ]))
}

func method(t typ, name string) (depth int) {
	return t.method(name, make(gg.Set[typ]))
}

// HasName returns whether the type t has a field or method with the given name.
func HasName(t typ, name string) (depth int) {
	if depth = field(t, name); depth > -1 {
		return
	}
	return method(t, name)
}

// RenameEmbedded returns whether embedded fields of type T which is defined at a specified position
// can be renamed to a new name.
func (sel *Selection) CanRenameEmbedded(def token.Pos, newName string) bool {
	// canRename returns whether the embedded fields of type t
	// can be renamed to a new name.
	canRename := func(t *chainedType) bool {
		for _, embeder := range t.embeders {
			if _, isInterface := embeder.t.(*iface); isInterface {
				continue
			}
			if !canRenameSelTo(embeder, newName) {
				return false
			}
		}
		return true
	}
	// embeders of T
	if t := sel.tm[typeKey{Pos: def}]; t != nil {
		if !canRename(t) {
			return false
		}
	}
	// embeders of *T
	if pt := sel.tm[typeKey{Pos: def, Ptr: true}]; pt != nil {
		if !canRename(pt) {
			return false
		}
	}
	return true
}

// RenameEmbedded renames embedded fields of type T which is defined at a specified position to a new name.
func (sel *Selection) RenameEmbedded(def token.Pos, newName string) {
	// rename renames the embedded fields of type t ot a new name.
	rename := func(t *chainedType) {
		for _, embeder := range t.embeders {
			if _, isInterface := embeder.t.(*iface); isInterface {
				continue
			}
			embeder := embeder.t.(*st)
			i := slices.IndexFunc(embeder.embedded, func(e typeName) bool { return e.t == t.t })
			embeder.embedded[i].name = newName
		}
	}

	// embeders of T
	if t := sel.tm[typeKey{Pos: def}]; t != nil {
		rename(t)
	}
	// embeders of *T
	if pt := sel.tm[typeKey{Pos: def, Ptr: true}]; pt != nil {
		rename(pt)
	}
}

// canRenameSelTo returns whether a method or field in t can be renamed to new name.
func canRenameSelTo(t *chainedType, name string) bool {
	// TODO: interface and its embedded interfaces can have duplicated methods with the same signature.
	if HasName(t.t, name) > -1 {
		return false
	}
	for _, t := range t.embeders {
		if HasName(t.t, name) > -1 {
			return false
		}
	}
	return true
}

// Rename tries to rename a field or method defined at a specified position to a new name.
// The return value indicates whether the field or method is renamed successfully.
func (sel *Selection) Rename(name string, pos token.Pos, newName string) bool {
	t := sel.fmm[pos]
	if !canRenameSelTo(t, newName) {
		return false
	}

	var renamed bool
	switch t := t.t.(type) {
	case *defined:
		renamed = renameDefinedSel(t, name, newName)
	case *st:
		renamed = renameStructField(t, name, newName)
	case *ptr:
		renamed = renamePtrSel(t, name, newName)
	case *iface:
		renamed = renameInterfaceMethod(t, name, newName)
	}
	if !renamed {
		panic("rename failed")
	}
	return true
}

func renameStructField(t *st, name, newName string) bool {
	if !t.fields.Contains(name) {
		return false
	}
	t.fields.Delete(name)
	t.fields.Add(newName)
	return true
}

func renamePtrSel(t *ptr, name, newName string) bool {
	switch t := t.base.(type) {
	case *defined:
		if t.ptrMethods.Contains(name) {
			t.ptrMethods.Delete(name)
			t.ptrMethods.Add(name)
			return true
		}
		return renameDefinedSel(t, name, newName)
	case *st:
		return renameStructField(t, name, newName)
	}
	return false
}

func renameInterfaceMethod(t *iface, name, newName string) bool {
	if !t.methods.Contains(name) {
		return false
	}
	t.methods.Delete(name)
	t.methods.Add(newName)
	return true
}

func renameDefinedSel(t *defined, name, newName string) bool {
	if t.methods.Contains(name) {
		t.methods.Delete(name)
		t.methods.Add(newName)
		return true
	}
	switch u := t.underlying.(type) {
	case *st:
		return renameStructField(u, name, newName)
	case *ptr:
		return renamePtrSel(u, name, newName)
	case *iface:
		return renameInterfaceMethod(u, name, newName)
	}
	return false
}
