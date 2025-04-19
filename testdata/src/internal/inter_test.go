package internal

import ttt "testing"

func TestInterfaceImpl(t *ttt.T) {
	var i unexportedImpl
	unexportedInterface(i).method()
}
