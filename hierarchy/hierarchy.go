// SPDX-License-Identifier: NONE
package hierarchy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"gitlab.com/fisherprime/hierarchy/lexer"
	"gitlab.com/fisherprime/hierarchy/types"
)

// REF: https://www.geeksforgeeks.org/generic-tree-level-order-traversal
//
// REF: https://www.geeksforgeeks.org/serialize-deserialize-n-ary-tree
//
// `rune`s are singe `UTF-8` codepoints, not used as the hierarchy expects strings.

type (
	// TraverseChan defines a channel to communicate info between `Node`
	// operations & it's callers.
	TraverseChan struct {
		newPeers bool
		node     *Node
		err      error
	}
)

const (
	// RootID is the identifier defaulted as a root node.
	RootID = "0"

	initialQueueLen = 0

	traverseBufferSize = 10

	// endMarker a `rune` indicating the end of a node's children.
	endMarker = ')'

	// valueSplitter is the character used to split the `Node` serialization output.
	valueSplitter = ','

	notChildFmt = "(%s) %w (%s)"
)

// Errors codes encountered when handling a `Node`.
var (
	ErrAlreadyChild            = errors.New("is a child of")
	ErrEmptyDeserializationSrc = errors.New("empty Node deserialization source")
	ErrIDNotFound              = errors.New("not found")
	ErrNoChildren              = errors.New("lacks children ")
	ErrNoParents               = errors.New("lacks parents")
	ErrNoTerminalNodes         = errors.New("lacks terminal nodes; tree is cyclic")
	ErrNotChild                = errors.New("is not a child of")
)

var (
	lLogger = logrus.New()
)

func SetLogger(l *logrus.Logger) { lLogger = l }

// New initiates a `Node`.
func New(_ context.Context, init string) *Node {
	return &Node{
		id:       init,
		children: make(childMap),
	}
}

// AddChild to a `Node`.
func (h *Node) AddChild(ctx context.Context, child *Node) (err error) {
	// Search for existing immediate child.
	if h.HasChild(ctx, child.id) > 0 {
		err = fmt.Errorf("(%s) %w (%s)", child.id, ErrAlreadyChild, h.id)
		return
	}

	child.parent = h
	h.children[child.id] = child

	return
}

// AddChildTo to a `Node`.
func (h *Node) AddChildTo(ctx context.Context, parentID string, child *Node) (err error) {
	var parent *Node
	if parent, err = h.Locate(ctx, parentID); err != nil {
		err = fmt.Errorf("parent (%s) %w", parentID, err)
		return
	}

	return parent.AddChild(ctx, child)
}

// PopChild removes an immediate child to the `Node`.
func (h *Node) PopChild(ctx context.Context, childID string) (child *Node, err error) {
	var index int
	if index = h.HasChild(ctx, childID); index < 0 {
		err = fmt.Errorf("child (%s) of (%s): %s", childID, h.id, ErrIDNotFound)
		return
	}

	child = h.children[childID]
	delete(h.children, childID)

	return
}

// HasChild checks for the existence of an immediate child.
func (h *Node) HasChild(_ context.Context, childID string) (index int) {
	index = -1
	if _, ok := h.children[childID]; ok {
		index = 1
	}

	return
}

// ListImmediateSubordinates lists the immediate children for a `Node`.
func (h *Node) ListImmediateChildren(ctx context.Context) (children *types.StringSlice) {
	*children = make(types.StringSlice, 0)
	for _, child := range h.children {
		*children = append(*children, child.id)
	}

	return
}

// ListChildrenOf returns a list of children as defined in `Node` for some parent
// model.
//
// NOTE: This operation is expensive.
func (h *Node) ListChildrenOf(ctx context.Context, parentID string) (children *types.StringSlice, err error) {
	children = new(types.StringSlice)
	hierChan := make(chan TraverseChan, traverseBufferSize)

	go h.WalkChildrenOf(ctx, parentID, hierChan)

	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if resl.err != nil {
			err = resl.err
			return
		}

		*children = append(*children, resl.node.id)
	}

	lLogger.Debugf("`Node` walk: %+v", *children)

	numChildren := len(*children)
	if numChildren > 0 {
		// Omit self from the list.
		*children = (*children)[1:]

		lLogger.Debugf("children: %+v", *children)
	}

	if len(*children) < 1 {
		err = ErrNoChildren
	}

	return
}

