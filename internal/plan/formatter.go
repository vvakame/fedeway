package plan

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	gqlformatter "github.com/vektah/gqlparser/v2/formatter"
	"github.com/vektah/gqlparser/v2/parser"
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

func (f *formatter) WriteMultiLine(text string) *formatter {
	ss := strings.Split(text, "\n")
	for _, s := range ss {
		f.WriteString(s).WriteNewline()
	}
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
	f.IncrementIndent()

	f.FormatPlanNodes([]PlanNode{queryPlan.Node})

	f.DecrementIndent()
	f.WriteNewline()
	f.WriteString("}")
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
		f.WriteNewline()
		f.WriteString(`Fetch(service: "`)
		f.WriteString(node.ServiceName)
		f.WriteWord(`")`)

		f.WriteWord("{")
		f.IncrementIndent()

		if len(node.Requires) != 0 {
			f.FormatQueryPlanSelectionNodes(node.Requires)
			f.WriteWord("=>")
		}

		f.DecrementIndent()
		f.WriteNewline()

		f.IncrementIndent()

		f.formatOperationString(node.Operation)

		f.DecrementIndent()
		f.WriteString("}")

	case *FlattenNode:
		f.WriteNewline()
		f.WriteString(`Flatten(path: "`)
		f.WriteString(node.Path.String())
		f.WriteWord(`")`)

		nodes = []PlanNode{node.Node}
	case *SequenceNode:
		f.WriteNewline()
		f.WriteWord("Sequence")
		nodes = node.Nodes
	case *ParallelNode:
		f.WriteNewline()
		f.WriteWord("Parallel")
		nodes = node.Nodes
	}

	if len(nodes) != 0 {
		f.WriteWord("{")
		f.IncrementIndent()

		f.FormatPlanNodes(nodes)

		f.DecrementIndent()
		f.WriteNewline()
		f.WriteString("}")
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
		f.WriteNewline()
		if node.Alias != "" {
			f.WriteString(node.Alias)
			f.WriteWord(":")
		}
		f.WriteWord(node.Name)

		if len(node.Selections) != 0 {
			f.WriteWord("{")
			f.IncrementIndent()

			f.FormatQueryPlanSelectionNodes(node.Selections)

			f.DecrementIndent()
			f.WriteNewline()
			f.WriteWord("}")
		}

	case *QueryPlanInlineFragmentNode:
		f.WriteNewline()
		f.WriteWord("{")
		f.WriteNewline()
		f.IncrementIndent()

		f.WriteWord("... on")
		f.WriteWord(node.TypeCondition)
		f.WriteWord("{")
		f.IncrementIndent()

		f.FormatQueryPlanSelectionNodes(node.Selections)

		f.DecrementIndent()
		f.WriteNewline()
		f.WriteWord("}")
		f.WriteNewline()

		f.DecrementIndent()
		f.WriteWord("}")
	default:
		panic(fmt.Sprintf("unknown type: %T", node))
	}
}

func (f *formatter) formatOperationString(operation string) {
	document, err := parser.ParseQuery(&ast.Source{
		Input: operation,
	})
	if err != nil {
		panic(err)
	}
	document = flattenEntitiesField(document)
	var buf bytes.Buffer
	gqlformatter.NewFormatter(&buf).FormatQueryDocument(document)
	f.WriteMultiLine(strings.TrimSpace(buf.String()))
}

func flattenEntitiesField(document *ast.QueryDocument) *ast.QueryDocument {
	for _, operation := range document.Operations {
		if operation.Operation != ast.Query {
			continue
		} else if len(operation.SelectionSet) == 0 {
			continue
		}

		firstSelection := operation.SelectionSet[0]
		field, ok := firstSelection.(*ast.Field)
		if !ok || field == nil {
			continue
		}

		if field.Name != "_entities" {
			continue
		}

		document = &ast.QueryDocument{
			Operations: ast.OperationList{
				&ast.OperationDefinition{
					Operation:    "",
					SelectionSet: field.SelectionSet,
				},
			},
			Fragments: document.Fragments,
		}
		break
	}

	return document
}
