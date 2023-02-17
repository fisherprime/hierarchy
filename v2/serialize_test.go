// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"testing"

	"gitlab.com/fisherprime/hierarchy/lexer/v2"
)

func TestHierarchy_Serialize(t *testing.T) {
	type args struct {
		ctx  context.Context
		opts lexer.Opts
	}

	tests := []struct {
		name       string
		fields     *Hierarchy[string]
		args       args
		wantOutput string
		wantErr    bool
	}{
		{
			name: "valid",
			args: args{context.Background(), *lexer.NewOpts()},
			fields: &Hierarchy[string]{
				value: "2",
				children: children[string]{"3": &Hierarchy[string]{
					value: "3", children: children[string]{},
				}},
			},
			wantOutput: "2,3))",
			// wantErr: true,
		},
		{
			name: "valid 2",
			args: args{context.Background(), *lexer.NewOpts()},
			fields: &Hierarchy[string]{
				value: "2",
				children: children[string]{
					"3": &Hierarchy[string]{
						value: "3", children: children[string]{},
					},
					"4": &Hierarchy[string]{
						value: "4", children: children[string]{},
					},
				},
			},
			wantOutput: "2,3),4))",
			// wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hierarchy[string]{
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
