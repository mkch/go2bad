package selection

import (
	"go/ast"
	"go/types"
	"slices"
)

// implSameMethod checks if two methods may implement the same interface method.
func implSameMethod(mtd1, mtd2 *types.Func) bool {
	if mtd1.Name() != mtd2.Name() {
		return false
	}
	sig1, sig2 := mtd1.Signature(), mtd2.Signature()
	return matchSignature(sig1, sig2)
}

// matchSignature returns if two signatures have intersection.
func matchSignature(sig1 *types.Signature, sig2 *types.Signature) bool {
	if sig1.Variadic() != sig2.Variadic() {
		return false
	}
	if sig1.Params().Len() != sig2.Params().Len() {
		return false
	}
	if sig1.Results().Len() != sig2.Results().Len() {
		return false
	}
	if !matchTuple(sig1.Params(), sig2.Params()) {
		return false
	}
	if !matchTuple(sig1.Results(), sig2.Results()) {
		return false
	}
	return true
}

// matchTuple returns if two tuples have the same length and their
// corresponding types match.
func matchTuple(t1, t2 *types.Tuple) bool {
	if t1.Len() != t2.Len() {
		return false
	}
	for i := range t1.Len() {
		var1, var2 := t1.At(i), t2.At(i)
		typ1, typ2 := var1.Type(), var2.Type()
		if !matchType(typ1, typ2) {
			return false
		}
	}
	return true
}

// matchType returns if two types can be the same.
func matchType(t1, t2 types.Type) bool {
	t1, t2 = types.Unalias(t1), types.Unalias(t2)
	if t1 == t2 {
		return true // same types.
	}

	switch t1 := t1.(type) {
	case *types.Basic:
		switch t2 := t2.(type) {
		case *types.Basic:
			return types.Identical(t1, t2)
		case *types.TypeParam:
			// e.g. int and {int | other} can be the same.
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			// Can't be the same as any other types.
			return false
		}
	case *types.Pointer:
		switch t2 := t2.(type) {
		case *types.Pointer:
			// Two pointer types can be the same only if their base types can be the same.
			return matchType(t1.Elem(), t2.Elem())
		case *types.TypeParam:
			// e.g. *int and {*int | other} can be the same.
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			return false
		}
	case *types.Slice:
		switch t2 := t2.(type) {
		case *types.Slice:
			// Two slice types can be the same only if their base types can be the same.
			return matchType(t1.Elem(), t2.Elem())
		case *types.TypeParam:
			// e.g. []int and {[]int | other} can be the same.
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			return false
		}
	case *types.Array:
		switch t2 := t2.(type) {
		case *types.Array:
			// Two array types can be the same only if they have the same length and their base types can be the same.
			return t1.Len() != t2.Len() && matchType(t1.Elem(), t2.Elem())
		case *types.TypeParam:
			// e.g. [3]int and {[3]int | other} can be the same.
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			return false
		}
	case *types.Map:
		switch t2 := t2.(type) {
		case *types.Map:
			// Two map types can be the same only if their key and value types both can be the same.
			return matchType(t1.Key(), t2.Key()) && matchType(t1.Elem(), t2.Elem())
		case *types.TypeParam:
			// e.g. map[K]V and {map[K]V | other} can be the same.
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			return false
		}
	case *types.Chan:
		switch t2 := t2.(type) {
		case *types.Chan:
			// Two channel types can be the same only if their base types can be the same ...
			return matchType(t1.Elem(), t2.Elem()) &&
				// and their directions are compatible.
				(t1.Dir() == types.SendRecv || t2.Dir() == types.SendRecv || t1.Dir() == t2.Dir())
			// e.g. chan<- int and chan U can be the same if constraint of U is {int | other}.
		case *types.TypeParam:
			// e.g. chan<- int and {chan<- int | other} intersect.
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			return false
		}
	case *types.Struct:
		switch t2 := t2.(type) {
		case *types.Struct:
			// Two structs intersect only if they have the same number of fields,
			// and their corresponding fields have the same name and their types can be the same.
			// e.g. struct{A int} and struct{A T} can be the same if constraint of T is {int | other}.
			if t1.NumFields() != t2.NumFields() {
				return false
			}
			for i := range t1.NumFields() {
				if t1.Field(i).Name() != t2.Field(i).Name() {
					return false
				}
				if !matchType(t1.Field(i).Type(), t2.Field(i).Type()) {
					return false
				}
			}
			return true
		case *types.TypeParam:
			// e.g. struct{A int} and {struct{A int} | other} intersect.
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			return false
		}
	case *types.Interface:
		switch t2 := t2.(type) {
		case *types.Interface:
			return types.Identical(t1, t2)
		case *types.TypeParam:
			// e.g. interface{A() int} and T can be the same if the constraint of T is interface{A() int; B()}.
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			return false
		}
	case *types.TypeParam:
		switch t2 := t2.(type) {
		case *types.TypeParam:
			u1 := t1.Underlying().(*types.Interface)
			u2 := t2.Underlying().(*types.Interface)
			// Two type parameters can be the same if their method sets intersect and
			// their unions intersect.
			return intersectMethodSet(u1, u2) && intersectTerms(u1, u2)
		default:
			// The behavior of types.Satisfies is unspecified if the first argument is an uninstantiated generic type
			if isUninstantiatedGeneric(t2) {
				panic("uninstantiated generic type")
			}
			// t2 is an non-type-param type.
			// This check is symmetrical to these that applied when t2 is an type-param type and t1 is a non-type-param type.
			return types.Satisfies(t2, t1.Underlying().(*types.Interface))
		}
	case *types.Named:
		switch t2 := t2.(type) {
		case *types.Named:
			if iface2, ok := t2.Underlying().(*types.Interface); ok {
				return matchType(t1, iface2)
			}
			// Two distinct defined types(*types.Named) can not possibly be the same
			// unless they are both instantiated generic types with the same origin.

			// (*types.Named).Origin() returns the named type itself if it is not a generic type,
			// so the following check returns false for two distinct defined types and
			// two instantiated generic types with the different origins.
			if !types.Identical(t1.Origin(), t2.Origin()) {
				return false
			}
			// Tow instantiated types with the same origin can be the same if their type arguments can be the same.
			// e.g. type T[int] and T[any] can be the same if T is defined as
			// 	type T[FT any] struct{F FT}.
			ta1 := t1.TypeArgs()
			ta2 := t2.TypeArgs()
			if ta1.Len() != ta2.Len() {
				panic("same origin but different type args")
			}
			for i := range ta1.Len() {
				if !matchType(ta1.At(i), ta2.At(i)) {
					return false
				}
			}
			return true
		case *types.TypeParam:
			// e.g. T1 and C intersect if T1 is defined as
			//  type T1 int
			// and constraint of C is {T1 | other}.
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			// Defined types are unique, they do not intersect with any other types.
			return false
		}
	case *types.Signature:
		switch t2 := t2.(type) {
		case *types.Signature:
			return matchSignature(t1, t2)
		case *types.TypeParam:
			return types.Satisfies(t1, t2.Underlying().(*types.Interface))
		default:
			// Function types do not intersect with any other types.
			return false
		}
	default:
		return true // safety first.
	}
}

