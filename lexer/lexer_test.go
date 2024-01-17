// SPDX-License-Identifier: MIT
package lexer

import (
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func BenchmarkLexer_Lex(b *testing.B) {
	src := "2,3,4))"

	logger := logrus.New()
	ctx := context.Background()

	b.ReportAllocs()
	b.SetBytes(int64(len(src)))
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		b.StopTimer()

		l := New(WithLogger(logger), WithSource(strings.NewReader(src)))

		b.StartTimer()

		go l.Lex(ctx)

		for {
			// ItemEOF check is unnecessary.
			if _, proceed := l.Item(); !proceed {
				break
			}
		}
	}
}
