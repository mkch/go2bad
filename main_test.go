package main

import (
	"testing"
)

func Test_internalPos(t *testing.T) {
	type args struct {
		pkgPath string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"leading", args{"internal"}, false},
		{"tailing", args{"a/internal"}, true},
		{"middle", args{"a/internal/b"}, true},
		{"multi_internal", args{"a/internal/b/internal"}, true},
		{"empty", args{""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := internalPos(tt.args.pkgPath) > 0; got != tt.want {
				t.Errorf("isInternalPackage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_canImport(t *testing.T) {
	type args struct {
		internalPkg string
		pkg         string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"tailing", args{"a/b/internal", "a/b"}, true},
		{"extra", args{"a/b/internal", "a/b/c"}, true},
		{"extra_internal", args{"a/b/internal", "a/b/internal/c"}, true},
		{"extra_multi_internal", args{"a/b/internal/c/internal/d", "a/b/internal/c"}, true},
		{"can't", args{"a/b/internal/c/internal/d", "a/b/c"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canImport(tt.args.internalPkg, tt.args.pkg); got != tt.want {
				t.Errorf("canImport() = %v, want %v", got, tt.want)
			}
		})
	}
}
