package federation

import (
	"context"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

type multierror []error

func (errs multierror) Error() string {
	if len(errs) == 1 {
		return errs[0].Error()
	}

	msgs := make([]string, len(errs))
	for idx, err := range errs {
		msgs[idx] = err.Error()
	}

	return strings.Join(msgs, "\n")
}

func ComposeAndValidate(ctx context.Context, serviceList []*ServiceDefinition) (schema *ast.Schema, supergraphSDL string, matadata *FederationMetadata, err error) {
	// NOTE: 全体的な設計方針
	//   js版はimmutableな構成になっていて、元データは破壊されない
	//   こちらの設計では非破壊にするのは手間が多い割りにそうする必要があるのか不明である
	//   実装当初は非破壊にしようと頑張っていたが次の2点の両立が難しい
	//     1. あるstructの値同士が同値比較が == でできるようにする (どこかで値が変わったら別の場所で参照している値も変更する)
	//     2. ASTの子孫に対して意図せぬ破壊的変更を行わない保証が難しい jsでの foo = { ...foo, bar: "buzz" } 相当の操作が難しい
	//   よって、残念ながらここでは破壊的変更を許容しコードを理解可能な状態に保つことを優先する

	var errors []error

	errors = validateServicesBeforeNormalization(ctx, serviceList)

	normalizedServiceList := make([]*ServiceDefinition, 0, len(serviceList))
	for _, service := range serviceList {
		typeDefs := service.TypeDefs
		typeDefs = normalizeTypeDefs(ctx, typeDefs)
		normalizedServiceList = append(normalizedServiceList, &ServiceDefinition{
			TypeDefs: typeDefs,
			Name:     service.Name,
			URL:      service.URL,
		})
	}

	errors = append(errors, validateServicesBeforeComposition(normalizedServiceList)...)

	schema, supergraphSDL, matadata, errs := composeServices(ctx, normalizedServiceList)

	if len(errs) != 0 {
		errors = append(errors, errs...)
	}

	errors = append(errors, validateComposedSchema(schema, matadata, serviceList)...)

	if len(errors) > 0 {
		err := multierror(errors)
		return schema, "", matadata, err
	}

	return schema, supergraphSDL, matadata, nil
}
