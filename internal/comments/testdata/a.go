// Package doc
//
//export a
package a

// hello

// world
//line :3

//a:b
import _ "fmt" /*line f:10:2*/

//line f:15
var x = 0

//line :1

// f doc
//
/*line :20*/
//go:noescape
func f() {}

//line :2

//go:generate cmd

// A doc
//
//my:directive
type A struct {
	F //line :20
}

//my:directive2
type (
	// B
	B int
	C int
)

const (
	// C1 doc
	//go:generate
	C1 = 0 //a:b
)
