package plan

import (
	"fmt"
	"github.com/vektah/gqlparser/v2/ast"
)

type QueryPlan struct {
	Node PlanNode // optional
}

type PlanNode interface {
	isPlanNode()
}

var _ PlanNode = (*SequenceNode)(nil)
var _ PlanNode = (*ParallelNode)(nil)
var _ PlanNode = (*FetchNode)(nil)
var _ PlanNode = (*FlattenNode)(nil)

type SequenceNode struct {
	Nodes []PlanNode
}

func (n *SequenceNode) isPlanNode() {}

type ParallelNode struct {
	Nodes []PlanNode
}

func (n *ParallelNode) isPlanNode() {}

type FetchNode struct {
	ServiceName    string
	VariableUsages []string
	Requires       []QueryPlanSelectionNode // optional
	Operation      string
}

func (n *FetchNode) isPlanNode() {}

type FlattenNode struct {
	Path ast.Path
	Node PlanNode
}

func (n *FlattenNode) isPlanNode() {}

type QueryPlanSelectionNode interface {
	isQueryPlanSelectionNode()
}

var _ QueryPlanSelectionNode = (*QueryPlanFieldNode)(nil)
var _ QueryPlanSelectionNode = (*QueryPlanInlineFragmentNode)(nil)

type QueryPlanFieldNode struct {
	Alias      string // optional
	Name       string
	Selections []QueryPlanSelectionNode // optional
}

func (n *QueryPlanFieldNode) isQueryPlanSelectionNode() {}

func (n *QueryPlanFieldNode) ResponseName() string {
	if n.Alias != "" {
		return n.Alias
	}

	return n.Name
}

type QueryPlanInlineFragmentNode struct {
	TypeCondition string                   // optional
	Selections    []QueryPlanSelectionNode // optional
}

func (n *QueryPlanInlineFragmentNode) isQueryPlanSelectionNode() {}

func TrimSelectionNodes(selections []ast.Selection) []QueryPlanSelectionNode {
	remapped := make([]QueryPlanSelectionNode, 0, len(selections))

	for _, selection := range selections {
		switch selection := selection.(type) {
		case *ast.Field:
			remapped = append(remapped, &QueryPlanFieldNode{
				Alias:      "",
				Name:       selection.Name,
				Selections: TrimSelectionNodes(selection.SelectionSet),
			})
		case *ast.InlineFragment:
			remapped = append(remapped, &QueryPlanInlineFragmentNode{
				TypeCondition: selection.TypeCondition,
				Selections:    TrimSelectionNodes(selection.SelectionSet),
			})
		default:
			panic(fmt.Sprintf("unexpected selection type: %T", selection))
		}
	}

	return remapped
}
