package selection

import (
	"go/types"
	"slices"
	"strings"

	"github.com/mkch/gg"
)

type SignatureSet []*types.Func

func (s *SignatureSet) Add(mtd *types.Func) {
	i, _ := slices.BinarySearchFunc(*s, mtd, cmpMethodSignature)
	*s = slices.Insert(*s, i, mtd)
}

func (s *SignatureSet) Find(mtd *types.Func) []*types.Func {
	i, found := slices.BinarySearchFunc(*s, mtd, cmpMethodSignature)
	if !found {
		return nil
	}
	j := i + 1
	for ; j < len(*s); j++ {
		if cmpMethodSignature((*s)[i], (*s)[j]) != 0 {
			break
		}
	}
	return (*s)[i:j]
}

func cmpMethodSignature(mtd1, mtd2 *types.Func) (r int) {
	if r = strings.Compare(mtd1.Name(), mtd2.Name()); r != 0 {
		return
	}
	sig1, sig2 := mtd1.Signature(), mtd2.Signature()
	if r = gg.If(sig1.Variadic(), 1, 0) - gg.If(sig2.Variadic(), 1, 0); r != 0 {
		return
	}
	// Due to generics, it is difficult to compare the types of params and results.
	if r = sig1.Params().Len() - sig2.Params().Len(); r != 0 {
		return
	}
	if r = sig1.Results().Len() - sig2.Results().Len(); r != 0 {
		return
	}
	return
}
