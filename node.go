// SPDX-License-Identifier: NONE
package hierarchy

import "context"

type (
	// Node defines an n-array tree to hold hierarchies.
	//
	// Synchronization is unnecessary, the type is designed for single write multiple read.
	Node struct {
		rootValue string

		// value contains the node's data.
		value string

		// parent contains a reference to the upper `Node`.
		parent *Node

		// children holds references to nodes at a lower level.
		children childMap
	}

	childMap map[string]*Node
)

// New instantiates a `Node`.
func New(_ context.Context, value string, rootValue ...string) *Node {
	rVal := DefaultRootValue
	if len(rootValue) == 1 {
		rVal = rootValue[0]
	}

	return &Node{
		rootValue: rVal,
		value:     value,
		// children:  make(childMap), // Allocated by `AddChild`.
	}
}

// Value retrieves the `Node`'s data.
func (h *Node) Value() string { return h.value }

// Parent retrieves the `Node`'s parent reference.
func (h *Node) Parent() *Node { return h.parent }

// SetParent for a `Node`.
func (h *Node) SetParent(parent *Node) { h.parent = parent }
