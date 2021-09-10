package planner

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vvakame/fedeway/internal/utils"
)

func createScope(qpctx *queryPlanningContext, parentType *ast.Definition) (*Scope, error) {
	return newScope(qpctx, parentType, nil, nil)
}

func newScope(qpctx *queryPlanningContext, parentType *ast.Definition, directives ast.DirectiveList, enclosing *Scope) (*Scope, error) {
	scope := &Scope{
		qpctx:      qpctx,
		parentType: parentType,
		directives: directives,
		enclosing:  enclosing,
	}

	return scope, nil
}

type Scope struct {
	qpctx      *queryPlanningContext
	parentType *ast.Definition
	directives ast.DirectiveList
	enclosing  *Scope

	cachedRuntimeTypes []*ast.Definition
	cachedIdentityKey  string
}

func (scope *Scope) MarshalLog() interface{} {
	if scope == nil {
		return nil
	}

	result := make(map[string]interface{})
	result["ParentType"] = scope.parentType.Name
	// TODO データ量いい感じに
	// result["Directives"] = scope.directives
	result["Enclosing"] = scope.enclosing.MarshalLog()
	return result
}

func (s *Scope) refine(ctx context.Context, typ *ast.Definition, directives ast.DirectiveList) (*Scope, error) {
	if len(directives) == 0 && typ == s.parentType {
		return s, nil
	}

	prunedScope, err := pruneRefinedTypes(s, typ)
	if err != nil {
		return nil, err
	}

	return newScope(s.qpctx, typ, directives, prunedScope)
}

func (s *Scope) computePossibleRuntimeTypes() []*ast.Definition {
	possibleTypes := s.qpctx.getPossibleTypes(s.parentType)

	nextScope := s.enclosing
	for nextScope != nil {
		enclosingPossibleTypes := s.qpctx.getPossibleTypes(nextScope.parentType)

		newPossibleTypes := make([]*ast.Definition, 0, len(possibleTypes))
		for _, typ := range possibleTypes {
			if containsType(enclosingPossibleTypes, typ) {
				newPossibleTypes = append(newPossibleTypes, typ)
				continue
			}
		}
		possibleTypes = newPossibleTypes

		nextScope = nextScope.enclosing
	}

	return possibleTypes
}

func (s *Scope) possibleRuntimeTypes() []*ast.Definition {
	if s.cachedRuntimeTypes == nil {
		s.cachedRuntimeTypes = s.computePossibleRuntimeTypes()
	}

	return s.cachedRuntimeTypes
}

func (s *Scope) identityKey() string {
	if s.cachedIdentityKey == "" {
		s.cachedIdentityKey = s.computeIdentityKey()
	}

	return s.cachedIdentityKey
}

func (s *Scope) computeIdentityKey() string {
	var directivesKey string
	if len(s.directives) != 0 {
		directivesKey = directivesIdentityKey(s.directives)
	}
	var enclosingKey string
	if s.enclosing != nil {
		enclosingKey = s.enclosing.computeIdentityKey()
	}

	return fmt.Sprintf("%s-%s-%s", s.parentType.Name, directivesKey, enclosingKey)
}

func pruneRefinedTypes(toPrune *Scope, refiningType *ast.Definition) (*Scope, error) {
	if toPrune == nil {
		return nil, nil
	}

	if len(toPrune.directives) == 0 && utils.IsTypeDefSubTypeOf(toPrune.qpctx.schema, refiningType, toPrune.parentType) {
		return pruneRefinedTypes(toPrune.enclosing, refiningType)
	}

	newEnclosing, err := pruneRefinedTypes(toPrune.enclosing, refiningType)
	if err != nil {
		return nil, err
	}

	return newScope(
		toPrune.qpctx,
		toPrune.parentType,
		toPrune.directives,
		newEnclosing,
	)
}

func (s *Scope) isStrictlyRefining(typ *ast.Definition) bool {
	// This scope will refine the provided type, unless that provided type is a subtype of all
	// the type in the chain.
	scope := s
	for scope != nil {
		if scope.parentType != typ && utils.IsTypeDefSubTypeOf(s.qpctx.schema, scope.parentType, typ) {
			return true
		}
		scope = scope.enclosing
	}
	return false
}

func valueIdentityKey(value *ast.Value) interface{} {
	switch value.Kind {
	case ast.Variable:
		return value.Raw
	case ast.IntValue:
		return "i" + value.Raw
	case ast.FloatValue:
		return "f" + value.Raw
	case ast.EnumValue:
		return "e" + value.Raw
	case ast.StringValue:
		return "s" + strconv.Quote(value.Raw)
	case ast.BooleanValue:
		return "b" + strconv.Quote(value.Raw)
	case ast.NullValue:
		return "<null>"
	case ast.ListValue:
		var val []string
		for _, elem := range value.Children {
			val = append(val, elem.Value.String())
		}
		return "[" + strings.Join(val, "-") + "]"
	case ast.ObjectValue:
		var val []string
		for _, elem := range value.Children {
			val = append(val, elem.Name+"-"+elem.Value.String())
		}
		return "{" + strings.Join(val, ",") + "}"
	default:
		panic(fmt.Sprintf("unexpected kind: %d", value.Kind))
	}
}

func directiveIdentityKey(d *ast.Directive) string {
	argsKeys := make([]string, 0, len(d.Arguments))
	for _, arg := range d.Arguments {
		key := fmt.Sprintf("%s-%s", arg.Name, valueIdentityKey(arg.Value))
		argsKeys = append(argsKeys, key)
	}

	sort.Strings(argsKeys)

	return fmt.Sprintf("%s-%s", d.Name, strings.Join(argsKeys, "-"))
}

func directivesIdentityKey(directives ast.DirectiveList) string {
	keys := make([]string, 0, len(directives))
	for _, d := range directives {
		keys = append(keys, directiveIdentityKey(d))
	}

	sort.Strings(keys)

	return strings.Join(keys, "-")
}