// intersectMethodSet returns if two interfaces have intersection.
// Two interfaces do not intersect if they have the same method name but different
// signatures.
func intersectMethodSet(t1 *types.Interface, t2 *types.Interface) bool {
	set1 := types.NewMethodSet(t1)
	set2 := types.NewMethodSet(t2)
	for mtd1 := range set1.Methods() {
		f1 := mtd1.Obj().(*types.Func)
		if mtd2 := set2.Lookup(f1.Pkg(), f1.Name()); mtd2 != nil {
			if !matchType(f1.Type(), mtd2.Obj().Type()) {
				return false
			}
		}
	}
	return true
}

// intersectTerms returns if the type terms of two interfaces have intersection.
func intersectTerms(t1, t2 *types.Interface) bool {
	return len(intersect(allTerms(t1), allTerms(t2))) > 0
}

// anyTerm is the type of go keyword `any`, aka `interface{}`.
var anyTerm = types.NewTerm(false, types.NewInterfaceType(nil, nil))

// allTerms returns all the type terms in an interface t.
// The result includes all the type terms in t and its recursive embedded interfaces.
func allTerms(t *types.Interface) []*types.Term {
	var result = []*types.Term{anyTerm}
	for embed := range t.EmbeddedTypes() {
		var components []*types.Term
		switch embed := embed.(type) {
		case *types.Union:
			for term := range embed.Terms() {
				if termIface, ok := term.Type().Underlying().(*types.Interface); ok {
					components = append(components, allTerms(termIface)...)
				} else {
					components = append(components, term)
				}
			}
		default:
			if embedIface, ok := embed.Underlying().(*types.Interface); ok {
				components = allTerms(embedIface)
			} else {
				components = []*types.Term{types.NewTerm(false, embed)}
			}
		}
		result = intersect(result, components)
	}

	return result
}

