// SPDX-License-Identifier: MIT
package lexer

import (
	"context"
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
		l := New(Opts{Logger: logger}, src)
		b.StartTimer()

		go l.Lex(ctx)

		for {
			if item, proceed := <-l.C; !proceed || item.ID == ItemEOF {
				break
			}
		}
	}
}
