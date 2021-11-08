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
	// Node defines an n-array tree to hold hierarchies.
	//
	// NOTE: Adding synchronization to this structure is not feasible as of 2021-05-28T07:47:29+0300.
	Node struct {
		id string

		// superior holds the node at the upper level.
		superior *Node

		// subordinates holds nodes at a lower level.
		subordinates subMap
	}

	subMap map[string]*Node

	// TraverseChan defines a channel to communicate info between `HierarchyNode`
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

	// valueSplitter is the character used to split the `HierarchyNode` serialization output.
	valueSplitter = ','

	notSubordinateFmt = "(%s) %w (%s)"
)

// Errors codes encountered when handling a `HierarchyNode`.
var (
	ErrAlreadySub              = errors.New("is a subordinate of")
	ErrIDNotFound              = errors.New("not found")
	ErrNotSubordinate          = errors.New("is not a subordinate of")
	ErrNoSubordinates          = errors.New("lacks subordinates")
	ErrNoSuperiors             = errors.New("lacks superiors")
	ErrNoTerminalNodes         = errors.New("lacks terminal nodes; tree is cyclic")
	ErrEmptyDeserializationSrc = errors.New("empty HierarchyNode deserialization source")
)

var (
	lLogger = logrus.New()
)

func SetLogger(l *logrus.Logger) { lLogger = l }

// New initiates a `HierarchyNode`.
func New(_ context.Context, init string) *Node {
	return &Node{
		id:           init,
		subordinates: make(subMap),
	}
}

// AddSubordinate to a `HierarchyNode`.
func (h *Node) AddSubordinate(ctx context.Context, subordinate *Node) (err error) {
	// Search for existing immediate subordinate.
	if h.HasSubordinate(ctx, subordinate.id) > 0 {
		err = fmt.Errorf("(%s) %w (%s)", subordinate.id, ErrAlreadySub, h.id)
		return
	}

	subordinate.superior = h
	h.subordinates[subordinate.id] = subordinate

	return
}

// AddSubordinateTo to a `HierarchyNode`.
func (h *Node) AddSubordinateTo(ctx context.Context, superior string, subordinate *Node) (err error) {
	var superiorNode *Node
	if superiorNode, err = h.Locate(ctx, superior); err != nil {
		err = fmt.Errorf("superior (%s) %w", superior, err)
		return
	}

	return superiorNode.AddSubordinate(ctx, subordinate)
}

// PopSubordinate removes an immediate subordinate to the `HierarchyNode`.
func (h *Node) PopSubordinate(ctx context.Context, id string) (subordinate *Node, err error) {
	var subIndex int
	if subIndex = h.HasSubordinate(ctx, id); subIndex < 0 {
		err = fmt.Errorf("subordinate (%s) of (%s): %s", id, h.id, ErrIDNotFound)
		return
	}

	subordinate = h.subordinates[id]
	delete(h.subordinates, id)

	return
}

// HasSubordinate checks for the existence of an immediate subordinate.
func (h *Node) HasSubordinate(_ context.Context, subordinate string) (index int) {
	index = -1

	if _, ok := h.subordinates[subordinate]; ok {
		index = 1
	}

	return
}

// ListImmediateSubordinates lists the immediate subordinates for a `HierarchyNode`.
func (h *Node) ListImmediateSubordinates(ctx context.Context) (subs *types.StringSlice) {
	*subs = make(types.StringSlice, 0)

	for _, sub := range h.subordinates {
		*subs = append(*subs, sub.id)
	}
	return
}

// ListSubordinatesTo returns a list of subordinates as defined in `HierarchyNode` for some superior
// model.
//
// NOTE: This operation is expensive.
func (h *Node) ListSubordinatesTo(ctx context.Context, superior string) (subs *types.StringSlice, err error) {
	subs = new(types.StringSlice)
	hierChan := make(chan TraverseChan, traverseBufferSize)

	go h.WalkSubordinatesTo(ctx, superior, hierChan)

	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if resl.err != nil {
			err = resl.err
			return
		}

		*subs = append(*subs, resl.node.id)
	}

	lLogger.Debugf("`HierarchyNode` walk: %+v", *subs)

	lenSubs := len(*subs)
	if lenSubs > 0 {
		// Omit self from the list.
		*subs = (*subs)[1:]

		lLogger.Debugf("subordinates: %+v", *subs)
	}

	if len(*subs) < 1 {
		err = ErrNoSubordinates
	}

	return
}

// ListSubordinatesToWithLevel returns an array-of-arrays of subordinates as defined in
// `HierarchyNode` for some superior model.
//
// NOTE: This operation will bottleneck list generation operations.
func (h *Node) ListSubordinatesToWithLevel(ctx context.Context, superior string) (subs *[]types.StringSlice, err error) {
	subs = &[]types.StringSlice{}
	hierChan := make(chan TraverseChan, traverseBufferSize)

	go h.WalkSubordinatesTo(ctx, superior, hierChan)

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
				*subs = append(*subs, peers)
			}
			peers = make([]string, initialQueueLen)
		}
		peers = append(peers, resl.node.id)
	}
	if len(peers) > 0 {
		*subs = append(*subs, peers)
	}

	lLogger.Debugf("`HierarchyNode` walk: %+v", *subs)

	lenSubs := len(*subs)
	if lenSubs > 0 {
		// Omit self from the list.
		*subs = (*subs)[1:]

		lLogger.Debugf("subordinates: %+v", *subs)
	}

	if len(*subs) < 1 {
		err = ErrNoSubordinates
	}

	return
}

