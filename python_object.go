package main

/*
#include "starlark.h"

extern PyObject *StarlarkError;
extern PyObject *SyntaxError;
extern PyObject *EvalError;
extern PyObject *ResolveError;
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
	"sync"
	"unsafe"

	"go.starlark.net/resolve"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type StarlarkState struct {
	Globals starlark.StringDict
	Mutex   sync.RWMutex
	Print   *C.PyObject
	Load    *C.PyObject
}

//export ConfigureStarlark
func ConfigureStarlark(allowSet C.int, allowGlobalReassign C.int, allowRecursion C.int) {
	// Ignore input values other than 0 or 1 and leave current value in place
	switch allowSet {
	case 0:
		resolve.AllowSet = false
	case 1:
		resolve.AllowSet = true
	}

	switch allowGlobalReassign {
	case 0:
		resolve.AllowGlobalReassign = false
	case 1:
		resolve.AllowGlobalReassign = true
	}

	switch allowRecursion {
	case 0:
		resolve.AllowRecursion = false
	case 1:
		resolve.AllowRecursion = true
	}
}

func rlockSelf(self *C.Starlark) *StarlarkState {
	state := cgo.Handle(self.handle).Value().(*StarlarkState)
	state.Mutex.RLock()
	return state
}

func lockSelf(self *C.Starlark) *StarlarkState {
	state := cgo.Handle(self.handle).Value().(*StarlarkState)
	state.Mutex.Lock()
	return state
}

//export Starlark_new
func Starlark_new(pytype *C.PyTypeObject, args *C.PyObject, kwargs *C.PyObject) *C.Starlark {
	self := C.starlarkAlloc(pytype)
	if self == nil {
		return nil
	}

	state := &StarlarkState{Globals: starlark.StringDict{
		"struct": starlark.NewBuiltin("struct", starlarkstruct.Make),
	}, Mutex: sync.RWMutex{}, Print: nil}
	self.handle = C.uintptr_t(cgo.NewHandle(state))

	return self
}

//export Starlark_init
func Starlark_init(self *C.Starlark, args *C.PyObject, kwargs *C.PyObject) C.int {
	var globals *C.PyObject = nil
	var print *C.PyObject = nil
	var load *C.PyObject = nil

	if C.parseInitArgs(args, kwargs, &globals, &print, &load) == 0 {
		return -1
	}

	if print != nil {
		if Starlark_set_print(self, print, nil) != 0 {
			return -1
		}
	}

	if load != nil {
		if Starlark_set_load(self, load, nil) != 0 {
			return -1
		}
	}

	if globals != nil {
		if C.PyMapping_Check(globals) != 1 {
			errmsg := C.CString(fmt.Sprintf("Can't initialize globals from %s", C.GoString(globals.ob_type.tp_name)))
			defer C.free(unsafe.Pointer(errmsg))
			C.PyErr_SetString(C.PyExc_TypeError, errmsg)
			return -1
		}

		retval := Starlark_set_globals(self, args, globals)
		if retval == nil {
			return -1
		}
	}

	return 0
}

//export Starlark_dealloc
func Starlark_dealloc(self *C.Starlark) {
	handle := cgo.Handle(self.handle)
	state := handle.Value().(*StarlarkState)

	handle.Delete()

	state.Mutex.Lock()
	defer state.Mutex.Unlock()

	if state.Print != nil {
		C.Py_DecRef(state.Print)
	}
	if state.Load != nil {
		C.Py_DecRef(state.Load)
	}

	C.starlarkFree(self)
}

func main() {}
