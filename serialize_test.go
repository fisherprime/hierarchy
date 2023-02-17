// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"testing"

	"gitlab.com/fisherprime/hierarchy/lexer"
)

func TestHierarchy_Serialize(t *testing.T) {
	type args struct {
		ctx  context.Context
		opts lexer.Opts
	}

	tests := []struct {
		name       string
		fields     *Hierarchy
		args       args
		wantOutput string
		wantErr    bool
	}{
		{
			name: "valid",
			args: args{context.Background(), *lexer.NewOpts()},
			fields: &Hierarchy{
				value: "2",
				children: childMap{"3": &Hierarchy{
					value: "3", children: childMap{},
				}},
			},
			wantOutput: "2,3))",
			// wantErr: true,
		},
		{
			name: "valid 2",
			args: args{context.Background(), *lexer.NewOpts()},
			fields: &Hierarchy{
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
			wantOutput: "2,3),4))",
			// wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hierarchy{
				value:    tt.fields.value,
				parent:   tt.fields.parent,
				children: tt.fields.children,
			}
			gotOutput, err := h.Serialize(tt.args.ctx, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Hierarchy.Serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOutput != tt.wantOutput {
				t.Errorf("Hierarchy.Serialize() = %v, want %v", gotOutput, tt.wantOutput)
			}
		})
	}
}
