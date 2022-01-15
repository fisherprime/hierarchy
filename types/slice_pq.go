//go:build pq

// SPDX-License-Identifier: NONE
package types

import (
	"fmt"

	"github.com/lib/pq"
)

type (
	// StoreIDList represents a list of PostgreSQL `ID`s.
	StoreIDList pq.Int64Array
)

// ToStringSlice for `StoreIDList`.
func (sl *StoreIDList) ToStringSlice(dst *StringSlice) {
	for index := range *sl {
		(*dst) = append((*dst), fmt.Sprint(((*sl)[index])))
	}
}

// HasRole checks for the existence of a store-type ID.
func (sl *StoreIDList) HasRole(n uint) (resl int) {
	role := int64(n)
	for index := range *sl {
		if (*sl)[index] == role {
			resl = index
			return
		}
	}

	return
}
