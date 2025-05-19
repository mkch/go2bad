package scope

import "github.com/mkch/gg/slices2"

// multiMap is a generic map that associates string keys with slices of values of type T.
type multiMap[T any] map[string][]T

// Lookup returns the values associated with the given name.
func (m multiMap[T]) Lookup(name string) []T {
	return m[name]
}

// LookupFunc returns a filtered slice of values associated with the given name.
func (m multiMap[T]) LookupFunc(name string, f func(pos T) bool) []T {
	return slices2.Filter(m.Lookup(name), f)
}

// Add appends one or more values to the slice associated with the given name.
func (m multiMap[T]) Add(name string, pos ...T) {
	old := m.Lookup(name)
	m[name] = append(old, pos...)
}