// ListTerminalNodes returns an array of terminal node data as defined in `HierarchyNode`.
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

		if resl.node.subordinates == nil || len(resl.node.subordinates) < 1 {
			*termItems = append(*termItems, resl.node.id)
		}
	}

	if len(*termItems) < 1 {
		err = ErrNoTerminalNodes
	}

	return
}

// GetSuperiorTo returns the superior `HierarchyNode` for some node identified by its `id`.
func (h *Node) GetSuperiorTo(ctx context.Context, id string) (superiorNode *Node, err error) {
	var node *Node
	if node, err = h.Locate(ctx, id); err != nil {
		err = fmt.Errorf("(%s) %w", id, err)
		return
	}

	// Update the variable to hold the superior node.
	superiorNode = node.superior

	return
}

// Locate searches for an `id` & returns it's `HierarchyNode`.
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

	if h.id == id {
		hierChan <- TraverseChan{node: h}
		return
	}

	if h.subordinates == nil || len(h.subordinates) < 1 {
		return
	}

	if node, ok := h.subordinates[id]; ok {
		hierChan <- TraverseChan{node: node}
		return
	}

	iWG := new(sync.WaitGroup)
	iWG.Add(len(h.subordinates))
	for _, v := range h.subordinates {
		go v.locate(ctx, id, hierChan, iWG)
	}
	iWG.Wait()

	return
}

// LocateSubordinateTo searches for the subordinate to some superior & returns it's `HierarchyNode`.
func (h *Node) LocateSubordinateTo(ctx context.Context, superior, subordinate string) (subordinateNode *Node, err error) {
	var superiorNode *Node

	if superior != RootID {
		if superiorNode, err = h.Locate(ctx, superior); err != nil {
			err = fmt.Errorf("superior (%s) %w", superior, err)
			return
		}
	} else {
		superiorNode = h
	}

	if subordinateNode, err = superiorNode.Locate(ctx, superior); err != nil {
		err = fmt.Errorf(notSubordinateFmt, subordinate, ErrNotSubordinate, superior)
	}

	return
}

// Serialize transforms a `HierarchyNode` into a string.
func (h *Node) Serialize(ctx context.Context) (output string, err error) {
	var buffer strings.Builder

	serChan := make(chan string)

	go func() {
		h.serialize(ctx, serChan)
		close(serChan)
	}()

	// Handle root `HierarchyNode`.
	fVal, fProceed := <-serChan
	if !fProceed {
		return
	}
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

	output, err = buffer.String(), ctx.Err()

	return
}

// serialize performs the serialization grunt work.
func (h *Node) serialize(ctx context.Context, serChan chan string) {
	if h == nil || h.id == RootID {
		return
	}

	serChan <- fmt.Sprint(h.id)
	for _, subordinate := range h.subordinates {
		select {
		case <-ctx.Done():
			return
		default:
			subordinate.serialize(ctx, serChan)
		}
	}
	serChan <- string(endMarker)
}

// Deserialize transforms a serialized `HierarchyNode` into a `HierarchyNode`.
//
// An invalid entry will result in a truncated `HierarchyNode`.
func (h *Node) Deserialize(ctx context.Context, input string) (err error) {
	if input == "" {
		err = ErrEmptyDeserializationSrc
		return
	}

	l := lexer.NewLexer(lLogger, input)
	go l.Lex(ctx)

	if _, err = h.deserialize(ctx, l); err != nil {
		err = fmt.Errorf("%w: %v", ErrInvalidHierarchySrc, err)
		return
	}

	diff := l.ValueCounter - l.EndCounter
	switch {
	case diff > 0:
		// Excessive values.
		err = fmt.Errorf("the serialized `HierarchyNode` has values in excess by: %d", diff)
	case diff < 0:
		// Excessive end markers.
		err = fmt.Errorf("the serialized `HierarchyNode` has end marker `%s` in excess by: %d", string(lexer.EndMarker), diff*-1)
	default:
		// Valid
	}
	if err != nil {
		return
	}

	subs, _ := h.ListSubordinatesToWithLevel(ctx, RootID)
	lLogger.Debugf("hierarchy: %+v", subs)

	err = ctx.Err()

	return
}

// deserialize performs the deserialization grunt work.
func (h *Node) deserialize(ctx context.Context, l *lexer.Lexer) (end bool, err error) {
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
		select {
		case <-ctx.Done():
			end = true
			return
		default:
			var endSubs bool

			// NOTE: Receivers are passed by copy & need to be initialized; a pointer to nil won't
			// store the results.
			sub := New(ctx, RootID)
			if endSubs, err = sub.deserialize(ctx, l); endSubs || err != nil {
				// End of children.
				return
			}
			sub.superior = h

			if sub.id != RootID {
				// h.subordinates = append(h.subordinates, sub)
				h.subordinates[sub.id] = sub
			}
		}
	}
}

// WalkSubordinatesTo traverses a `HierarchyNode` pushing its values to a channel argument.
//
// Using a channel allowing for processing on a returned value as more are obtained.
func (h *Node) WalkSubordinatesTo(ctx context.Context, superior string, hierChan chan TraverseChan) {
	if superior == RootID {
		h.Walk(ctx, hierChan)
		return
	}

	if node, err := h.Locate(ctx, superior); err == nil {
		node.Walk(ctx, hierChan)
		return
	}

	close(hierChan)
}

// Walk performs level-order traversal on a `HierarchyNode`, pushing its values to its channel
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
				if front.subordinates != nil {
					if front.subordinates == nil {
						continue
					}

					for _, v := range front.subordinates {
						queue = append(queue, v)
					}
				}
			}
		}
	}
}
