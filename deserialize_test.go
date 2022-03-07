// SPDX-License-Identifier: NONE
package hierarchy

import (
	"context"
	"reflect"
	"testing"

	"gitlab.com/fisherprime/hierarchy/lexer"
)

func TestDeserialize(t *testing.T) {
	type args struct {
		ctx   context.Context
		input string
	}
	tests := []struct {
		name    string
		args    args
		wantH   *Node
		wantErr bool
	}{{
		name: "valid",
		args: args{context.Background(), "2,3))"},
		wantH: &Node{
			value:     "2",
			rootValue: "0",
			children: childMap{"3": &Node{
				value:     "3",
				rootValue: "0",
			}},
		},
		// wantErr: true,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotH, err := Deserialize(tt.args.ctx, tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Deserialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotH, tt.wantH) {
				t.Errorf("Deserialize() = %v, want %v", gotH, tt.wantH)
			}
		})
	}
}

func TestNode_deserialize(t *testing.T) {
	type fields struct {
		rootValue string
		value     string
		parent    *Node
		children  childMap
	}
	type args struct {
		ctx context.Context
		l   *lexer.Lexer
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantEnd bool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Node{
				rootValue: tt.fields.rootValue,
				value:     tt.fields.value,
				parent:    tt.fields.parent,
				children:  tt.fields.children,
			}
			gotEnd, err := h.deserialize(tt.args.ctx, tt.args.l)
			if (err != nil) != tt.wantErr {
				t.Errorf("Node.deserialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotEnd != tt.wantEnd {
				t.Errorf("Node.deserialize() = %v, want %v", gotEnd, tt.wantEnd)
			}
		})
	}
}