// ListChildrenOfByLevel returns an array-of-arrays of children as defined in
// `Node` for some parent model.
//
// NOTE: This operation will bottleneck list generation operations.
func (h *Node) ListChildrenOfByLevel(ctx context.Context, parentID string) (children *[]types.StringSlice, err error) {
	children = &[]types.StringSlice{}
	hierChan := make(chan TraverseChan, traverseBufferSize)

	go h.WalkChildrenOf(ctx, parentID, hierChan)

	var peers []string
	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if resl.err != nil {
			err = resl.err
			return
		}

		if resl.newPeers {
			if len(peers) > 0 {
				*children = append(*children, peers)
			}
			peers = make([]string, initialQueueLen)
		}
		peers = append(peers, resl.node.id)
	}
	if len(peers) > 0 {
		*children = append(*children, peers)
	}

	lLogger.Debugf("`Node` walk: %+v", *children)

	numChildren := len(*children)
	if numChildren > 0 {
		// Omit self from the list.
		*children = (*children)[1:]

		lLogger.Debugf("children: %+v", *children)
	}

	if len(*children) < 1 {
		err = ErrNoChildren
	}

	return
}

// ListTerminalNodes returns an array of terminal node data as defined in `Node`.
func (h *Node) ListTerminalNodes(ctx context.Context) (termItems *types.StringSlice, err error) {
	termItems = new(types.StringSlice)
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

		if resl.node.children == nil || len(resl.node.children) < 1 {
			*termItems = append(*termItems, resl.node.id)
		}
	}

	if len(*termItems) < 1 {
		err = ErrNoTerminalNodes
	}

	return
}

// GetParentTo returns the parent `Node` for some node identified by its `id`.
func (h *Node) GetParentTo(ctx context.Context, childID string) (parent *Node, err error) {
	var node *Node
	if node, err = h.Locate(ctx, childID); err != nil {
		err = fmt.Errorf("(%s) %w", childID, err)
		return
	}

	// Update the variable to hold the parent node.
	parent = node.parent

	return
}

// Locate searches for an `id` & returns it's `Node`.
func (h *Node) Locate(ctx context.Context, id string) (node *Node, err error) {
	select {
	case <-ctx.Done():
		return
	default:
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
			err = ErrIDNotFound
			return
		}

		if node, err = resl.node, resl.err; node != nil {
			return
		}

		err = ErrIDNotFound
	}

	return
}

