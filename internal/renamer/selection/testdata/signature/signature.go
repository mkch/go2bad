package signature

import (
	"io"
	"os"
)

type t1 int

func (t1) f(int) {}

type t2 int

func (t2) f(int) {}

type t3 int

func (t3) f(...int) {}

type t4 int

func (t4) f(f ...int) {}

type IntSlice []int

type t5 int

func (t5) f(IntSlice) {}

type t6 int

func (t6) f([]int) {}

type IntSliceAlias = []int

type t7 int

func (t7) f(IntSliceAlias) {}

type IntSlice2 []int

type t8 int

func (t8) f(IntSlice2) {}

type t9[T string] int

func (t9[T]) f(T) {}

type Pair[T1, T2 any] struct {
	A T1
	B T2
}

type t10[T string | byte] int

func (t10[T]) f(Pair[T, byte]) {}

type t11[T byte] int

func (t11[T]) f(Pair[T, T]) {}

type t12 int

func (t12) f(func() int) {}

type t13 int

func (t13) f(int) func() int { return nil }

type C interface {
	~byte
	t12
}
type t14[T C] int

func (t14[T]) f(T) {}

type t15 int

func (t15) f(int, *os.File) {}

type t16 int

func (t16) f(int, io.Reader) {}

type St1 struct{ a int }

type I1 interface {
	int | string | St1
}

type I2 interface {
	~int | byte | I1 | ~struct{ a int }
}

type t17[T I2] int

func (t17[T]) f(T) {}

type iface interface {
	f(int)
}

func verify_I2_int() {
	var _ iface = t17[int](0)
	var _ iface = t1(0)
}

type t18 int

func (t18) f() {} // no param or result

type t19 int

func (t19) f(struct {
	a int `tag`
}) {
}

type t20 int

func (t20) f(struct {
	a int `tag`
}) {
}

type t21 int

func (t21) f(struct {
	a int
}) {
}

type t22[T any] int

func (t22[T]) f(struct {
	a T `tag`
}) {
}

type t23[T any] int

func (t23[T]) f(struct {
	a T
}) {
}

type iface19alias = interface {
	f(struct {
		a int `tag`
	})
}

func verify_t19() {
	var i iface19alias = t19(0)

	i = t20(0)
	//i = t21(0)
	i = t22[int](0)
	//i = t32[int](0)

	_ = i
}

type t24 int

func (t24) f2(interface{ f1() int }) {}

type t25[T any] int

func (t25[T]) f2(interface{ f1() T }) {}

func verity_t24() {
	var i interface {
		f2(interface{ f1() int })
	} = t24(0)
	i = t25[int](0)
	_ = i
}
