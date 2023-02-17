// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"gitlab.com/fisherprime/hierarchy/lexer"
)

func TestDeserialize(t *testing.T) {
	type args struct {
		ctx   context.Context
		input string
		opts  lexer.Opts
	}

	tests := []struct {
		name    string
		args    args
		wantH   *Hierarchy
		wantErr bool
	}{
		{
			name: "valid",
			args: args{
				context.Background(),
				"2,3))",
				lexer.Opts{Logger: fLogger},
			},
			wantH: &Hierarchy{
				value: "2",
				children: childMap{"3": &Hierarchy{
					value: "3", children: childMap{},
				}},
			},
			// wantErr: true,
		},
		{
			name: "invalid (missing end delimiter)",
			args: args{
				context.Background(),
				"2,3,4))",
				lexer.Opts{Logger: fLogger},
			},
			wantH: &Hierarchy{
				value: "2",
				children: childMap{
					"3": &Hierarchy{
						value: "3", children: childMap{},
					},
					"4": &Hierarchy{
						value: "4", children: childMap{},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotH, err := Deserialize(tt.args.ctx, tt.args.opts, tt.args.input)
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

			sort.Strings(gotNodes)
			sort.Strings(wantNodes)
			t.Logf("gotNodes: %+v, wantNodes: %+v\n", gotNodes, wantNodes)

			if !reflect.DeepEqual(gotNodes, wantNodes) {
				t.Errorf("Deserialize() = %v, want %v", gotH, tt.wantH)
			}
		})
	}
}
