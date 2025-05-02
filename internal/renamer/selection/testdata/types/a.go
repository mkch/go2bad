package demo

type t1 struct {
	field1 string
	field2 *t1
}

type t2 struct {
	t1
}

type i1 interface {
	if1()
	if2()
}

type i2 interface {
	any
	i1
}

func (t t1) f(a int) int {
	return 0
}

func (t *t1) fp() {}

func (t *t1) fp2() {}

type tt[T any] struct{}

func (t tt[T]) f() {}

func (t *tt[T]) fp() {}

func (t *tt[T]) fp2() {}
