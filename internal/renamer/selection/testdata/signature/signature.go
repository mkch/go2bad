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
