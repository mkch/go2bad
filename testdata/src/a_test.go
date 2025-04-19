package a

import "testing"

func TestReturn1(t *testing.T) {
	if return1() != 1 {
		t.Fail()
	}
}
