package main

/*
#include "starlark.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"go.starlark.net/starlark"
)

//export Starlark_get_load
func Starlark_get_load(self *C.Starlark, closure unsafe.Pointer) *C.PyObject {
	state := rlockSelf(self)
	if state == nil {
		return nil
	}
	defer state.Mutex.RUnlock()

	if state.Load == nil {
		return C.cgoPy_NewRef(C.Py_None)
	}

	return C.cgoPy_NewRef(state.Load)
}

//export Starlark_set_load
func Starlark_set_load(self *C.Starlark, value *C.PyObject, closure unsafe.Pointer) C.int {
	if value == C.Py_None {
		value = nil
	}

	if value != nil {
		if C.PyCallable_Check(value) != 1 {
			errmsg := C.CString(fmt.Sprintf("%s is not callable", C.GoString(value.ob_type.tp_name)))
			defer C.free(unsafe.Pointer(errmsg))
			C.PyErr_SetString(C.PyExc_TypeError, errmsg)
			return -1
		}
	}

	state := lockSelf(self)
	if state == nil {
		return -1
	}
	defer state.Mutex.Unlock()

	state.Load = C.cgoPy_NewRef(value)
	return 0
}

func pythonLoad(self *C.Starlark, load *C.PyObject) *C.PyObject {
	if load == nil {
		state := rlockSelf(self)
		if state == nil {
			return nil
		}
		defer state.Mutex.RUnlock()
		load = state.Load
	}

	if load != nil {
		if C.PyCallable_Check(load) != 1 {
			errmsg := C.CString(fmt.Sprintf("%s is not callable", C.GoString(load.ob_type.tp_name)))
			defer C.free(unsafe.Pointer(errmsg))
			C.PyErr_SetString(C.PyExc_TypeError, errmsg)
			return nil
		}
	}

	return load
}

// Invoke the Python-provided "load" function from Starlark
func callPythonLoad(load *C.PyObject, filename string, parentFilename *string) (starlark.StringDict, error) {
	// cfilename := C.CString(filename)
	// defer C.free(unsafe.Pointer(cfilename))
	// pyfilename := C.cgoPy_BuildString(cfilename)

	var starlarkParentFilename starlark.Value
	if parentFilename != nil {
		starlarkParentFilename = starlark.String(*parentFilename)
	} else {
		starlarkParentFilename = starlark.None
	}
	args, err := starlarkTupleToPython(starlark.Tuple{
		starlark.String(filename),
		starlarkParentFilename,
	})
	if err != nil {
		return nil, err
	}

	// args := C.PyTuple_New(2)
	// defer C.Py_DecRef(args)

	// if C.PyTuple_SetItem(args, 0, pyfilename) != 0 {
	// 	C.Py_DecRef(pyfilename)
	// 	return nil, fmt.Errorf("error calling Python load function: cannot set argument 0")
	// }

	result := C.PyObject_CallObject(load, args)
	if result == nil {
		if C.PyErr_Occurred() != nil {
			C.PyErr_Print()
		}
		return nil, fmt.Errorf("error calling Python load function")
	}
	defer C.Py_DecRef(result)

	// expects a starlark instance
	if C.cgoPyStarlarkInstance_Check(result) != 1 {
		return nil, fmt.Errorf("error calling Python load function: expected a starlark instance, got %T", result)
	}

	starlark := (*C.Starlark)(unsafe.Pointer(result))
	state := rlockSelf(starlark)
	defer state.Mutex.RUnlock()

	return state.Globals, nil
}
