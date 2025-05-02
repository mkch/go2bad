// Package a doc here
package a

import (
	. "iter"
	"slices"

	it2 "github.com/mkch/iter2"
)

//go:generate

// t1 is int
type t1 int

func (v t1) value() int {
	return int(v)
}

type t2 struct {
	t1
}

func (v t2) value2() int {
	return 0
}

type intValuer interface {
	value() int
}

type iface1 interface {
	value() byte
	f(a int)
}

var v1 t1 = 0

var x = 1

func return1() int {
	// comment 1
	// comment 2
	/*




	 */
	var v1valuer intValuer = v1
	var v1 byte = byte(v1valuer.value()) // v1 shadows v1
	return int(v1 + byte(x))             // ret
}

func Return2() (n int) {
	x := return1()
	seq := concat(slices.Values([]int{x, 1}))
	for i := range seq {
		n += i
	}
	return
}

type Int = int

type a[T any] struct{ a T }

func (a[T]) value() Int {
	return 0
}

func (a[T]) f(b T) {

}

func concat(seqs ...Seq[int]) Seq[int] {
	var unused a[string]
	unused.a = ""
	_ = unused
	var args []Seq[int] = seqs
	return it2.Concat(args...)
}

func ta() {
	var any1 any
	switch t1 := any1.(type) {
	case int:
		t1 = 1
	case string:
		t1 = "1"
	default:
		_ = t1
	}
}

var (
	v2, v3, v4, v5, v6, v7, v8, v9, v10, v11, v12, v13, v14, v15, v16, v17, v19, v20 int
)

func init() {
	_ = v2
	_ = v3
	_ = v4
	_ = v5
	_ = v6
	_ = v7
	_ = v8
	_ = v9
	_ = v10
	_ = v11
	_ = v12
	_ = v13
	_ = v14
	_ = v15
	_ = v16
	_ = v17
	_ = v19
	_ = v20
}
