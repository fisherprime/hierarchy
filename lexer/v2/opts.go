// SPDX-License-Identifier: MIT
package lexer

import (
	"github.com/sirupsen/logrus"
)

type (
	// Opts defines options for the Lexer's operations.
	Opts struct {
		Debug     bool
		EndMarker rune
		Splitter  rune
		Logger    logrus.FieldLogger
	}
)

const (
	// defEndMarker a `rune` indicating the end of a node's children.
	defEndMarker = ')'

	// dDefSplitter is the character used to split the `HierarchyNode` serialization output.
	dDefSplitter = ','

	emptyRune rune = 0
)

// NewOpts configures the lexer's Opts.
func NewOpts() *Opts {
	return &Opts{
		EndMarker: defEndMarker,
		Splitter:  dDefSplitter,
		Logger:    logrus.New(),
	}
}

// Validate populates missing Opts entries with defaults.
func (o *Opts) Validate() {
	if o.EndMarker == emptyRune {
		o.EndMarker = defEndMarker
	}
	if o.Splitter == emptyRune {
		o.Splitter = dDefSplitter
	}
	if o.Logger == nil {
		o.Logger = logrus.New()
	}
}
