package internal

func Sum(a1, b1 Int) int {
	var i unexportedImpl
	unexportedInterface(i).method()
	return int(a1 + b1)
}

type Int int

func Keep() {}

func Keep2() {}

var Keep3 int

type unexportedInterface interface {
	method()
}

type unexportedImpl struct{}

func (unexportedImpl) method() {}
