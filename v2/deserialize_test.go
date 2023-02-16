// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"gitlab.com/fisherprime/hierarchy/lexer/v2"
)

func TestDeserialize(t *testing.T) {
	type args struct {
		ctx context.Context
		cfg []lexer.Option
	}

	logger := logrus.New()

	tests := []struct {
		name    string
		args    args
		wantH   *Hierarchy[int]
		wantErr bool
	}{
		{
			name: "valid",
			args: args{
				context.Background(),
				[]lexer.Option{lexer.WithLogger(logger), lexer.WithSource(strings.NewReader("2,3))"))},
			},
			wantH: &Hierarchy[int]{
				value: 2,
				children: children[int]{3: &Hierarchy[int]{
					value: 3, children: children[int]{},
				}},
				cfg: DefOpts(),
			},
			// wantErr: true,
		},
		{
			name: "valid (excessive whitespace)",
			args: args{
				context.Background(),
				[]lexer.Option{lexer.WithLogger(logger), lexer.WithSource(strings.NewReader(" 2 ,     3 )    )         "))},
			},
			wantH: &Hierarchy[int]{
				value: 2,
				children: children[int]{3: &Hierarchy[int]{
					value: 3, children: children[int]{},
				}},
				cfg: DefOpts(),
			},
			// wantErr: true,
		},
		{
			name: "invalid (missing end delimiter)",
			args: args{
				context.Background(),
				[]lexer.Option{lexer.WithLogger(logger), lexer.WithSource(strings.NewReader("2,3,4))"))},
			},
			wantH: &Hierarchy[int]{
				value: 2,
				children: children[int]{
					3: &Hierarchy[int]{
						value: 3, children: children[int]{},
					},
					4: &Hierarchy[int]{
						value: 4, children: children[int]{},
					},
				},
				cfg: DefOpts(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotH, err := Deserialize[int](tt.args.ctx, tt.args.cfg...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Deserialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Comparison of the Hierarchy given the childMap member will always fail.
			gotNodes, err := gotH.AllChildren(tt.args.ctx)
			if err != nil {
				t.Errorf("Deserialize()->Hierarchy.AllChildren() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotNodes = append(gotNodes, gotH.value)

			wantNodes, err := tt.wantH.AllChildren(tt.args.ctx)
			if err != nil {
				t.Errorf("Deserialize()->Hierarchy.AllChildren() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			wantNodes = append(wantNodes, tt.wantH.value)

			sort.Ints(gotNodes)
			sort.Ints(wantNodes)
			t.Logf("gotNodes: %+v, wantNodes: %+v\n", gotNodes, wantNodes)

			if !reflect.DeepEqual(gotNodes, wantNodes) {
				t.Errorf("Deserialize() = %v, want %v", gotH, tt.wantH)
			}
		})
	}
}
