package internal_test

import (
	i "path/to/demo/internal"
	tt "testing"
)

func Test_Sum(t *tt.T) {
	if i.Sum(1, 2) != 3 {
		t.Fatal()
	}
}
