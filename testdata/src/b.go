package a

import (
	_ "embed"
	"path/to/demo/internal"
)

// doc
//
//go:embed abc.txt
var abc string

func init() {
	if abc != "abc" {
		panic(abc)
	}
	f3()
}

// f3 doc
//
//line :500
var f3 = func() {
	var init = 1
	/* comment 1
	comment 2
	*/
	internal.Sum(internal.Int(init), 2)
}

// T1 doc
type T1 int
