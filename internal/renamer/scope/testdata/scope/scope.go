package scope

import (
	"fmt"
	_ "unsafe"
)

var pkgVar1 int

func f1(b int) {
	if pkgVar1 == 0 {
		var tag int
		_ = tag
		fmt.Println(pkgVar1)
	}
}

func f2() {
	for {
		var tag int
		_ = tag
		b := ""
		_ = b
	}
}

var _ = pkgVar1

var tag int
