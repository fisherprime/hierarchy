// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// REF: https://www.geeksforgeeks.org/generic-tree-level-order-traversal
//
// REF: https://www.geeksforgeeks.org/serialize-deserialize-n-ary-tree
//
// runes are singe UTF-8 codepoints, not used as the hierarchy expects strings.

type (
	// Hierarchy defines an n-array tree to hold hierarchies.
	//
	// Synchronization is unnecessary, the type is designed for single write multiple read.
	Hierarchy struct {
		// parent contains a reference to the upper Hierarchy.
		parent *Hierarchy

		// children holds references to nodes at a lower level.
		children childMap

		// value contains the node's data.
		value string
	}

	// List is a type wrapper for []*Hierarchy.
	List []*Hierarchy

	childMap map[string]*Hierarchy

	// TraverseChan defines a channel to communicate info between Hierarchy operations & it's callers.
	TraverseChan struct {
		node     *Hierarchy
		err      error
		newPeers bool
	}
)

const (
	initialQueueLen    = 0
	traverseBufferSize = 10

	notChildFmt = "(%s) %w (%s)"
)

// Errors encountered when handling a Hierarchy.
var (
	ErrNotFound = errors.New("not found")

	ErrNoLeaves     = errors.New("lacks leaves; tree is cyclic")
	ErrAlreadyChild = errors.New("is a child of")
	ErrNoChildren   = errors.New("lacks children ")
	ErrNotChild     = errors.New("is not a child of")
)

var fLogger logrus.FieldLogger = logrus.NewEntry(logrus.New())

// SetLogger configures a logrus.FieldLogger for the package.
func SetLogger(l logrus.FieldLogger) { fLogger = l }

// New instantiates a Hierarchy.
func New(value string) *Hierarchy {
	return &Hierarchy{
		value:    value,
		children: make(childMap),
	}
}

// Value retrieves the Hierarchy's data.
func (h *Hierarchy) Value() string { return h.value }

// Parent retrieves the Hierarchy's parent reference.
func (h *Hierarchy) Parent() *Hierarchy { return h.parent }

// SetParent for a Hierarchy.
func (h *Hierarchy) SetParent(parent *Hierarchy) { h.parent = parent }

// HasChild checks for the existence of an immediate child.
func (h *Hierarchy) HasChild(_ context.Context, childID string) (ok bool) {
	_, ok = h.children[childID]
	return
}

// AddChild to a Hierarchy.
func (h *Hierarchy) AddChild(ctx context.Context, child *Hierarchy) (err error) {
	// Search for existing immediate child.
	if h.HasChild(ctx, child.value) {
		err = fmt.Errorf("(%s) %w (%s)", child.value, ErrAlreadyChild, h.value)
		return
	}

	child.parent = h
	h.children[child.value] = child

	return
}

// AddChildTo to a Hierarchy.
func (h *Hierarchy) AddChildTo(ctx context.Context, parentID string, child *Hierarchy) (err error) {
	var parent *Hierarchy
	if parent, err = h.Locate(ctx, parentID); err != nil {
		err = fmt.Errorf("parent (%s) %w", parentID, err)
		return
	}

	return parent.AddChild(ctx, child)
}

// PopChild removes an immediate child to the Hierarchy.
func (h *Hierarchy) PopChild(ctx context.Context, childID string) (child *Hierarchy, err error) {
	if !h.HasChild(ctx, childID) {
		err = fmt.Errorf("child (%s) of (%s): %w", childID, h.value, ErrNotFound)
		return
	}

	child = h.children[childID]
	delete(h.children, childID)

	return
}

// Children lists the immediate children for a Hierarchy.
func (h *Hierarchy) Children(ctx context.Context) (children []string) {
	children = make([]string, len(h.children))

	index := 0
	for key := range h.children {
		children[index] = h.children[key].value
		index++
	}

	return
}

// AllChildren for a Hierarchy.
//
// NOTE: This operation is expensive.
func (h *Hierarchy) AllChildren(ctx context.Context) (children []string, err error) {
	children = make([]string, 0)
	hierChan := make(chan TraverseChan, traverseBufferSize)

	go h.Walk(ctx, hierChan)

	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if resl.err != nil {
			err = resl.err
			return
		}

		children = append(children, resl.node.value)
	}

	fLogger.Debugf("Hierarchy walk: %+v", children)

	if len(children) > 0 {
		// Omit self from the list.
		children = (children)[1:]

		fLogger.Debugf("children: %+v", children)
	}

	if len(children) < 1 {
		err = ErrNoChildren
	}

	return
}

// AllChildrenByLevel for a Hierarchy.
//
// NOTE: This operation is expensive.
func (h *Hierarchy) AllChildrenByLevel(ctx context.Context) (children [][]string, err error) {
	children = make([][]string, 0)
	hierChan := make(chan TraverseChan, traverseBufferSize)

	go h.Walk(ctx, hierChan)

	var peers []string
	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if err = resl.err; err != nil {
			return
		}

		if !resl.newPeers {
			peers = append(peers, resl.node.value)
			continue
		}

		if len(peers) > 0 {
			children = append(children, peers)
		}
		peers = []string{resl.node.value}
	}

	if len(peers) > 0 {
		children = append(children, peers)
	}

	fLogger.Debugf("Hierarchy walk: %+v", children)

	if len(children) > 0 {
		// Omit self from the list.
		children = (children)[1:]
		fLogger.Debugf("children: %+v", children)
	}

	if len(children) < 1 {
		err = ErrNoChildren
	}

	return
}

