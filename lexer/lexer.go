// SPDX-License-Identifier: MIT
package lexer

// REF: https://github.com/sh4t/sql-parser
// REF: https://gitlab.com/fisherprime/go-ddbms/-/blob/master/internal/v1/lexer.go

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

type (
	// ItemID int holding an identifier for the Item tokens
	ItemID int

	// NextOperation type for the next function to be executed
	NextOperation func(context.Context) NextOperation

	// ValidationFunction type for functions that validate rune identities
	ValidationFunction func(rune) bool

	// Item type holding token, value & item type of scanned rune
	Item struct {
		ID  ItemID // The type of this Item
		Pos int    // The starting position, (in bytes) of this Item
		Val string // The value of this Item
		Err error
	}

	// Lexer defines a type to capture identifiers from a string.
	Lexer struct {
		Source      io.RuneReader // input source
		SourceIndex int           // start position of this item's rune

		Buffer      []rune // slice of runes being lexed
		BufferIndex int    // current buffer position

		Item chan Item // channel for scanned Items

		*logrus.Logger

		ValueCounter int
		EndCounter   int
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

const (
	queryLimit = 512

	// EndMarker a `rune` indicating the end of a node's children.
	EndMarker = ')'

	// ValueSplitter is the character used to split the `HierarchyNode` serialization output.
	ValueSplitter = ','
)

// New creates a new scanner for the input string
func New(logger *logrus.Logger, source string) *Lexer {
	return &Lexer{
		Buffer: make([]rune, 0, 10),
		Item:   make(chan Item),
		Logger: logger,
		Source: strings.NewReader(source),
	}
}

// Lex lexes the input by executing state functions.
func (l *Lexer) Lex(ctx context.Context) {
	for stateFunction := l.LexWhitespace; stateFunction != nil; {
		stateFunction = stateFunction(ctx)
	}

	// Close channel
	close(l.Item)
}

// LexWhitespace search for whitespace.
func (l *Lexer) LexWhitespace(ctx context.Context) NextOperation {
	select {
	case <-ctx.Done():
		return nil
	default:
		// Do nothing with white spaces.
		if err := l.AcceptWhile(isWhitespace); err != nil {
			return l.LogError(err)
		}

		next, err := l.Peek()
		l.Logger.Debug("next token: ", string(next), "\ncurrent token: ",
			string(l.Buffer[l.BufferIndex]), "\nerr: ", err)
		if err != nil {
			return l.LogError(err)
		}

		switch {
		case next == EndMarker:
			_, _ = l.Next()
			l.EndCounter++
			l.Emit(ItemEndMarker)

			return l.LexWhitespace(ctx)
		case next == ValueSplitter:
			_, _ = l.Next()
			l.Emit(ItemSplitter)

			return l.LexWhitespace(ctx)
		case isValue(next):
			return l.LexValue(ctx)
		default:
			nextRunes, err := l.PeekNext(queryLimit)
			if err != nil {
				return l.LogError(err)
			}

			l.LogError(fmt.Errorf("unknown operation for: %v", string(nextRunes)))

			return nil
		}
	}
}

// LexValue search for identifiers / SQL keywords
func (l *Lexer) LexValue(ctx context.Context) NextOperation {
	select {
	case <-ctx.Done():
		return nil
	default:
		next, err := l.Next()
		if err != nil {
			if err == io.EOF {
				l.Emit(ItemEOF)
				return nil
			}
			l.LogError(err)

			return nil
		}

		if isValue(next) {
			if err := l.AcceptWhile(isValue); err != nil {
				l.LogError(err)
				return nil
			}

			l.ValueCounter++
			l.Emit(ItemValue)
		}

		return l.LexWhitespace(ctx)
	}
}

// Next return the Next rune in the input.
func (l *Lexer) Next() (r rune, err error) {
	if l.BufferIndex < len(l.Buffer) {
		l.BufferIndex++
		r = l.Buffer[l.BufferIndex-1]

		return
	}

	// Check if end of source input
	r, _, err = l.Source.ReadRune()
	if err != nil {
		return
	}

	// Append rune to the buffer & update the buffer index
	l.Buffer = append(l.Buffer, r)
	l.BufferIndex++

	return
}

// Peek return next rune, don't update index.
func (l *Lexer) Peek() (r rune, err error) {
	if l.BufferIndex < len(l.Buffer) {
		r = l.Buffer[l.BufferIndex]
		return
	}

	if _, err = l.PeekNext(1); err == nil {
		r = l.Buffer[l.BufferIndex]
	}

	return
}

// PeekNext return next N runes, without updating the index.
//
// This operation will return a shorter array if the the end of the source input is reached.
func (l *Lexer) PeekNext(length int) (rList []rune, err error) {
	if length < 1 {
		err = fmt.Errorf("invalid peek length: %d", length)
		return
	}

	if l.BufferIndex+length < len(l.Buffer) {
		rList = l.Buffer[l.BufferIndex:(l.BufferIndex + length)]
		return
	}

	var seen = 0
	for ; seen < length; seen++ {
		var r rune

		r, _, err = l.Source.ReadRune()
		if err != nil {
			if err != io.EOF {
				return
			}
			err = nil

			break
		}

		l.Buffer = append(l.Buffer, r)
	}
	rList = l.Buffer[l.BufferIndex:(l.BufferIndex + seen)]

	return
}

// Backup step back one rune.
func (l *Lexer) Backup() error { return l.BackupFor(1) }

// BackupFor step back for N runes.
func (l *Lexer) BackupFor(length int) (err error) {
	if l.BufferIndex < length {
		err = fmt.Errorf("attempt to back up (%d) spaces on buffer index: %d", length, l.BufferIndex)
		return
	}
	l.BufferIndex -= length

	return
}

// Ignore skip scanner input before the current buffer index.
func (l *Lexer) Ignore() {
	itemBytes := 0

	for index := 0; index < l.BufferIndex; index++ {
		itemBytes += utf8.RuneLen(l.Buffer[index])
	}

	l.SourceIndex += itemBytes
	l.Buffer = l.Buffer[l.BufferIndex:]

	l.BufferIndex = 0
}

// AcceptWhile consumes runes while condition is true.
func (l *Lexer) AcceptWhile(fn ValidationFunction) (err error) {
	r, err := l.Next()
	if err != nil {
		return
	}

	for fn(r) {
		if r, err = l.Next(); err != nil {
			return
		}
	}

	// Backup if validation fails.
	err = l.Backup()

	return
}

// Emit send an Item specification to the LexType.
func (l *Lexer) Emit(t ItemID) {
	l.Logger.Debug("Lexer buffer content: ", string(l.Buffer[:l.BufferIndex]))
	l.Item <- Item{
		ID:  t,
		Pos: l.SourceIndex,
		Val: string(l.Buffer[:l.BufferIndex]),
	}
	l.Ignore()
}

// LogError set the item to the error token that'll terminate the scan process (nextItem).
func (l *Lexer) LogError(err error) NextOperation {
	l.Item <- Item{
		ID:  ItemError,
		Pos: l.SourceIndex,
		Err: err,
	}
	return nil
}

// NextItem return the next Item from the input.
func (l *Lexer) NextItem() Item { return <-l.Item }

// isSpace return true for whitespace, newline & carrier return.
func isWhitespace(r rune) bool { return isSpace(r) || r == '\r' || r == '\n' }

// isSpace return true for space or tab.
func isSpace(r rune) bool { return r == ' ' || r == '\t' }

// isAlpha return true for an alphabetic sequence.
func isAlpha(r rune) bool { return r == '_' || unicode.IsLetter(r) || r == '-' }

// isNumeric return true for a real number.
func isNumeric(r rune) bool { return unicode.IsDigit(r) }

// isNumeric return true for an alphanumeric sequence.
func isValue(r rune) bool { return isAlpha(r) || isNumeric(r) }
