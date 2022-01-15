// SPDX-License-Identifier: NONE
package hierarchy

type (
	// Node defines an n-array tree to hold hierarchies.
	//
	// NOTE: Adding synchronization to this structure is not feasible as of 2021-05-28T07:47:29+0300.
	Node struct {
		id string

		// parent holds the node at the upper level.
		parent *Node

		// children holds nodes at a lower level.
		children childMap
	}

	childMap map[string]*Node
)

// GetID from a `Node`.
func (h *Node) GetID() string { return h.id }

// GetParent from a `Node`.
func (h *Node) GetParent() *Node { return h.parent }

func (h *Node) SetParent(parent *Node) { h.parent = parent }
