package selection

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"slices"
	"strings"
	"testing"

	"github.com/mkch/iter2"
)

func Test_implSameMethod(t *testing.T) {
	pkg, _ := loadPackage()

	f1 := lookupMethod(pkg, "t1", 0)
	f2 := lookupMethod(pkg, "t2", 0)
	f3 := lookupMethod(pkg, "t3", 0)
	f4 := lookupMethod(pkg, "t4", 0)
	f5 := lookupMethod(pkg, "t5", 0)
	f6 := lookupMethod(pkg, "t6", 0)
	f7 := lookupMethod(pkg, "t7", 0)
	f8 := lookupMethod(pkg, "t8", 0)
	f9 := lookupMethod(pkg, "t9", 0)
	f10 := lookupMethod(pkg, "t10", 0)
	f11 := lookupMethod(pkg, "t11", 0)
	f12 := lookupMethod(pkg, "t12", 0)
	f13 := lookupMethod(pkg, "t13", 0)
	f14 := lookupMethod(pkg, "t14", 0)
	f15 := lookupMethod(pkg, "t15", 0)
	f16 := lookupMethod(pkg, "t16", 0)
	f17 := lookupMethod(pkg, "t17", 0)
	f18 := lookupMethod(pkg, "t18", 0)
	f19 := lookupMethod(pkg, "t19", 0)
	f20 := lookupMethod(pkg, "t20", 0)
	f21 := lookupMethod(pkg, "t21", 0)
	f22 := lookupMethod(pkg, "t22", 0)
	f23 := lookupMethod(pkg, "t23", 0)
	f24 := lookupMethod(pkg, "t24", 0)
	f25 := lookupMethod(pkg, "t25", 0)

	assertImplSameMethod(t, f1, f2, true, "simple match")
	assertImplSameMethod(t, f1, f3, false, "variadic vs non-variadic")
	assertImplSameMethod(t, f3, f4, true, "variadic match")
	assertImplSameMethod(t, f5, f6, false, "defined types are unique")
	assertImplSameMethod(t, f5, f7, false, "defined types are unique")
	assertImplSameMethod(t, f6, f7, true, "alias match")
	assertImplSameMethod(t, f5, f8, false, "defined types are unique")
	assertImplSameMethod(t, f6, f8, false, "defined types are unique")
	assertImplSameMethod(t, f1, f9, false, "not satisfies")
	assertImplSameMethod(t, f10, f11, true, "Type terms intersect")
	assertImplSameMethod(t, f10, f1, false, "defined types are unique")
	assertImplSameMethod(t, f12, f1, false, "simple mismatch: argument")
	assertImplSameMethod(t, f13, f1, false, "simple mismatch: return value")
	assertImplSameMethod(t, f14, f9, false, "not satisfies")
	assertImplSameMethod(t, f15, f16, false, "defined types are unique")
	assertImplSameMethod(t, f1, f17, true, "satisfies")
	assertImplSameMethod(t, f18, f18, true, "no param nor result")
	assertImplSameMethod(t, f19, f20, true, "identical structs")
	assertImplSameMethod(t, f19, f21, false, "tags diff")
	assertImplSameMethod(t, f1, f21, false, "param diff")
	assertImplSameMethod(t, f19, f22, true, "potentially identical structs")
	assertImplSameMethod(t, f19, f23, false, "tags diff")
	assertImplSameMethod(t, f21, f23, true, "potentially identical structs")
	assertImplSameMethod(t, f24, f25, true, "potentially identical interfaces")

}

