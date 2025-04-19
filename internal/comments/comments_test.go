package comments

import (
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func Test_isDirective(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{"extern", "//extern ", true},
		{"extern name", "//extern name", true},
		{"export", "//export ", true},
		{"export name", "//export name", true},
		{"go generate", "//go:generate", true},
		{"go generate cmd", "//go:generate cmd", true},
		{"line", "//line :1", true},
		{"line file", "//line f:1", true},
		{"line file col", "//line f:col:", true},
		{"line file block", "/*line f:1", true},
		{"line file col block", "/*line f:col:", true},

		{"invalid go", "// go:generate cmd", false},
		{"invalid line", "//line a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDirective(tt.arg); got != tt.want {
				t.Errorf("isDirective() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Trim(t *testing.T) {
	src, err := os.ReadFile("testdata/a.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "testdata/a.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	Trim(f)

	var dest strings.Builder
	err = format.Node(&dest, fset, f)
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile("testdata/a-trimmed.go")
	if err != nil {
		t.Fatal(err)
	}

	if got := dest.String(); got != string(want) {
		t.Fatalf("want %v\ngot %v\n", string(want), got)
	}
}
