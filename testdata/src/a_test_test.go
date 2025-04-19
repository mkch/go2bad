package a_test

import (
	a "path/to/demo"
	"testing"
)

func TestReturn2(t *testing.T) {
	if helper() != 3 {
		t.Fail()
	}
}

func helper() int {
	return a.Return2() + 1
}
