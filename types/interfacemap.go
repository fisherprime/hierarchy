// SPDX-License-Identifier: NONE
package types

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/sirupsen/logrus"
)

type (
	// InterfaceMap contains JWT claims or HTTP request values.
	//
	// This map's values have to be `string`s to be storable in an AuthBoss managed session.
	InterfaceMap map[string]interface{}
)

const (
	ReadErrFmt = "failed to read (%s): %w"
)

var (
	lLogger *logrus.Logger
)

// Claims errors.
var (
	ErrInvalidType = errors.New("invalid data type")
)

func init() {
	lLogger = logrus.New()
}

func SetLogger(l *logrus.Logger) { lLogger = l }

// Add value to `InterfaceMap`.
func (a *InterfaceMap) Add(key string, val interface{}) { (*a)[key] = val }

// Delete from `InterfaceMap`.
func (a *InterfaceMap) Delete(key string) { delete((*a), key) }

// Get value from `InterfaceMap`.
func (a *InterfaceMap) Get(key string) (out string, ok bool) {
	var val interface{}
	if val, ok = (*a)[key]; ok {
		out = fmt.Sprint(val)
	}

	return
}

// LoadInterface from `InterfaceMap`.
func (a *InterfaceMap) LoadInterface(key string) (out interface{}, ok bool) {
	if out = (*a)[key]; out != nil {
		ok = true
	}

	return
}

// LoadString …
func (a *InterfaceMap) LoadString(key string) (result string, ok bool, err error) {
	var val interface{}
	if val, ok = (*a)[key]; !ok {
		return
	}

	if result, ok = val.(string); !ok {
		err = fmt.Errorf(ReadErrFmt, key, ErrInvalidType)
	}

	return
}

// LoadInt …
func (a *InterfaceMap) LoadInt(key string) (result int, ok bool, err error) {
	var val interface{}
	if val, ok = (*a)[key]; !ok {
		return
	}

	var id float64
	if id, ok = val.(float64); !ok {
		err = fmt.Errorf(ReadErrFmt, key, ErrInvalidType)
		return
	}
	result = int(id)

	return
}

// LoadUint …
func (a *InterfaceMap) LoadUint(key string) (result uint, ok bool, err error) {
	var val interface{}
	if val, ok = (*a)[key]; !ok {
		return
	}

	// Handle frontend requests.
	var id float64
	if id, ok = val.(float64); !ok {
		// Handle authorization claims.
		if result, ok = val.(uint); !ok {
			err = fmt.Errorf(ReadErrFmt, key, ErrInvalidType)
			return
		}
		return
	}
	result = uint(id)

	return
}

// LoadBool …
func (a *InterfaceMap) LoadBool(key string) (result bool, ok bool, err error) {
	var val interface{}
	if val, ok = (*a)[key]; !ok {
		return
	}

	if result, ok = val.(bool); !ok {
		err = fmt.Errorf(ReadErrFmt, key, ErrInvalidType)
	}

	return
}

// LoadStringSlice obtains an `[]interface{}` from `AuthClaims`.
func (a *InterfaceMap) LoadStringSlice(fieldName string) (result StringSlice, ok bool, err error) {
	var val interface{}
	if val, ok = (*a)[fieldName]; !ok {
		return
	}

	lLogger.Debugf("field: %s, type: %v, value: %v", fieldName, reflect.TypeOf(val), reflect.ValueOf(val))

	var iSlice []interface{}
	if iSlice, ok = val.([]interface{}); !ok {
		err = fmt.Errorf(ReadErrFmt, fieldName, ErrInvalidType)
		return
	}

	result = make(StringSlice, len(iSlice))
	for index := range iSlice {
		if result[index], ok = iSlice[index].(string); !ok {
			err = fmt.Errorf(ReadErrFmt, fieldName, ErrInvalidType)
			return
		}
	}

	return
}

// LoadUintSlice obtains a `[]uint` from `AuthClaims`.
func (a *InterfaceMap) LoadUintSlice(fieldName string) (result UintSlice, ok bool, err error) {
	var val interface{}
	if val, ok = (*a)[fieldName]; !ok {
		return
	}

	lLogger.Debugf("field: %s, type: %v, value: %v", fieldName, reflect.TypeOf(val), reflect.ValueOf(val))

	var iSlice []interface{}
	if iSlice, ok = val.([]interface{}); !ok {
		err = fmt.Errorf(ReadErrFmt, fieldName, ErrInvalidType)
		return
	}

	result = make(UintSlice, len(iSlice))

	var id float64
	for index := range iSlice {
		if id, ok = iSlice[index].(float64); !ok {
			err = fmt.Errorf(ReadErrFmt, fieldName, ErrInvalidType)
			return
		}
		result[index] = uint(id)
	}

	return
}

// Merge an `InterfaceMap` with the current one.
func (a *InterfaceMap) Merge(data InterfaceMap) {
	for k, v := range data {
		(*a)[k] = v
	}
}
