package internal_test

import (
	i "path/to/demo/internal"
	ttt "testing"
)

func Test_Sum(t *ttt.T) {
	if i.Sum(1, 2) != 3 {
		t.Fatal()
	}
}
