// SPDX-License-Identifier: MIT
package lexer

// REF: https://github.com/sh4t/sql-parser
// REF: https://gitlab.com/fisherprime/go-ddbms/-/blob/master/internal/v1/lexer.go
// REF: https://dave.cheney.net/high-performance-json.html
// REF: []rune to []byte conversion. https://stackoverflow.com/a/29255836

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
	// NextOperation type for the next function to be executed
	NextOperation func(context.Context) NextOperation

	// ValidationFunction type for functions that validate rune identities
	ValidationFunction func(rune) bool

	// Lexer defines a type to capture identifiers from a string.
	Lexer struct {
		opts      Opts
		Debug     bool
		endMarker rune
		splitter  rune
		logger    logrus.FieldLogger

		// c is a channel for communicating lexed Items.
		c chan Item

		// source is the input source.
		source io.RuneReader

		// buffer is a slice of runes being lexed.
		buffer []rune
		//  bufferIndex is the current buffer position.
		//
		// When this value exceeds the length of buffer, the buffer is populated from the source.
		bufferIndex int

		valueCounter int
		endCounter   int
	}

	// Option defines the Lexer functional option type
	Option func(*Lexer)
)

const (
	sourceLimit   = 512
	defBufferSize = 10
)

// Lexing errors.
var (
	ErrInvalidPeekLength   = fmt.Errorf("invalid peek length")
	ErrInvalidBackupAmount = fmt.Errorf("invalid backup amount")
	ErrUnknownTokens       = fmt.Errorf("unknown tokens")
)

// Improves on performance compared to ORs.
//
// Reduces function cost improving probalility of inlining.
var (
	whitespace = [256]bool{
		' ':  true,
		'\t': true,
		'\r': true,
		'\n': true,
	}

	alphaSymbols = [256]bool{
		'_': true,
		'-': true}
)