// AllChildrenOf returns a list of children as defined in Hierarchy for some parent Hierarchy.
func (h *Hierarchy) AllChildrenOf(ctx context.Context, parentID string) (children []string, err error) {
	node, err := h.Locate(ctx, parentID)
	if err != nil {
		return
	}

	return node.AllChildren(ctx)
}

// AllChildrenOfByLevel returns an array-of-arrays of children as defined in Hierarchy for some
// parent Hierarchy.
func (h *Hierarchy) AllChildrenOfByLevel(ctx context.Context, parentID string) (children [][]string, err error) {
	node, err := h.Locate(ctx, parentID)
	if err != nil {
		return
	}

	return node.AllChildrenByLevel(ctx)
}

// LeafNodes returns an array of terminal Hierarchy(ies).
func (h *Hierarchy) LeafNodes(ctx context.Context) (termNodes List, err error) {
	termNodes = make(List, 0)
	hierChan := make(chan TraverseChan, traverseBufferSize)

	go h.Walk(ctx, hierChan)

	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if err = resl.err; err != nil {
			return
		}

		if len(resl.node.children) < 1 {
			termNodes = append(termNodes, resl.node)
		}
	}

	if len(termNodes) < 1 {
		err = ErrNoLeaves
	}

	return
}

// Leaves returns an array of leaves (terminal node) values as defined in Hierarchy.
func (h *Hierarchy) Leaves(ctx context.Context) (termValues []string, err error) {
	nodes, err := h.LeafNodes(ctx)
	if err != nil {
		return
	}

	termValues = make([]string, len(nodes))
	for index := range nodes {
		termValues[index] = nodes[index].value
	}

	return
}

// ParentTo returns the parent Hierarchy for some node identified by its id.
func (h *Hierarchy) ParentTo(ctx context.Context, childID string) (parent *Hierarchy, err error) {
	var node *Hierarchy
	if node, err = h.Locate(ctx, childID); err != nil {
		err = fmt.Errorf("(%s) %w", childID, err)
		return
	}

	// Update the variable to hold the parent node.
	parent = node.parent

	return
}

// Locate searches for an id & returns it's Hierarchy.
func (h *Hierarchy) Locate(ctx context.Context, id string) (node *Hierarchy, err error) {
	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	default:
		if h.Value() == id {
			return h, nil
		}

		hierChan := make(chan TraverseChan)
		wg := new(sync.WaitGroup)
		wg.Add(1)
		go h.locate(ctx, id, hierChan, wg)
		go func() {
			wg.Wait()
			close(hierChan)
		}()

		resl, proceed := <-hierChan
		if !proceed {
			err = ErrNotFound
			return
		}

		if node, err = resl.node, resl.err; node != nil {
			return
		}

		err = ErrNotFound
	}

	return
}

func (h *Hierarchy) locate(ctx context.Context, id string, hierChan chan TraverseChan, wg *sync.WaitGroup) {
	defer wg.Done()
	// fLogger.Debugf("locate val %s in %+v", id, h)

	select {
	case <-ctx.Done():
		return
	default:
		if h.value == id {
			hierChan <- TraverseChan{node: h}
			return
		}

		if len(h.children) < 1 {
			return
		}

		if node, ok := h.children[id]; ok {
			hierChan <- TraverseChan{node: node}
			return
		}

		internalWG := new(sync.WaitGroup)
		internalWG.Add(len(h.children))
		for _, v := range h.children {
			go v.locate(ctx, id, hierChan, internalWG)
		}
		internalWG.Wait()
	}
}

// ChildTo searches for the child to some parent & returns it's Hierarchy.
func (h *Hierarchy) ChildTo(ctx context.Context, parentID, childID string) (child *Hierarchy, err error) {
	var parent *Hierarchy
	if parent, err = h.Locate(ctx, parentID); err != nil {
		err = fmt.Errorf("parent (%s) %w", parentID, err)
		return
	}

	if child, err = parent.Locate(ctx, parentID); err != nil {
		err = fmt.Errorf(notChildFmt, childID, ErrNotChild, parentID)
	}

	return
}

// Walk performs level-order traversal on a Hierarchy, pushing its values to its channel
// argument.
//
// This operation uses channels to minimize resource wastage.
// A context.Context is used to terminate the walk operation.
func (h *Hierarchy) Walk(ctx context.Context, hierChan chan TraverseChan) {
	defer close(hierChan)

	select {
	case <-ctx.Done():
		// Received context cancelation.
		return
	default:
		// Default operation is to walk.
		if h == nil {
			return
		}

		// Level order traversal.
		queue := List{h}

		for {
			qLen := len(queue)
			if qLen < 1 {
				break
			}

			// Iterate over the node's children.
			newPeers := true
			for qLen > 0 {
				// Pop from queue.
				var front *Hierarchy
				front, queue = queue[0], queue[1:]
				qLen--

				// Debug: this operation is noisy.
				// fLogger.Debugf("front: %+v, peers: %+v", front, newPeers)

				// Send node to caller via the channel.
				hierChan <- TraverseChan{node: front, newPeers: newPeers}
				newPeers = false

				// Add children to the queue.
				if len(front.children) < 1 {
					continue
				}

				for _, v := range front.children {
					queue = append(queue, v)
				}
			}
		}
	}
}

// Len is the number of elements in the collection.
func (l *List) Len() int { return len(*l) }

// Less reports whether the element with index i must sort before the element with index j.
func (l *List) Less(i int, j int) bool { return (*l)[i].value < (*l)[j].value }

// Swap swaps the elements with indexes i and j.
func (l *List) Swap(i int, j int) { (*l)[i], (*l)[j] = (*l)[j], (*l)[i] }
