package flags

import (
	"slices"
	"strings"
	"testing"
)

func Test_seesFlag(t *testing.T) {
	var flag seedsFlag
	flag.Set("a bc123")
	flag.Set("中文")
	want := []string{
		"a", "b", "c", "1", "2", "3", "中", "文",
	}
	if got := []string(flag); !slices.Equal(got, want) {
		t.Fatal(got)
	}
	if got := flag.String(); got != "abc123中文" {
		t.Fatal(got)
	}
}

func Test_parseKeepFlag(t *testing.T) {
	type args struct {
		value string
	}
	tests := []struct {
		name     string
		args     args
		wantPkg  string
		wantName string
	}{
		{"with_path", args{"a.com/path/pkg.Name"}, "a.com/path/pkg", "Name"},
		{"no_path", args{"pkg.name"}, "pkg", "name"},
		{"wrong_path", args{"/pkg.name"}, "", ""},
		{"wrong_path", args{"a//pkg.name"}, "", ""},
		{"wrong_path", args{"a pkg.name"}, "", ""},
		{"wrong_path", args{"pkg.0name"}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPkg, gotName := parseKeepFlag(tt.args.value)
			if gotPkg != tt.wantPkg {
				t.Errorf("parseKeepFlag() gotPkg = %v, want %v", gotPkg, tt.wantPkg)
			}
			if gotName != tt.wantName {
				t.Errorf("parseKeepFlag() gotName = %v, want %v", gotName, tt.wantName)
			}
		})
	}
}

func Test_keepFlags_Set(t *testing.T) {
	var flag keepFlag
	if !flag.Empty() {
		t.Fatal("should be empty")
	}

	flag.Set("path/pkg1.Name1")
	flag.Set("pkg1.Name2,pkg1.Name1,Name2")
	flag.Set("Name2,Name1,Name2,pkg2.Name1,path/pkg1.Name1")

	if flag.Empty() {
		t.Fatal("should not be empty")
	}

	sortStr := func(str string) string {
		wantSlice := strings.Split(str, ",")
		slices.Sort(wantSlice)
		return strings.Join(wantSlice, ",")
	}

	want := sortStr("Name1,Name2,pkg1.Name1,pkg1.Name2,pkg2.Name1,path/pkg1.Name1")
	got := sortStr(flag.String())
	if want != got {
		t.Fatalf("want %v, got %v", want, got)
	}

	if !flag.Contains("any", "Name1") {
		t.Fatal("Name1")
	}
	if !flag.Contains("any", "Name2") {
		t.Fatal("Name2")
	}
	if flag.Contains("pkg1", "Name3") {
		t.Fatal("Name3")
	}

	if !flag.Contains("pkg1", "Name1") {
		t.Fatal("pkg1.Name1")
	}
	if !flag.Contains("pkg1", "Name2") {
		t.Fatal("pkg1.Name2")
	}

	if !flag.Contains("path/pkg1", "Name1") {
		t.Fatal("path/pkg1.Name1")
	}

	if !flag.Contains("pkg2", "Name1") {
		t.Fatal("pkg2.Name1")
	}
}
