// SPDX-License-Identifier: MIT
package hierarchy

/* func TestBuilderList_NewHierarchy(t *testing.T) {
 *     type args struct {
 *         ctx context.Context
 *     }
 *
 *     tests := []struct {
 *         name    string
 *         b       BuilderList
 *         args    args
 *         wantH   *Hierarchy
 *         wantErr bool
 *     }{
 *         {
 *             name:  "valid",
 *             b:     BuilderList{&DefaultBuilder{value: "1", parent: ""}},
 *             args:  args{context.Background()},
 *             wantH: &Hierarchy{value: "1", children: childMap{}},
 *             // wantErr: true,
 *         },
 *         {
 *             name:    "invalid (missing root node)",
 *             b:       BuilderList{&DefaultBuilder{value: "1", parent: "3"}},
 *             args:    args{context.Background()},
 *             wantH:   nil,
 *             wantErr: true,
 *         },
 *     }
 *
 *     for _, tt := range tests {
 *         t.Run(tt.name, func(t *testing.T) {
 *             gotH, err := tt.b.NewHierarchy(tt.args.ctx)
 *             if (err != nil) != tt.wantErr {
 *                 t.Errorf("BuilderList.NewHierarchy() error = %v, wantErr %v", err, tt.wantErr)
 *                 return
 *             }
 *             if !reflect.DeepEqual(gotH, tt.wantH) {
 *                 t.Errorf("BuilderList.NewHierarchy() = %v, want %v", gotH, tt.wantH)
 *             }
 *         })
 *     }
 * } */