// intersect returns the intersection of terms1 and terms2.
func intersect(terms1, terms2 []*types.Term) []*types.Term {
	var result = make([]*types.Term, 0, max(len(terms1), len(terms2)))
	for _, t1 := range terms1 {
		for _, t2 := range terms2 {
			if types.Satisfies(types.NewInterfaceType(nil, []types.Type{types.NewUnion([]*types.Term{t1})}),
				types.NewInterfaceType(nil, []types.Type{types.NewUnion([]*types.Term{t2})})) {
				result = append(result, t1)
			} else if types.Satisfies(types.NewInterfaceType(nil, []types.Type{types.NewUnion([]*types.Term{t2})}),
				types.NewInterfaceType(nil, []types.Type{types.NewUnion([]*types.Term{t1})})) {
				result = append(result, t2)
			}
		}
	}
	// unique
	var unique []*types.Term
result_loop:
	for _, r := range result {
		for j, u := range unique {
			if types.Identical(r.Type(), u.Type()) {
				// a term u which has the same base type of r already exists.
				if r.Tilde() == u.Tilde() || u.Tilde() {
					// u is broader than r, keeps u.
					continue result_loop
				}
				if r.Tilde() {
					// r is broader than u, use r instead.
					unique[j] = r
					continue result_loop
				}
			} else if types.Identical(r.Type().Underlying(), u.Type().Underlying()) {
				// the base types of r and u share the same underlying type.
				if u.Tilde() {
					// u is the underlying type of r.
					continue result_loop
				} else if r.Tilde() {
					// r is the underlying type of u.
					unique[j] = r
					continue result_loop
				}
			}
		}
		unique = append(unique, r)
	}
	return unique
}

// isUninstantiatedGeneric checks if a types.Type is an uninstantiated generic type.
func isUninstantiatedGeneric(t types.Type) bool {
	t = types.Unalias(t)
	// A generic type must be a named type
	named, ok := t.(*types.Named)
	if !ok {
		return false // Not a named type
	}
	return named.Origin() == named && named.TypeParams().Len() > 0
}

type Method struct {
	ID *ast.Ident
	F  *types.Func
}

// GroupMethod groups all the declared method in a package by the implementation of same interface method.
// The implMap[mtd] is a list of methods(include mtd itself) that implement the same interface method of mtd.
func GroupMethod(defs map[*ast.Ident]types.Object) (implMap map[*types.Func][]Method) {
	type group struct {
		g  []Method // generic methods implements the same method
		ng []Method // non-generic methods that implements the same method(with g if g is not empty).
	}
	var groups []group

	var iterateMethods = func(callback func(mtd Method)) {
		for id, def := range defs {
			if f, _ := def.(*types.Func); f != nil { // methods or funcs
				recv := f.Signature().Recv()
				if recv == nil {
					continue // skip funcs
				}
				callback(Method{id, f})
			}
		}
	}

	// group generic methods
	iterateMethods(func(mtd Method) {
		// A method with generic receiver need not to have generic parameters.
		// i.e func (T[Param]) f(int) {}
		// This check may result some false-positive.
		if mtd.F.Signature().RecvTypeParams() == nil &&
			mtd.F.Signature().TypeParams() == nil { // not really necessary for now(go 1.24). A method can't have its own type params.
			return // skip non-generic method
		}
		for i, group := range groups {
			if slices.IndexFunc(group.g, func(e Method) bool {
				return implSameMethod(e.F, mtd.F)
			}) > -1 {
				// found belonging group.
				groups[i].g = append(group.g, Method{})
				return
			}
		}
		// start its own group
		groups = append(groups, group{g: []Method{mtd}})
	})

	// add non-generic methods to the groups
	iterateMethods(func(mtd Method) {
		if mtd.F.Signature().RecvTypeParams() != nil ||
			mtd.F.Signature().TypeParams() != nil { // not really necessary for now(go 1.24)
			return // skip generic method
		}
		for i, group := range groups {
			if slices.IndexFunc(group.g, func(e Method) bool { return implSameMethod(e.F, mtd.F) }) > -1 {
				// found belonging group.
				groups[i].ng = append(group.ng, mtd)
				return
			}
			if len(group.g) == 0 && len(group.ng) > 0 && implSameMethod(group.ng[0].F, mtd.F) {
				// found belonging group.
				groups[i].ng = append(group.ng, mtd)
				return
			}
		}
		// start its own group
		groups = append(groups, group{ng: []Method{mtd}})

	})

	implMap = make(map[*types.Func][]Method)
	for _, group := range groups {
		finalGroup := append(group.g, group.ng...)
		for _, mtd := range finalGroup {
			implMap[mtd.F] = finalGroup
		}
	}
	return implMap
}
