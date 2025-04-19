// package idgen implements unique identifier generation.
package idgen

import (
	"regexp"
	"strings"

	"github.com/mkch/gg"
)

// ids starts with Lu
var reLu = regexp.MustCompile(`^\p{Lu}[_\pL\p{Nd}]*$`)

// ids starts with _, Lm, Lo or Lt
var reLlmot = regexp.MustCompile(`^[_\p{Ll}\p{Lm}\p{Lo}\p{Lt}]+[_\pL\p{Nd}]*$`)

// string composed of letters an digits.
var reLNd = regexp.MustCompile(`^[_\pL\p{Nd}]+$`)

type Generator struct {
	// sort elements
	lu   []string
	lmot []string
	all  []string
}

// New creates a new Generator.
// The parameter elements is used to form IDs.
// Any non-letter-digit elements will be discarded.
// If there are no usable elements, default values will be added.
func NewGenerator(elements ...string) *Generator {
	var ret Generator
	existing := make(gg.Set[string])
	for _, elem := range elements {
		if existing.Contains(elem) {
			continue
		}
		existing.Add(elem)
		if reLu.MatchString(elem) {
			ret.lu = append(ret.lu, elem)
			ret.all = append(ret.all, elem)
		} else if reLlmot.MatchString(elem) {
			ret.lmot = append(ret.lmot, elem)
			ret.all = append(ret.all, elem)
		} else if reLNd.MatchString(elem) {
			ret.all = append(ret.all, elem)
		}
	}
	if len(ret.lu) == 0 {
		ret.lu = []string{"A"}
	}
	if len(ret.lmot) == 0 {
		ret.lmot = []string{"_"}
	}
	if len(ret.all) == 0 {
		ret.all = append(ret.lu, ret.lmot...)
	}
	return &ret
}

var reserved = []string{
	// built-ins
	"any", "bool", "byte", "comparable",
	"complex64", "complex128", "error", "float32", "float64",
	"int", "int8", "int16", "int32", "int64", "rune", "string",
	"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
	"true", "false", "iota",
	"nil",
	"append", "cap", "clear", "close", "complex", "copy", "delete", "imag", "len",
	"make", "max", "min", "new", "panic", "print", "println", "real", "recover",
	// keywords
	"break", "default", "func", "interface", "select",
	"case", "defer", "go", "map", "struct",
	"chan", "else", "goto", "package", "switch",
	"const", "fallthrough", "if", "range", "type",
	"continue", "for", "import", "return", "var",
}

func forbiddenUnexported(userDefined gg.Set[string]) gg.Set[string] {
	ret := make(gg.Set[string])
	for _, w := range reserved {
		ret.Add(w)
	}
	for w := range userDefined {
		ret.Add(w)
	}
	return ret
}

func (g *Generator) genHelper(d0 []string, stack *[]int, forbidden gg.Set[string]) string {
	for {
		var builder strings.Builder
		builder.WriteString(d0[(*stack)[len(*stack)-1]])
		for i := len(*stack) - 2; i >= 0; i-- {
			builder.WriteString(g.all[(*stack)[i]])
		}
		incIndexes(stack, len(d0), len(g.all))
		id := builder.String()
		if forbidden == nil {
			return id
		} else if _, in := forbidden[id]; !in {
			return id
		}
	}
}

// NewUnexported returns a unexport ind generator.
// IDs in the forbidden list will never be generated.
func (g *Generator) NewUnexported(forbidden gg.Set[string]) func() string {
	var stack = []int{0}
	forbidden = forbiddenUnexported(forbidden)
	return func() (id string) {
		return g.genHelper(g.lmot, &stack, forbidden)
	}
}

// NewUnexported returns a export ind generator.
// IDs in the forbidden list will never be generated.
func (g *Generator) NewExported(forbidden gg.Set[string]) func() string {
	var stack = []int{0}
	return func() (id string) {
		return g.genHelper(g.lu, &stack, forbidden)
	}
}

// incIndexes increase indexes by 1.
// Elements of number are the reversed index,
// where the base of (*number)[len(*number)-1] is base0,
// the base of other digits is base.
func incIndexes(indexes *[]int, base0, base int) {
	if len(*indexes) == 0 || base0 <= 0 || base <= 0 {
		panic("invalid arguments")
	}
	(*indexes)[0]++
	for i, d := range (*indexes)[:len(*indexes)-1] {
		if d > base-1 {
			// carry
			(*indexes)[i+1]++
			(*indexes)[i] = 0
		}
	}
	if (*indexes)[len(*indexes)-1] > base0-1 {
		// carry
		(*indexes)[len(*indexes)-1] = 0
		*indexes = append(*indexes, 0)
	}
}
