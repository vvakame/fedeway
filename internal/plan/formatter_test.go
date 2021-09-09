package plan

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestFormatter(t *testing.T) {
	tests := []struct {
		name string
		node *QueryPlan
		want string
	}{
		{
			name: "blank",
			node: &QueryPlan{},
			want: heredoc.Doc(`
				QueryPlan {
				}
			`),
		},
		{
			name: "with fetch node",
			node: &QueryPlan{
				Node: &SequenceNode{
					Nodes: []PlanNode{
						&FetchNode{
							ServiceName:    "users",
							VariableUsages: []string{},
							Operation:      "{ me { id } }",
						},
					},
				},
			},
			want: heredoc.Doc(`
				QueryPlan {
					Sequence {
						Fetch(service: "users") {
							{ me { id } }
						},
					},
				}
			`),
		},
		{
			name: "with fetch node + requires",
			node: &QueryPlan{
				Node: &SequenceNode{
					Nodes: []PlanNode{
						&FetchNode{
							ServiceName:    "users",
							VariableUsages: []string{},
							Requires: []QueryPlanSelectionNode{
								&QueryPlanInlineFragmentNode{
									TypeCondition: "Product",
									Selections: []QueryPlanSelectionNode{
										&QueryPlanFieldNode{Name: "__typename"},
										&QueryPlanFieldNode{Name: "upc"},
									},
								},
							},
							Operation: "{ me { id } }",
						},
					},
				},
			},
			want: heredoc.Doc(`
				QueryPlan {
					Sequence {
						Fetch(service: "users") { ... on Product { __typename upc } =>
							{ me { id } }
						},
					},
				}
			`),
		},
		{
			name: "with flatten node",
			node: &QueryPlan{
				Node: &SequenceNode{
					Nodes: []PlanNode{
						&FlattenNode{
							Path: ast.Path{
								ast.PathName("me"),
								ast.PathName("@"),
								ast.PathName("product"),
							},
							Node: &FetchNode{
								ServiceName:    "users",
								VariableUsages: []string{},
								Operation:      "{ me { id } }",
							},
						},
					},
				},
			},
			want: heredoc.Doc(`
				QueryPlan {
					Sequence {
						Flatten(path: "me.@.product") {
							Fetch(service: "users") {
								{ me { id } }
							},
						},
					},
				}
			`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			f := NewFormatter(w)
			f.FormatQueryPlan(tt.node)

			if got := strings.TrimSpace(w.String()); got != strings.TrimSpace(tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}
