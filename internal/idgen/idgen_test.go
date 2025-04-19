package idgen

import (
	"testing"

	"github.com/mkch/gg"
)

func Test_New_exported(t *testing.T) {
	next := NewGenerator("A", "b", "0").NewExported(gg.Set[string]{"A": struct{}{}})

	if id := next(); id != "AA" {
		t.Fatal(id)
	}

	if id := next(); id != "Ab" {
		t.Fatal(id)
	}

	if id := next(); id != "A0" {
		t.Fatal(id)
	}

	if id := next(); id != "AAA" {
		t.Fatal(id)
	}

	if id := next(); id != "AAb" {
		t.Fatal(id)
	}

	if id := next(); id != "AA0" {
		t.Fatal(id)
	}
}

func Test_New_unexported(t *testing.T) {
	next := NewGenerator("A", "0").NewUnexported(nil)

	if id := next(); id != "_" {
		t.Fatal(id)
	}

	if id := next(); id != "_A" {
		t.Fatal(id)
	}

	if id := next(); id != "_0" {
		t.Fatal(id)
	}

	if id := next(); id != "_AA" {
		t.Fatal(id)
	}

	if id := next(); id != "_A0" {
		t.Fatal(id)
	}

	if id := next(); id != "_0A" {
		t.Fatal(id)
	}

	if id := next(); id != "_00" {
		t.Fatal(id)
	}

	if id := next(); id != "_AAA" {
		t.Fatal(id)
	}
}
