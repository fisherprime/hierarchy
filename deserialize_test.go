// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gitlab.com/fisherprime/hierarchy/v3/lexer"
)

func TestDeserialize(t *testing.T) {
	type args struct {
		ctx     context.Context
		lexCfg  []lexer.Option
		hierCfg []Option[int]
	}

	logger := slog.Default()
	hierCfg := []Option[int]{WithConfig[int](&Config{Logger: logger})}

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
				hierCfg,
			},
			wantH: &Hierarchy[int]{
				value: 2,
				children: children[int]{3: &Hierarchy[int]{
					value: 3, children: children[int]{},
				}},
				cfg: DefConfig(),
			},
			// wantErr: true,
		},
		{
			name: "valid (excessive whitespace)",
			args: args{
				context.Background(),
				[]lexer.Option{lexer.WithLogger(logger), lexer.WithSource(strings.NewReader(" 2 ,     3 )    )         "))},
				hierCfg,
			},
			wantH: &Hierarchy[int]{
				value: 2,
				children: children[int]{3: &Hierarchy[int]{
					value: 3, children: children[int]{},
				}},
				cfg: DefConfig(),
			},
			// wantErr: true,
		},
		{
			name: "invalid (missing end delimiter)",
			args: args{
				context.Background(),
				[]lexer.Option{lexer.WithLogger(logger), lexer.WithSource(strings.NewReader("2,3,4))"))},
				hierCfg,
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
				cfg: DefConfig(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.args.lexCfg...)
			gotH, err := Deserialize(tt.args.ctx, l, tt.args.hierCfg...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Deserialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Comparison of the Hierarchy given the childMap member will always fail.
			children, err := gotH.AllChildren(tt.args.ctx)
			if err != nil {
				t.Errorf("Deserialize()->Hierarchy.AllChildren() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotNodes := children.Values(tt.args.ctx)
			gotNodes = append(gotNodes, gotH.value)

			children, err = tt.wantH.AllChildren(tt.args.ctx)
			if err != nil {
				t.Errorf("Deserialize()->Hierarchy.AllChildren() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			wantNodes := children.Values(tt.args.ctx)
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
