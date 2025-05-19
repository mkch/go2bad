package selection

import (
	"go/types"
	"testing"
)

func TestSelection(t *testing.T) {
	var s1 = newStruct()
	s1.AddField("n1")
	if i := method(s1, "a"); i != -1 {
		t.Fatal(i)
	}
	if i := field(s1, "n1"); i != 0 {
		t.Fatal(i)
	}

	var ps1 = newPtr(s1)
	if i := method(ps1, "a"); i != -1 {
		t.Fatal(i)
	}
	if i := field(ps1, "n1"); i != 0 {
		t.Fatal(i)
	}

	var pps1 = newPtr(ps1)
	if i := method(pps1, "a"); i != -1 {
		t.Fatal(i)
	}
	if i := field(pps1, "n1"); i != -1 {
		t.Fatal(i)
	}

	var d1 = newDefined(s1)
	d1.AddMethod("f")
	d1.AddPtrMethod("fp")
	if i := field(d1, "n1"); i != 0 {
		t.Fatal(i)
	}
	if i := method(d1, "f"); i != 0 {
		t.Fatal(i)
	}
	if i := method(d1, "fp"); i != -1 {
		t.Fatal(i)
	}

	var pd1 = newPtr(d1)
	if i := field(pd1, "n1"); i != 0 {
		t.Fatal(i)
	}
	if i := method(pd1, "f"); i != 0 {
		t.Fatal(i)
	}
	if i := method(pd1, "fp"); i != 0 {
		t.Fatal(i)
	}

	var dpd1 = newDefined(pd1)
	if i := field(dpd1, "n1"); i != 0 {
		t.Fatal(i)
	}
	if i := method(dpd1, "f"); i != -1 {
		t.Fatal(i)
	}
	if i := method(dpd1, "fp"); i != -1 {
		t.Fatal(i)
	}

	var d2 = newDefined(d1)
	d2.AddMethod("f2")
	d2.AddPtrMethod("fp2")
	if i := field(d2, "n1"); i != 0 {
		t.Fatal(i)
	}
	if i := method(d2, "f"); i != -1 {
		t.Fatal(i)
	}
	if i := method(d2, "fp"); i != -1 {
		t.Fatal(i)
	}
	if i := method(d2, "f2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(d2, "fp2"); i != -1 {
		t.Fatal(i)
	}

}

func TestPrompt(t *testing.T) {
	var s1 = newStruct()
	s1.AddField("n1")
	var d1 = newDefined(s1)
	d1.AddMethod("f1")
	d1.AddPtrMethod("fp1")

	var s2 = newStruct()
	s2.AddField("n2")
	s2.AddEmbedded("d1", d1)
	if i := field(s2, "d1"); i != 0 {
		t.Fatal(i)
	}
	if i := field(s2, "n1"); i != 1 {
		t.Fatal(i)
	}
	if i := field(s2, "n2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(s2, "f1"); i != 1 {
		t.Fatal(i)
	}
	if i := method(s2, "pf1"); i != -1 {
		t.Fatal(i)
	}

	var ps2 = newPtr(s2)
	if i := field(ps2, "d1"); i != 0 {
		t.Fatal(i)
	}
	if i := field(ps2, "n1"); i != 1 {
		t.Fatal(i)
	}
	if i := field(ps2, "n2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(ps2, "f1"); i != 1 {
		t.Fatal(i)
	}
	if i := method(ps2, "fp1"); i != 1 {
		t.Fatal(i)
	}

	var d2 = newDefined(s2)
	d2.AddMethod("f2")
	d2.AddPtrMethod("fp2")
	if i := field(d2, "d1"); i != 0 {
		t.Fatal(i)
	}
	if i := field(d2, "n1"); i != 1 {
		t.Fatal(i)
	}
	if i := field(d2, "n2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(d2, "f2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(d2, "fp2"); i != -1 {
		t.Fatal(i)
	}
	if i := method(d2, "f1"); i != 1 {
		t.Fatal(i)
	}
	if i := method(d2, "fp1"); i != -1 {
		t.Fatal(i)
	}

	var pd2 = newPtr(d2)
	if i := field(pd2, "d1"); i != 0 {
		t.Fatal(i)
	}
	if i := field(pd2, "n1"); i != 1 {
		t.Fatal(i)
	}
	if i := field(pd2, "n2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(pd2, "f2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(pd2, "fp2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(pd2, "f1"); i != 1 {
		t.Fatal(i)
	}
	if i := method(pd2, "fp1"); i != 1 {
		t.Fatal(i)
	}

	var s3 = newStruct()
	s3.AddEmbedded("d1", newPtr(d1))
	if i := field(s3, "n1"); i != 1 {
		t.Fatal(i)
	}
	var d3 = newDefined(s3)
	if i := method(d3, "f1"); i != 1 {
		t.Fatal(i)
	}
	if i := method(d3, "fp1"); i != 1 {
		t.Fatal(i)
	}

	var pd3 = newPtr(d3)
	if i := method(pd3, "f1"); i != 1 {
		t.Fatal(i)
	}
	if i := method(pd3, "fp1"); i != 1 {
		t.Fatal(i)
	}
}

func TestIface(t *testing.T) {
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	var i1 = newIface()
	i1.AddMethod("f1", sig)

	var i2 = newIface()
	i2.AddMethod("f2", sig)
	i2.AddEmbedded(i1)

	var i3 = newIface()
	i3.AddMethod("f3", sig)
	i3.AddEmbedded(newDefined(i2))
	if i := field(i3, "a"); i != -1 {
		t.Fatal(i)
	}
	if i := method(i3, "f1"); i != 0 {
		t.Fatal(i)
	}
	if i := method(i3, "f2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(i3, "f3"); i != 0 {
		t.Fatal(i)
	}
	if can := i3.CanRenameTo("f3", "f2"); !can {
		// duplicated methods in an interface
		//  and its embeds with the same signature is allowed.
		t.Fatal(can)
	}
	if can := i3.CanRenameTo("f3", "fff"); !can {
		t.Fatal(can)
	}

	var d3 = newDefined(i3)
	if i := field(d3, "a"); i != -1 {
		t.Fatal(i)
	}
	if i := method(d3, "f1"); i != 0 {
		t.Fatal(i)
	}
	if i := method(d3, "f2"); i != 0 {
		t.Fatal(i)
	}
	if i := method(d3, "f3"); i != 0 {
		t.Fatal(i)
	}

	var p3 = newPtr(i3)
	if i := method(p3, "f1"); i != -1 {
		t.Fatal(i)
	}

	var dp3 = newDefined(p3)
	if i := method(dp3, "f1"); i != -1 {
		t.Fatal(i)
	}
}

func Test_recursive(t *testing.T) {
	var s1 = newStruct()
	var d1 = newDefined(s1)
	s1.AddEmbedded("d1", d1)

	if i := method(d1, "a"); i != -1 {
		t.Fatal(i)
	}
}
