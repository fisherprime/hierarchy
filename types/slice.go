// SPDX-License-Identifier: NONE
package types

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type (
	// ByteSliceis a `[]byte`
	ByteSlice []byte

	// StringSlice for `string`.
	StringSlice []string

	// UintSlice for `uint`.
	UintSlice []uint
)

// Validation errors.
var (
	ErrInvalidIndex = errors.New("invalid index")
)

// Locate for `UintSlice`.
func (sl *UintSlice) Locate(val uint) (resl int) {
	resl = -1

	for index := range *sl {
		if (*sl)[index] == val {
			resl = index
			return
		}
	}

	return
}

// ToStringSlice for `UintSlice`.
func (sl *UintSlice) ToStringSlice(dst *StringSlice) {
	for index := range *sl {
		(*dst) = append((*dst), fmt.Sprint(((*sl)[index])))
	}
}

// String is the `fmt.Stringer` interface implementation for `UintSlice`
func (sl *UintSlice) String() (dst string) {
	lenSl := len(*sl)
	if lenSl > 0 {
		buffer := strings.Builder{}
		fmt.Fprintf(&buffer, "[%d", (*sl)[0])
		for index := 1; index < lenSl; index++ {
			fmt.Fprintf(&buffer, ",%d", (*sl)[index])
		}
		buffer.WriteString("]")

		dst = buffer.String()
	}

	return
}

// Sort for `UintSlice`.
func (sl *UintSlice) Sort() {
	sort.Slice(*sl, func(i, j int) bool { return (*sl)[i] < (*sl)[j] })
}

// Sort for `StringSlice`.
func (sl *StringSlice) Sort() {
	sort.Strings(*sl)
	// sort.Slice(*sl, func(i, j int) bool {return ()})
}

// Locate for `StringSlice`.
func (sl *StringSlice) Locate(val string) (resl int) {
	resl = -1

	for index := range *sl {
		if (*sl)[index] == val {
			resl = index
			return
		}
	}

	return
}

// UniquePrepend to `StringSlice`.
func (sl *StringSlice) UniquePrepend(values ...string) {
	if len(values) < 1 {
		return
	}

	for index := range values {
		newValue := values[index]
		if sl.Locate(newValue) > -1 {
			continue
		}

		*sl = append(StringSlice{newValue}, *sl...)
	}
}

// UniqueAppend to `StringSlice`.
func (sl *StringSlice) UniqueAppend(values ...string) {
	if len(values) < 1 {
		return
	}

	for index := range values {
		newValue := values[index]
		if sl.Locate(newValue) > -1 {
			continue
		}

		*sl = append(*sl, newValue)
	}
}

// ToByteSlice converts a `StringSlice` to a `[][]byte`.
func (sl *StringSlice) ToByteSlice(output *[][]byte) {
	for index := range *sl {
		*output = append(*output, ByteSlice((*sl)[index]))
	}
}

// ToUintSlice for `StringSlice`.
func (sl *StringSlice) ToUintSlice(dst *UintSlice) (err error) {
	var val uint64
	for index := range *sl {
		if val, err = strconv.ParseUint((*sl)[index], 10, 64); err != nil {
			return
		}
		(*dst) = append((*dst), uint(val))
	}

	return
}

// Pop from `StringSlice`.
func (sl *StringSlice) Pop(values ...string) {
	for index := range values {
		if loc := sl.Locate(values[index]); loc > -1 {
			*sl = append((*sl)[:loc], (*sl)[loc:]...)
		}
	}
}
