// SPDX-License-Identifier: NONE
package hierarchy

import (
	"context"
	"reflect"
	"testing"
)

func TestBuilderList_NewHierarchy(t *testing.T) {
	type args struct {
		ctx       context.Context
		rootValue []string
	}

	tests := []struct {
		name    string
		b       BuilderList
		args    args
		wantH   *Node
		wantErr bool
	}{
		{
			name:  "valid",
			b:     BuilderList{&DefaultBuilder{value: "1", parent: "0"}},
			args:  args{context.Background(), []string{"0"}},
			wantH: &Node{value: "1", rootValue: "0"},
			// wantErr: true,
		},
		{
			name:    "missing root node",
			b:       BuilderList{&DefaultBuilder{value: "1", parent: "3"}},
			args:    args{context.Background(), []string{"0"}},
			wantH:   nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotH, err := tt.b.NewHierarchy(tt.args.ctx, tt.args.rootValue...)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuilderList.NewHierarchy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotH, tt.wantH) {
				t.Errorf("BuilderList.NewHierarchy() = %v, want %v", gotH, tt.wantH)
			}
		})
	}
}