// New creates a new scanner for the input string
func New(opts ...Option) *Lexer {
	l := &Lexer{
		endMarker: defEndMarker,
		splitter:  dDefSplitter,
		logger:    logrus.New(),

		c: make(chan Item, defBufferSize),

		buffer: make([]rune, 0, defBufferSize),
		source: strings.NewReader(""),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// WithDebug configures the debug option.
func WithDebug(debug bool) Option { return func(l *Lexer) { l.Debug = debug } }

// WithEndMarker configures the endMarker option.
func WithEndMarker(r rune) Option { return func(l *Lexer) { l.endMarker = r } }

// WithSplitter configures the splitter option.
func WithSplitter(r rune) Option { return func(l *Lexer) { l.splitter = r } }

// WithLogger configures the logger option.
func WithLogger(logger logrus.FieldLogger) Option { return func(l *Lexer) { l.logger = logger } }

// WithSource configures the source option.
func WithSource(source io.RuneReader) Option { return func(l *Lexer) { l.source = source } }

// EndMarker obtains the configured end marker.
func (l *Lexer) EndMarker() rune { return l.endMarker }

// Splitter obtains the configured value splitter.
func (l *Lexer) Splitter() rune { return l.splitter }

// ValueCounter obtains valueCounter.
func (l *Lexer) ValueCounter() int { return l.valueCounter }

// EndCounter obtains endCounter.
func (l *Lexer) EndCounter() int { return l.endCounter }

// Logger obtains the logger.
func (l *Lexer) Logger() logrus.FieldLogger { return l.logger }

// Lex lexes the input by executing state functions.
func (l *Lexer) Lex(ctx context.Context) {
	select {
	case <-ctx.Done():
		l.EmitError(ctx.Err())
	default:
		for stateFunction := l.LexWhitespace; stateFunction != nil; {
			stateFunction = stateFunction(ctx)
		}
	}

	// Close channel
	close(l.c)
}

// LexWhitespace search for whitespace.
func (l *Lexer) LexWhitespace(ctx context.Context) NextOperation {
	if err := l.AcceptWhile(isWhitespace); err != nil {
		l.EmitError(err)
		return nil
	}
	// Ignore white spaces, discard instead of emit.
	l.Discard()

	next := l.Next()
	switch {
	case next == emptyRune:
		return nil
	case next == l.endMarker:
		l.endCounter++
		l.Emit(ItemEndMarker)

		return l.LexWhitespace(ctx)
	case next == l.splitter:
		l.Emit(ItemSplitter)

		return l.LexWhitespace(ctx)
	case isValue(next):
		return l.LexValue(ctx)
	default:
		if err := l.Backup(); err != nil {
			l.EmitError(err)
			return nil
		}

		nextRunes, err := l.PeekN(sourceLimit)
		if err != nil {
			l.EmitError(err)
			return nil
		}

		l.EmitError(fmt.Errorf("%w: %s", ErrUnknownTokens, string(nextRunes)))

		return nil
	}
}

// LexValue search for identifiers / SQL keywords
func (l *Lexer) LexValue(ctx context.Context) NextOperation {
	if err := l.AcceptWhile(isValue); err != nil {
		l.EmitError(err)
		return nil
	}

	l.valueCounter++
	l.Emit(ItemValue)

	return l.LexWhitespace(ctx)
}

// Next return the Next rune in the input.
func (l *Lexer) Next() (r rune) {
	if l.bufferIndex >= len(l.buffer) {
		// Request data from the source.
		if l.Source(0) < 1 {
			r = emptyRune
			return
		}
	}

	r = l.buffer[l.bufferIndex]
	l.bufferIndex++

	return

}

// Peek return the next rune, without updating the index.
func (l *Lexer) Peek() (r rune, err error) {
	list, err := l.PeekN(1)
	if err != nil {
		return
	}
	r = list[0]

	return
}

// PeekN return the next N runes, without updating the index.
//
// This operation will return a shorter slice if the the end of the source is reached.
func (l *Lexer) PeekN(n int) (list []rune, err error) {
	if n < 1 {
		err = fmt.Errorf("%w: %d", ErrInvalidPeekLength, n)
		return
	}

	limit := l.bufferIndex + n
	if limit >= len(l.buffer) {
		// Request data from the source.
		if n = l.Source(0); n < 1 {
			err = io.EOF
			return
		}

		limit = l.bufferIndex + n
	}

	list = l.buffer[l.bufferIndex:limit]

	return
}

// Backup step back one rune.
func (l *Lexer) Backup() error { return l.BackupN(1) }

// BackupN step back N runes.
func (l *Lexer) BackupN(n int) (err error) {
	if l.bufferIndex < n {
		err = fmt.Errorf("%w: amount %d index: %d", ErrInvalidBackupAmount, n, l.bufferIndex)
		return
	}
	l.bufferIndex -= n

	return
}

// Discard the buffer content before the current buffer index.
func (l *Lexer) Discard() {
	l.buffer = l.buffer[l.bufferIndex:]
	l.bufferIndex = 0
}

// Source runes from the source reader.
func (l *Lexer) Source(amount int) (sourced int) {
	if amount < defBufferSize {
		amount = defBufferSize
	}

	buffer := make([]rune, amount)
	for ; sourced < amount; sourced++ {
		// NOTE: Function cost reduced by swapping the error check's condition.
		if r, _, err := l.source.ReadRune(); err == nil {
			buffer[sourced] = r
			continue
		}

		// Error can only be io.EOF
		break
	}

	l.buffer = append(l.buffer, buffer[:sourced]...)

	return
}

// AcceptWhile consumes runes while condition is true.
func (l *Lexer) AcceptWhile(fn ValidationFunction) (err error) {
	for {
		r := l.Next()
		if r == emptyRune {
			// End of input.
			return io.EOF
		}

		// End of current token type.
		if !fn(r) {
			// TODO: Validate the necessity of propagating this error.
			//
			// An error at this point should never occur; unless the Lexer is modified externally.
			return l.Backup()
		}
	}
}

// Emit sends an Item over the communication channel.
func (l *Lexer) Emit(t ItemID) {
	runes := l.buffer[:l.bufferIndex]

	bufSize := 0
	for _, r := range runes {
		bufSize += utf8.RuneLen(r)
	}
	buf := make([]byte, bufSize)

	index := 0
	for _, r := range runes {
		index += utf8.EncodeRune(buf[index:], r)
	}

	if l.Debug {
		// Debug operation makes this operation un-inlinable.
		l.logger.Debug("lexer Emit: ", string(buf))
	}

	l.c <- Item{
		ID:  t,
		Val: buf,
	}
	l.Discard()
}

// EmitEOF sends an ItemEOF Item over the communication channel.
func (l *Lexer) EmitEOF() {
	l.c <- Item{ID: ItemEOF}
}

// EmitError sends an error over the `Lexer`'s channel.
//
// This terminates the scan process with an error or an ItemEOF for io.EOF.
func (l *Lexer) EmitError(err error) {
	if err == io.EOF {
		l.EmitEOF()
		return
	}

	l.c <- Item{
		ID:  ItemError,
		Err: err,
	}
}

// Item return a lexed Item from the input.
func (l *Lexer) Item() (i Item, ok bool) {
	i, ok = <-l.c
	return
}

// isSpace return true for whitespace, newline & carrier return.
func isWhitespace(r rune) bool { return whitespace[r] }

// isAlpha return true for an alphabetic sequence.
func isAlpha(r rune) bool { return alphaSymbols[r] || unicode.IsLetter(r) }

// isNumeric return true for a real number.
func isNumeric(r rune) bool { return unicode.IsDigit(r) }

// isNumeric return true for an alphanumeric sequence.
func isValue(r rune) bool { return isAlpha(r) || isNumeric(r) }
