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

// Claims errors.
var (
	ErrMissingClaims = errors.New("claims missing from the context")
	ErrInvalidType   = errors.New("invalid data type")
)

var (
	// ReadClaimsErrFmt defines the format for the `ErrMissingClaims` message.
	ReadClaimsErrFmt = "failed to read claims (%s): %w"

	lLogger *logrus.Logger
)

func init() {
	lLogger = logrus.New()
}

func SetLogger(l *logrus.Logger) { lLogger = l }

// Add value to `InterfaceMap`.
func (a *InterfaceMap) Add(key string, val interface{}) { (*a)[key] = val }

// GetInterface from `InterfaceMap`.
func (a *InterfaceMap) GetInterface(key string) (out interface{}, ok bool) {
	if out = (*a)[key]; out != nil {
		ok = true
	}

	return
}

// Delete from `InterfaceMap`.
func (a *InterfaceMap) Delete(key string) { delete((*a), key) }

// Get value from `InterfaceMap`.
//
// Implements the `authboss.ClientState` interface.
func (a *InterfaceMap) Get(key string) (out string, ok bool) {
	var val interface{}
	if val, ok = (*a)[key]; ok {
		out = fmt.Sprint(val)
	}

	return
}

// GetValString …
func (a *InterfaceMap) GetValString(key string) (strVal string, err error) {
	if val, ok := (*a)[key]; ok {
		if strVal, ok = val.(string); !ok {
			err = fmt.Errorf(ReadErrFmt, key, ErrInvalidType)
		}
	}

	return
}

// GetValInt …
func (a *InterfaceMap) GetValInt(key string) (intVal int, err error) {
	if val, ok := (*a)[key]; ok {
		var id float64
		if id, ok = val.(float64); !ok {
			err = fmt.Errorf(ReadErrFmt, key, ErrInvalidType)
			return
		}
		intVal = int(id)
	}

	return
}

// GetValUint …
func (a *InterfaceMap) GetValUint(key string) (uintVal uint, err error) {
	if val, ok := (*a)[key]; ok {
		// Handle frontend requests.
		var id float64
		if id, ok = val.(float64); !ok {
			// Handle authorization claims.
			if uintVal, ok = val.(uint); !ok {
				err = fmt.Errorf(ReadErrFmt, key, ErrInvalidType)
				return
			}
			return
		}
		uintVal = uint(id)
	}

	return
}

// GetValBool …
func (a *InterfaceMap) GetValBool(key string) (boolVal bool, err error) {
	if val, ok := (*a)[key]; ok {
		if boolVal, ok = val.(bool); !ok {
			err = fmt.Errorf(ReadErrFmt, key, ErrInvalidType)
		}
	}

	return
}

// GetStringSlice obtains an `[]interface{}` from `AuthClaims`.
func (a *InterfaceMap) GetStringSlice(fieldName string) (result StringSlice, err error) {
	val, ok := (*a)[fieldName]
	if !ok || val == nil {
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

// GetUintSlice obtains a `[]uint` from `AuthClaims`.
func (a *InterfaceMap) GetUintSlice(fieldName string) (result UintSlice, err error) {
	val, ok := (*a)[fieldName]
	if !ok || val == nil {
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
