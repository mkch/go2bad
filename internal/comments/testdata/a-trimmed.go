//export a
package a

//line :3

//a:b
import _ "fmt" /*line f:10:2*/

//line f:15
var x = 0

//line :1

/*line :20*/
//go:noescape
func f() {}

//line :2

//go:generate cmd

//my:directive
type A struct {
	F //line :20
}

//my:directive2
type (
	B int
	C int
)

const (

	//go:generate
	C1 = 0 //a:b
)
