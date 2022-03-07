// SPDX-License-Identifier: NONE
package hierarchy

import (
	"context"
	"testing"
)

func TestNode_Serialize(t *testing.T) {
	type fields struct {
		rootValue string
		value     string
		parent    *Node
		children  childMap
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantOutput string
		wantErr    bool
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
			gotOutput, err := h.Serialize(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Node.Serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOutput != tt.wantOutput {
				t.Errorf("Node.Serialize() = %v, want %v", gotOutput, tt.wantOutput)
			}
		})
	}
}

func TestNode_serialize(t *testing.T) {
	type fields struct {
		rootValue string
		value     string
		parent    *Node
		children  childMap
	}
	type args struct {
		ctx     context.Context
		serChan chan string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			h.serialize(tt.args.ctx, tt.args.serChan)
		})
	}
}
