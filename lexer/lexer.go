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
)

type (
	// NextOperation type for the next function to be executed
	NextOperation func(context.Context) NextOperation

	// ValidationFunction type for functions that validate rune identities
	ValidationFunction func(rune) bool

	// Lexer defines a type to capture identifiers from a string.
	Lexer struct {
		opts Opts

		C chan Item // channel for scanned Items

		source      io.RuneReader // input source
		sourceIndex int           // start position of this item's rune

		buffer      []rune // slice of runes being lexed
		bufferIndex int    // current buffer position

		valueCounter int
		endCounter   int
	}
)

const (
	peekLimit = 512

	defBufferSize = 10
)

// New creates a new scanner for the input string
func New(opts Opts, source string) *Lexer {
	(&opts).Validate()

	return &Lexer{
		opts: opts,

		C: make(chan Item),

		source: strings.NewReader(source),
		buffer: make([]rune, 0, defBufferSize),
	}
}

// EndMarker obtains the configured end marker.
func (l *Lexer) EndMarker() rune { return l.opts.EndMarker }

// Splitter obtains the configured value splitter.
func (l *Lexer) Splitter() rune { return l.opts.Splitter }

// ValueCounter obtains valueCounter.
func (l *Lexer) ValueCounter() int { return l.valueCounter }

// EndCounter obtains endCounter.
func (l *Lexer) EndCounter() int { return l.endCounter }

// Lex lexes the input by executing state functions.
func (l *Lexer) Lex(ctx context.Context) {
	for stateFunction := l.LexWhitespace; stateFunction != nil; {
		stateFunction = stateFunction(ctx)
	}

	// Close channel
	close(l.C)
}

// LexWhitespace search for whitespace.
func (l *Lexer) LexWhitespace(ctx context.Context) NextOperation {
	select {
	case <-ctx.Done():
		return nil
	default:
		// Do nothing with white spaces.
		if err := l.AcceptWhile(isWhitespace); err != nil {
			return l.EmitError(err)
		}

		next, err := l.Peek()
		l.opts.Logger.Debug("next token: ", string(next), "\ncurrent token: ",
			string(l.buffer[l.bufferIndex]), "\nerr: ", err)
		if err != nil {
			return l.EmitError(err)
		}

		switch {
		case next == l.opts.EndMarker:
			_, _ = l.Next()
			l.endCounter++
			l.Emit(ItemEndMarker)

			return l.LexWhitespace(ctx)
		case next == l.opts.Splitter:
			_, _ = l.Next()
			l.Emit(ItemSplitter)

			return l.LexWhitespace(ctx)
		case isValue(next):
			return l.LexValue(ctx)
		default:
			nextRunes, err := l.PeekNext(peekLimit)
			if err != nil {
				return l.EmitError(err)
			}

			l.EmitError(fmt.Errorf("unknown operation for: %v", string(nextRunes)))

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
			l.EmitError(err)

			return nil
		}

		if isValue(next) {
			if err := l.AcceptWhile(isValue); err != nil {
				l.EmitError(err)
				return nil
			}

			l.valueCounter++
			l.Emit(ItemValue)
		}

		return l.LexWhitespace(ctx)
	}
}

// Next return the Next rune in the input.
func (l *Lexer) Next() (r rune, err error) {
	if l.bufferIndex < len(l.buffer) {
		l.bufferIndex++
		r = l.buffer[l.bufferIndex-1]

		return
	}

	// Check if end of source input
	if r, _, err = l.source.ReadRune(); err != nil {
		return
	}

	// Append rune to the buffer & update the buffer index
	l.buffer = append(l.buffer, r)
	l.bufferIndex++

	return
}

// Peek return next rune, don't update index.
func (l *Lexer) Peek() (r rune, err error) {
	if l.bufferIndex < len(l.buffer) {
		r = l.buffer[l.bufferIndex]
		return
	}

	if _, err = l.PeekNext(1); err == nil {
		r = l.buffer[l.bufferIndex]
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

	if l.bufferIndex+length < len(l.buffer) {
		rList = l.buffer[l.bufferIndex:(l.bufferIndex + length)]
		return
	}

	var seen = 0
	for ; seen < length; seen++ {
		var r rune

		if r, _, err = l.source.ReadRune(); err != nil {
			if err != io.EOF {
				return
			}
			err = nil

			break
		}

		l.buffer = append(l.buffer, r)
	}
	rList = l.buffer[l.bufferIndex:(l.bufferIndex + seen)]

	return
}

// Backup step back one rune.
func (l *Lexer) Backup() error { return l.BackupFor(1) }

// BackupFor step back for N runes.
func (l *Lexer) BackupFor(length int) (err error) {
	if l.bufferIndex < length {
		err = fmt.Errorf("attempt to back up (%d) spaces on buffer index: %d", length, l.bufferIndex)
		return
	}
	l.bufferIndex -= length

	return
}

// Ignore skip scanner input before the current buffer index.
func (l *Lexer) Ignore() {
	itemBytes := 0

	for index := 0; index < l.bufferIndex; index++ {
		itemBytes += utf8.RuneLen(l.buffer[index])
	}

	l.sourceIndex += itemBytes
	l.buffer = l.buffer[l.bufferIndex:]

	l.bufferIndex = 0
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
	l.opts.Logger.Debug("Lexer buffer content: ", string(l.buffer[:l.bufferIndex]))
	l.C <- Item{
		ID:  t,
		Pos: l.sourceIndex,
		Val: string(l.buffer[:l.bufferIndex]),
	}
	l.Ignore()
}

// EmitError sends an error over the `Lexer`'s channel.
//
// This terminates the scan process (nextItem).
func (l *Lexer) EmitError(err error) NextOperation {
	l.C <- Item{
		ID:  ItemError,
		Pos: l.sourceIndex,
		Err: err,
	}
	return nil
}

// NextItem return the next Item from the input.
func (l *Lexer) NextItem() Item { return <-l.C }

// isSpace return true for whitespace, newline & carrier return.
func isWhitespace(r rune) bool { return r == ' ' || r == '\t' || r == '\r' || r == '\n' }

// isAlpha return true for an alphabetic sequence.
func isAlpha(r rune) bool { return r == '_' || unicode.IsLetter(r) || r == '-' }

// isNumeric return true for a real number.
func isNumeric(r rune) bool { return unicode.IsDigit(r) }

// isNumeric return true for an alphanumeric sequence.
func isValue(r rune) bool { return isAlpha(r) || isNumeric(r) }
