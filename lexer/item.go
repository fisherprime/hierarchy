// SPDX-License-Identifier: MIT
package lexer

type (
	// ItemID int holding an identifier for the Item tokens
	ItemID int

	// Item type holding token, value & item type of scanned rune
	Item struct {
		Err error
		Val string // The value of this Item
		ID  ItemID // The type of this Item
		Pos int    // The starting position, (in bytes) of this Item
	}
)

// iota is used to define an incrementing number sequence for const
// declarations
const (
	_             = iota // Consume 0 to start actual numbering at 1.
	ItemError            // Notify occurrence of an `error`.
	ItemSplitter         // References `serSplitter`.
	ItemEOF              // End of the file
	ItemValue            // `HierarchyNode` data.
	ItemEndMarker        // ')'.
)