func (h *Node) locate(ctx context.Context, id string, hierChan chan TraverseChan, wg *sync.WaitGroup) {
	defer wg.Done()
	// lLogger.Debugf("locate val %s in %+v", id, h)

	select {
	case <-ctx.Done():
		return
	default:
		if h.id == id {
			hierChan <- TraverseChan{node: h}
			return
		}

		if h.children == nil || len(h.children) < 1 {
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

// LocateChildTo searches for the child to some parent & returns it's `Node`.
func (h *Node) LocateChildTo(ctx context.Context, parentID, childID string) (child *Node, err error) {
	var parent *Node
	if parentID != RootID {
		if parent, err = h.Locate(ctx, parentID); err != nil {
			err = fmt.Errorf("parent (%s) %w", parentID, err)
			return
		}
	} else {
		parent = h
	}

	if child, err = parent.Locate(ctx, parentID); err != nil {
		err = fmt.Errorf(notChildFmt, childID, ErrNotChild, parentID)
	}

	return
}

// Serialize transforms a `Node` into a string.
func (h *Node) Serialize(ctx context.Context) (output string, err error) {
	select {
	case <-ctx.Done():
		return
	default:
		serChan := make(chan string)
		go func() {
			h.serialize(ctx, serChan)
			close(serChan)
		}()

		// Handle root `Node`.
		fVal, fProceed := <-serChan
		if !fProceed {
			return
		}
		var buffer strings.Builder
		if _, err = buffer.WriteString(fVal); err != nil {
			// Invalidate serialization output.
			return
		}

		for {
			val, proceed := <-serChan
			if !proceed {
				break
			}

			if val != string(endMarker) {
				if _, err = buffer.WriteString(string(valueSplitter)); err != nil {
					return
				}
			}
			if _, err = buffer.WriteString(val); err != nil {
				// Invalidate serialization output.
				return
			}
		}

		output = buffer.String()
	}

	return
}

// serialize performs the serialization grunt work.
func (h *Node) serialize(ctx context.Context, serChan chan string) {
	if h == nil || h.id == RootID {
		return
	}
	serChan <- h.id

	for _, child := range h.children {
		select {
		case <-ctx.Done():
			return
		default:
			child.serialize(ctx, serChan)
		}
	}
	serChan <- string(endMarker)
}

// Deserialize transforms a serialized `Node` into a `Node`.
//
// An invalid entry will result in a truncated `Node`.
func (h *Node) Deserialize(ctx context.Context, input string) (err error) {
	if input == "" {
		err = ErrEmptyDeserializationSrc
		return
	}

	select {
	case <-ctx.Done():
		return
	default:
		l := lexer.New(lLogger, input)
		go l.Lex(ctx)

		if _, err = h.deserialize(ctx, l); err != nil {
			err = fmt.Errorf("%w: %v", ErrInvalidHierarchySrc, err)
			return
		}

		diff := l.ValueCounter - l.EndCounter
		switch {
		case diff > 0:
			// Excessive values.
			err = fmt.Errorf("the serialized `Node` has values in excess by: %d", diff)
		case diff < 0:
			// Excessive end markers.
			err = fmt.Errorf("the serialized `Node` has end marker `%s` in excess by: %d", string(lexer.EndMarker), diff*-1)
		default:
			// Valid
		}
		if err != nil {
			return
		}

		children, _ := h.ListChildrenOfByLevel(ctx, RootID)
		lLogger.Debugf("hierarchy: %+v", children)
	}

	return
}

// deserialize performs the deserialization grunt work.
func (h *Node) deserialize(ctx context.Context, l *lexer.Lexer) (end bool, err error) {
	select {
	case <-ctx.Done():
		end = true
		return
	default:
		item, proceed := <-l.Item
		if !proceed {
			end = true
			return
		}

		lLogger.Debugf("lexed item: %+v", item)

		switch item.ID {
		case lexer.ItemEOF:
			end = true
			return
		case lexer.ItemError:
			// Stop input processing.
			end = true
			err = item.Err
			return
		case lexer.ItemEndMarker:
			end = true
			return
		case lexer.ItemSplitter:
			return
		}

		h = New(ctx, item.Val)
		for {
			var endChildren bool

			// NOTE: Receivers are passed by copy & need to be initialized; a pointer to nil won't
			// store the results.
			child := New(ctx, RootID)
			if endChildren, err = child.deserialize(ctx, l); endChildren || err != nil {
				// End of children.
				return
			}
			child.parent = h

			if child.id != RootID {
				// h.children = append(h.children, sub)
				h.children[child.id] = child
			}
		}
	}
}

// WalkChildrenOf traverses a `Node` pushing its values to a channel argument.
//
// Using a channel allowing for processing on a returned value as more are obtained.
func (h *Node) WalkChildrenOf(ctx context.Context, parentID string, hierChan chan TraverseChan) {
	if parentID == RootID {
		h.Walk(ctx, hierChan)
		return
	}

	if node, err := h.Locate(ctx, parentID); err == nil {
		node.Walk(ctx, hierChan)
		return
	}

	close(hierChan)
}

// Walk performs level-order traversal on a `Node`, pushing its values to its channel
// argument.
//
// This operation uses channels to minimize resource wastage.
// A `context.Context` is used to terminate the walk operation.
func (h *Node) Walk(ctx context.Context, hierChan chan TraverseChan) {
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
		queue := make([]*Node, initialQueueLen)
		queue = append(queue, h)

		for {
			qLen := len(queue)
			if qLen < 1 {
				break
			}

			// Iterate over the node's children.
			newPeers := true
			for qLen > 0 {
				// Pop from queue.
				var front *Node
				front, queue = queue[0], queue[1:]
				qLen--

				// Debug: this operation is noisy.
				// lLogger.Debugf("front: %+v, peers: %+v", front, newPeers)

				// Send node to caller via the channel.
				hierChan <- TraverseChan{node: front, newPeers: newPeers}
				newPeers = false

				// Add children to the queue.
				if front.children != nil {
					if front.children == nil {
						continue
					}

					for _, v := range front.children {
						queue = append(queue, v)
					}
				}
			}
		}
	}
}