func Test_GroupMethods(t *testing.T) {
	pkg, info := loadPackage()
	implMap := GroupMethods(info.Defs)

	var equal = func(s1, s2 []*types.Func) bool {
		s1 = slices.Clone(s1)
		s2 = slices.Clone(s2)
		cmp := func(f1, f2 *types.Func) int {
			return strings.Compare(f1.String(), f2.String())
		}
		slices.SortFunc(s1, cmp)
		slices.SortFunc(s2, cmp)
		return slices.EqualFunc(s1, s2, func(f1, f2 *types.Func) bool { return f1 == f2 })
	}

	var stringify = func(s []*types.Func) string {
		ss := slices.Collect(iter2.Map(slices.Values(s), func(f *types.Func) string {
			var recvName string
			switch recv := f.Signature().Recv().Type().(type) {
			case *types.Named:
				recvName = recv.Obj().Name()
			default:
				recvName = recv.String()
			}
			return recvName + "." + f.Name()
		}))
		return fmt.Sprintf("%s", ss)
	}

	var assertEqualGroup = func(t *testing.T, gotMethods []Method, want []*types.Func) {
		t.Helper()
		got := slices.Collect(iter2.Map(slices.Values(gotMethods), func(mtd Method) *types.Func { return mtd.F }))
		if !equal(got, want) {
			t.Errorf("GroupMethod got %v, expected %v", stringify(got), stringify(want))
		}
	}

	f1 := lookupMethod(pkg, "t1", 0)
	f2 := lookupMethod(pkg, "t2", 0)
	f3 := lookupMethod(pkg, "t3", 0)
	f4 := lookupMethod(pkg, "t4", 0)
	f5 := lookupMethod(pkg, "t5", 0)
	f6 := lookupMethod(pkg, "t6", 0)
	f7 := lookupMethod(pkg, "t7", 0)
	f8 := lookupMethod(pkg, "t8", 0)
	f9 := lookupMethod(pkg, "t9", 0)
	f10 := lookupMethod(pkg, "t10", 0)
	f11 := lookupMethod(pkg, "t11", 0)
	f12 := lookupMethod(pkg, "t12", 0)
	f13 := lookupMethod(pkg, "t13", 0)
	f14 := lookupMethod(pkg, "t14", 0)
	f15 := lookupMethod(pkg, "t15", 0)
	f16 := lookupMethod(pkg, "t16", 0)
	f17 := lookupMethod(pkg, "t17", 0)
	f18 := lookupMethod(pkg, "t18", 0)
	fi := lookupType(pkg, "iface").(*types.Named).Underlying().(*types.Interface).ExplicitMethod(0)
	f19 := lookupMethod(pkg, "t19", 0)
	f20 := lookupMethod(pkg, "t20", 0)
	f21 := lookupMethod(pkg, "t21", 0)
	f22 := lookupMethod(pkg, "t22", 0)
	f23 := lookupMethod(pkg, "t23", 0)
	fi19 := lookupType(pkg, "iface19alias").(*types.Alias).Underlying().(*types.Interface).ExplicitMethod(0)

	assertEqualGroup(t, implMap[f1], []*types.Func{f1, f2, f9, f17, f21, f23, fi})
	assertEqualGroup(t, implMap[f2], []*types.Func{f1, f2, f9, f17, f21, f23, fi})
	assertEqualGroup(t, implMap[f9], []*types.Func{f1, f2, f9, f17, f21, f23, fi})
	assertEqualGroup(t, implMap[f17], []*types.Func{f1, f2, f9, f17, f21, f23, fi})
	assertEqualGroup(t, implMap[f21], []*types.Func{f1, f2, f9, f17, f21, f23, fi})
	assertEqualGroup(t, implMap[f23], []*types.Func{f1, f2, f9, f17, f21, f23, fi})
	assertEqualGroup(t, implMap[fi], []*types.Func{f1, f2, f9, f17, f21, f23, fi})

	assertEqualGroup(t, implMap[f3], []*types.Func{f3, f4})
	assertEqualGroup(t, implMap[f4], []*types.Func{f3, f4})

	assertEqualGroup(t, implMap[f5], []*types.Func{f5})

	assertEqualGroup(t, implMap[f6], []*types.Func{f6, f7})

	assertEqualGroup(t, implMap[f8], []*types.Func{f8})

	assertEqualGroup(t, implMap[f10], []*types.Func{f10, f11})
	assertEqualGroup(t, implMap[f11], []*types.Func{f10, f11})

	assertEqualGroup(t, implMap[f12], []*types.Func{f12})

	assertEqualGroup(t, implMap[f13], []*types.Func{f13})

	assertEqualGroup(t, implMap[f14], []*types.Func{f14})

	assertEqualGroup(t, implMap[f15], []*types.Func{f15})

	assertEqualGroup(t, implMap[f16], []*types.Func{f16})

	assertEqualGroup(t, implMap[f18], []*types.Func{f18})

	assertEqualGroup(t, implMap[f19], []*types.Func{f19, f20, f22, fi19})
	assertEqualGroup(t, implMap[f20], []*types.Func{f19, f20, f22, fi19})
	assertEqualGroup(t, implMap[f22], []*types.Func{f19, f20, f22, fi19})

}

// assertImplSameMethod is a helper for testing MayImplSameMethod.
func assertImplSameMethod(t *testing.T, mtd1, mtd2 *types.Func, expected bool, msg string) {
	t.Helper()
	actual := implSameMethod(mtd1, mtd2)
	if actual != expected {
		t.Errorf("MayImplSameMethod got %v, expected %v. %s", actual, expected, msg)
	}
}

func lookupMethod(pkg *types.Package, typeName string, mtdIndex int) *types.Func {
	return lookupType(pkg, typeName).(*types.Named).Method(mtdIndex)
}

func lookupType(pkg *types.Package, typeName string) types.Type {
	return pkg.Scope().Lookup(typeName).Type()
}

func loadPackage() (pkg *types.Package, info *types.Info) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "testdata/signature/signature.go", nil, 0)
	if err != nil {
		log.Fatal(err)
	}
	conf := types.Config{Importer: importer.Default()}
	info = &types.Info{Defs: make(map[*ast.Ident]types.Object)}
	pkg, err = conf.Check("signature", fset, []*ast.File{f}, info)
	if err != nil {
		log.Fatal(err)
	}
	return
}
