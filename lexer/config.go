// SPDX-License-Identifier: MIT
package lexer

import (
	"log/slog"
)

type (
	// Config defines configuration options for the Lexer's operations.
	Config struct {
		Logger    *slog.Logger
		EndMarker rune
		Debug     bool
		Splitter  rune
	}
)

const (
	// DefaultEndMarker a `rune` indicating the end of a node's children.
	DefaultEndMarker = ')'

	// DefaultSplitter is the character used to split the `HierarchyNode` serialization output.
	DefaultSplitter = ','

	emptyRune rune = 0
)

// DefaultConfig configures the lexer's Opts.
func DefaultConfig() *Config {
	return &Config{
		EndMarker: DefaultEndMarker,
		Splitter:  DefaultSplitter,
		Logger:    slog.Default(),
	}
}

// Validate populates missing Opts entries with defaults.
func (c *Config) Validate() {
	if c.EndMarker == emptyRune {
		c.EndMarker = DefaultEndMarker
	}

	if c.Splitter == emptyRune {
		c.Splitter = DefaultSplitter
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}
