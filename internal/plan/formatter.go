package plan

import (
	"fmt"
	"io"
	"strings"
)

type Formatter interface {
	FormatQueryPlan(queryPlan *QueryPlan)
}

func NewFormatter(w io.Writer) Formatter {
	return &formatter{writer: w}
}

type formatter struct {
	writer io.Writer

	indent int

	padNext  bool
	lineHead bool
}

func (f *formatter) writeString(s string) {
	_, _ = f.writer.Write([]byte(s))
}

func (f *formatter) writeIndent() *formatter {
	if f.lineHead {
		f.writeString(strings.Repeat("\t", f.indent))
	}
	f.lineHead = false
	f.padNext = false

	return f
}

func (f *formatter) WriteNewline() *formatter {
	f.writeString("\n")
	f.lineHead = true
	f.padNext = false

	return f
}

func (f *formatter) WriteWord(word string) *formatter {
	if f.lineHead {
		f.writeIndent()
	}
	if f.padNext {
		f.writeString(" ")
	}
	f.writeString(strings.TrimSpace(word))
	f.padNext = true

	return f
}

func (f *formatter) WriteString(s string) *formatter {
	if f.lineHead {
		f.writeIndent()
	}
	if f.padNext {
		f.writeString(" ")
	}
	f.writeString(s)
	f.padNext = false

	return f
}

func (f *formatter) IncrementIndent() {
	f.indent++
}

func (f *formatter) DecrementIndent() {
	f.indent--
}

func (f *formatter) NoPadding() *formatter {
	f.padNext = false

	return f
}

func (f *formatter) NeedPadding() *formatter {
	f.padNext = true

	return f
}

func (f *formatter) FormatQueryPlan(queryPlan *QueryPlan) {
	f.WriteWord("QueryPlan").WriteWord("{")
	f.FormatPlanNodes([]PlanNode{queryPlan.Node})
	f.WriteWord("}")
}

func (f *formatter) FormatPlanNodes(nodes []PlanNode) {
	for _, node := range nodes {
		if node == nil {
			continue
		}

		f.FormatPlanNode(node)
		f.WriteWord(",")
	}
}

func (f *formatter) FormatPlanNode(node PlanNode) {
	var nodes []PlanNode
	switch node := node.(type) {
	case *FetchNode:
		f.WriteString(`Fetch(service: "`)
		f.WriteString(node.ServiceName)
		f.WriteWord(`")`)

		f.WriteWord("{")

		if len(node.Requires) != 0 {
			f.FormatQueryPlanSelectionNodes(node.Requires)
			f.WriteWord("=>")
		}

		f.WriteWord(node.Operation) // TODO parse & format
		f.WriteWord("}")

	case *FlattenNode:
		f.WriteString(`Flatten(path: "`)
		f.WriteString(node.Path.String())
		f.WriteWord(`")`)

		nodes = []PlanNode{node.Node}
	case *SequenceNode:
		f.WriteWord("Sequence")
		nodes = node.Nodes
	case *ParallelNode:
		f.WriteWord("Parallel")
		nodes = node.Nodes
	}

	if len(nodes) != 0 {
		f.WriteWord("{")
		f.FormatPlanNodes(nodes)
		f.WriteWord("}")
	}
}

func (f *formatter) FormatQueryPlanSelectionNodes(nodes []QueryPlanSelectionNode) {
	for _, node := range nodes {
		f.FormatQueryPlanSelectionNode(node)
	}
}

func (f *formatter) FormatQueryPlanSelectionNode(node QueryPlanSelectionNode) {
	switch node := node.(type) {
	case *QueryPlanFieldNode:
		if node.Alias != "" {
			f.WriteString(node.Alias)
			f.WriteWord(":")
		}
		f.WriteWord(node.Name)

		if len(node.Selections) != 0 {
			f.WriteWord("{")
			f.FormatQueryPlanSelectionNodes(node.Selections)
			f.WriteWord("}")
		}

	case *QueryPlanInlineFragmentNode:
		f.WriteWord("... on")
		f.WriteWord(node.TypeCondition)

		f.WriteWord("{")
		f.FormatQueryPlanSelectionNodes(node.Selections)
		f.WriteWord("}")
	default:
		panic(fmt.Sprintf("unknown type: %T", node))
	}
}
