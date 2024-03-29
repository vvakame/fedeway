package federation

import (
	pkgerrors "errors"
	"fmt"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vvakame/fedeway/internal/graphql"
)

func postCompositionValidators() []func(*ast.Schema, *FederationMetadata, []*ServiceDefinition) []error {
	return []func(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error{
		externalUnused,
		externalMissingOnBase,
		externalTypeMismatch,
		requiresFieldsMissingExternal,
		requiresFieldsMissingOnBase,
		keyFieldsMissingOnBase,
		keyFieldsSelectInvalidType,
		providesFieldsMissingExternal,
		providesFieldsSelectInvalidType,
		providesNotOnEntity,
		executableDirectivesInAllServices,
		executableDirectivesIdentical,
		keysMatchBaseService,
	}
}

// for every @external field, there should be a @requires, @key, or @provides
// directive that uses it
func externalUnused(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, parentTypeName := range typeNames {
		parentType := schema.Types[parentTypeName]

		// Only object types have fields
		if parentType.Kind != ast.Object {
			continue
		}

		// If externals is populated, we need to look at each one and confirm
		// it is used
		typeFederationMetadata := metadata.FederationTypeMap.Get(parentType)

		// Escape a validation case that's falling through incorrectly. This case
		// is handled by `keysMatchBaseService`.
		if typeFederationMetadata != nil {
			serviceName := typeFederationMetadata.ServiceName
			keys := typeFederationMetadata.Keys
			if serviceName != "" && keys != nil && keys[serviceName] == nil {
				continue
			}
		}

		if len(typeFederationMetadata.Externals) == 0 {
			continue
		}

		// loop over every service that has extensions with @external
		serviceNames := make([]string, 0, len(typeFederationMetadata.Externals))
		for serviceName := range typeFederationMetadata.Externals {
			serviceNames = append(serviceNames, serviceName)
		}
		sort.Strings(serviceNames)
		for _, serviceName := range serviceNames {
			externalFieldsForService := typeFederationMetadata.Externals[serviceName]
			// for a single service, loop over the external fields.
		OUTER:
			for _, externalFields := range externalFieldsForService {
				externalField := externalFields.Field
				externalFieldName := externalField.Name

				// check the selected fields of every @key provided by `serviceName`
				for _, selections := range typeFederationMetadata.Keys[externalFields.ServiceName] {
					for _, selection := range selections {
						field, ok := selection.(*ast.Field)
						if !ok {
							continue
						}

						if field.Name == externalFieldName {
							continue OUTER
						}
					}
				}

				// @provides is most commonly used from another type than where
				// the @external directive is applied. We need to find all
				// fields on any type in the schema that return this type
				// and see if they have a provides directive that uses this
				// external field

				// extend type Review {
				//   author: User @provides(fields: "username")
				// }

				// extend type User @key(fields: "id") {
				//   id: ID! @external
				//   username: String @external
				//   reviews: [Review]
				// }
				fields := findFieldsThatReturnType(
					schema,
					parentType,
				)
				for _, field := range fields {
					fieldMeta := metadata.FederationFieldMap.Get(field)
					if len(fieldMeta.Provides) == 0 {
						continue
					}

					// find the selections which are fields with names matching
					// our external field name
					for _, selection := range fieldMeta.Provides {
						field, ok := selection.(*ast.Field)
						if !ok {
							continue
						}
						if field.Name == externalFieldName {
							continue OUTER
						}
					}
				}

				// @external fields can be selected by subfields of a selection on another type
				//
				// For example, with these defs, `canWrite` is marked as external and is
				// referenced by a selection set inside the @requires of User.isAdmin
				//
				//    extend type User @key(fields: "id") {
				//      roles: AccountRoles!
				//      isAdmin: Boolean! @requires(fields: "roles { canWrite permission { status } }")
				//    }
				//    extend type AccountRoles {
				//      canWrite: Boolean @external
				//      permission: Permission @external
				//    }
				//
				//    extend type Permission {
				//      status: String @external
				//    }
				//
				// So, we need to search for fields with requires, then parse the selection sets,
				// and try to recursively find the external field's PARENT type, then the external field's name
				for _, namedType := range schema.Types {
					if namedType.Kind != ast.Object {
						continue
					}

					// for every object type, loop over its fields and find fields
					// with requires directives
					for _, field := range namedType.Fields {
						fieldMeta := metadata.FederationFieldMap.Get(field)
						if len(fieldMeta.Requires) == 0 {
							continue
						}
						if selectionIncludesField(schema, fieldMeta.Requires, namedType, parentType, externalFieldName) {
							continue OUTER
						}
					}
				}

				for _, maybeRequiresField := range parentType.Fields {
					fieldMeta := metadata.FederationFieldMap.Get(maybeRequiresField)
					fieldOwner := fieldMeta.ServiceName
					if fieldOwner != serviceName {
						continue
					}

					for _, selection := range fieldMeta.Requires {
						field, ok := selection.(*ast.Field)
						if !ok {
							continue
						}

						if field.Name == externalFieldName {
							continue OUTER
						}
					}
				}

				// @external fields can be required when an interface is returned by
				// a field and its concrete implementations need to be defined in a
				// service which use non-key fields from other services. Take for example:
				//
				//  // Service A
				//  type Car implements Vehicle @key(fields: "id") {
				//    id: ID!
				//    speed: Int
				//  }
				//
				//  interface Vehicle {
				//    id: ID!
				//    speed: Int
				//  }
				//
				//  // Service B
				//  type Query {
				//    vehicles: [Vehicle]
				//  }
				//
				//  extend type Car implements Vehicle @key(fields: "id") {
				//    id: ID! @external
				//    speed: Int @external
				//  }
				//
				//  interface Vehicle {
				//    id: ID!
				//    speed: Int
				//  }
				//
				//  Service B defines Car.speed as an external field which is okay
				//  because it is required for Query.vehicles to exist in the schema

				// Loop over the parent's interfaces
				for _, interfaceName := range parentType.Interfaces {
					// Collect the field names from each interface in a set
					for _, field := range schema.Types[interfaceName].Fields {
						if field.Name == externalFieldName {
							continue OUTER
						}
					}
				}

				gErr := gqlerror.ErrorPosf(
					externalField.Directives.ForName("external").Position,
					"%s is marked as @external but is not used by a @requires, @key, or @provides directive.",
					logServiceAndType(serviceName, parentTypeName, externalFieldName),
				)
				if gErr.Extensions == nil {
					gErr.Extensions = make(map[string]interface{})
				}
				gErr.Extensions["code"] = "EXTERNAL_UNUSED"
				errors = append(errors, gErr)
			}
		}
	}

	return errors
}

// All fields marked with @external must exist on the base type
func externalMissingOnBase(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		namedType := schema.Types[typeName]
		// Only object types have fields
		if namedType.Kind != ast.Object {
			continue
		}

		typeFederationMetadata := metadata.FederationTypeMap.Get(namedType)

		// If externals is populated, we need to look at each one and confirm
		// that field exists on base service
		if len(typeFederationMetadata.Externals) == 0 {
			continue
		}

		// loop over every service that has extensions with @external
		serviceNames := make([]string, 0, len(typeFederationMetadata.Externals))
		for serviceName := range typeFederationMetadata.Externals {
			serviceNames = append(serviceNames, serviceName)
		}
		sort.Strings(serviceNames)
		for _, serviceName := range serviceNames {
			externalFieldsForService := typeFederationMetadata.Externals[serviceName]
			// for a single service, loop over the external fields.
			for _, externalFieldForService := range externalFieldsForService {
				externalField := externalFieldForService.Field
				externalFieldName := externalField.Name
				matchingBaseField := namedType.Fields.ForName(externalFieldName)

				// @external field referenced a field that isn't defined anywhere
				if matchingBaseField == nil {
					gErr := gqlerror.ErrorPosf(
						externalField.Directives.ForName("external").Position,
						"%s marked @external but %s is not defined on the base service of %s (%s)",
						logServiceAndType(serviceName, typeName, externalFieldName),
						externalFieldName,
						typeName,
						typeFederationMetadata.ServiceName,
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "EXTERNAL_MISSING_ON_BASE"
					errors = append(errors, gErr)
					continue
				}

				// if the field has a serviceName, then it wasn't defined by the
				// service that owns the type
				fieldFederationMetadata := metadata.FederationFieldMap.Get(matchingBaseField)

				if fieldFederationMetadata.ServiceName != "" {
					gErr := gqlerror.ErrorPosf(
						externalField.Directives.ForName("external").Position,
						"%s marked @external but %s was defined in %s, not in the service that owns %s (%s)",
						logServiceAndType(serviceName, typeName, externalFieldName),
						externalFieldName,
						fieldFederationMetadata.ServiceName,
						typeName,
						typeFederationMetadata.ServiceName,
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "EXTERNAL_MISSING_ON_BASE"
					errors = append(errors, gErr)
				}
			}
		}
	}

	return errors
}

// All fields marked with @external must match the type definition of the base service.
// Additional warning if the type of the @external field doesn't exist at all on the schema
func externalTypeMismatch(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		namedType := schema.Types[typeName]
		// Only object types have fields
		if namedType.Kind != ast.Object {
			continue
		}

		// If externals is populated, we need to look at each one and confirm
		// there is a matching @requires
		typeFederationMetadata := metadata.FederationTypeMap.Get(namedType)
		if len(typeFederationMetadata.Externals) == 0 {
			continue
		}

		// loop over every service that has extensions with @external
		serviceNames := make([]string, 0, len(typeFederationMetadata.Externals))
		for serviceName := range typeFederationMetadata.Externals {
			serviceNames = append(serviceNames, serviceName)
		}
		sort.Strings(serviceNames)
		for _, serviceName := range serviceNames {
			externalFieldsForService := typeFederationMetadata.Externals[serviceName]
			// for a single service, loop over the external fields.
			for _, externalFieldForService := range externalFieldsForService {
				externalField := externalFieldForService.Field
				externalFieldName := externalField.Name
				matchingBaseField := namedType.Fields.ForName(externalFieldName)

				externalFieldType := externalField.Type

				if schema.Types[externalField.Type.Name()] == nil {
					gErr := gqlerror.ErrorPosf(
						externalField.Type.Position,
						"%s the type of the @external field does not exist in the resulting composed schema",
						logServiceAndType(serviceName, typeName, externalFieldName),
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "EXTERNAL_TYPE_MISMATCH"
					errors = append(errors, gErr)

				} else if matchingBaseField != nil && matchingBaseField.Type.String() != externalFieldType.String() {
					gErr := gqlerror.ErrorPosf(
						externalField.Type.Position,
						"%s Type `%s` does not match the type of the original field in %s (`%s`)",
						logServiceAndType(serviceName, typeName, externalFieldName),
						externalFieldType.String(),
						typeFederationMetadata.ServiceName,
						matchingBaseField.Type.String(),
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "EXTERNAL_TYPE_MISMATCH"
					errors = append(errors, gErr)
				}
			}
		}
	}

	return errors
}

// for every @requires, there should be a matching @external
func requiresFieldsMissingExternal(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		namedType := schema.Types[typeName]
		// Only object types have fields
		if namedType.Kind != ast.Object {
			continue
		}

		// for each field, if there's a requires on it, check that there's a matching
		// @external field, and that the types referenced are from the base type
		for _, field := range namedType.Fields {
			fieldName := field.Name
			fieldFederationMetadata := metadata.FederationFieldMap.Get(field)
			serviceName := fieldFederationMetadata.ServiceName

			// serviceName should always exist on fields that have @requires federation data, since
			// the only case where serviceName wouldn't exist is on a base type, and in that case,
			// the `requires` metadata should never get added to begin with. This should be caught in
			// composition work. This kind of error should be validated _before_ composition.
			if serviceName == "" {
				continue
			}

			if len(fieldFederationMetadata.Requires) == 0 {
				continue
			}

			typeFederationMetadata := metadata.FederationTypeMap.Get(namedType)
			externalFieldsOnTypeForService := typeFederationMetadata.Externals[serviceName]

			selections := fieldFederationMetadata.Requires
			for _, selection := range selections {
				selectionField := selection.(*ast.Field)

				var foundMatchingExternal bool
				for _, ext := range externalFieldsOnTypeForService {
					if ext.Field.Name == selectionField.Name {
						foundMatchingExternal = true
						break
					}
				}
				if !foundMatchingExternal {
					typeNode := findTypeNodeInServiceList(typeName, serviceName, serviceList)
					fieldNode := typeNode.Fields.ForName(fieldName)

					gErr := gqlerror.ErrorPosf(
						fieldNode.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
						"%s requires the field `%s` to be marked as @external.",
						logServiceAndType(serviceName, typeName, fieldName),
						selectionField.Name,
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "REQUIRES_FIELDS_MISSING_EXTERNAL"
					errors = append(errors, gErr)
				}
			}
		}
	}

	return errors
}

// The fields arg in @requires can only reference fields on the base type
func requiresFieldsMissingOnBase(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		namedType := schema.Types[typeName]
		// Only object types have fields
		if namedType.Kind != ast.Object {
			continue
		}

		// for each field, if there's a requires on it, check that there's a matching
		// @external field, and that the types referenced are from the base type
		for _, field := range namedType.Fields {
			fieldName := field.Name
			fieldFederationMetadata := metadata.FederationFieldMap.Get(field)
			serviceName := fieldFederationMetadata.ServiceName

			// serviceName should always exist on fields that have @requires federation data, since
			// the only case where serviceName wouldn't exist is on a base type, and in that case,
			// the `requires` metadata should never get added to begin with. This should be caught in
			// composition work. This kind of error should be validated _before_ composition.
			if serviceName == "" {
				continue
			}

			if len(fieldFederationMetadata.Requires) == 0 {
				continue
			}

			selections := fieldFederationMetadata.Requires
			for _, selection := range selections {
				selectionField := selection.(*ast.Field)

				// check the selections are from the _base_ type (no serviceName)
				matchingFieldOnType := namedType.Fields.ForName(selectionField.Name)
				typeFederationMetadata := metadata.FederationFieldMap.Get(matchingFieldOnType)

				if typeFederationMetadata.ServiceName != "" {
					typeNode := findTypeNodeInServiceList(typeName, serviceName, serviceList)
					fieldNode := typeNode.Fields.ForName(fieldName)

					gErr := gqlerror.ErrorPosf(
						fieldNode.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
						"%s requires the field `%s` to be @external. @external fields must exist on the base type, not an extension.",
						logServiceAndType(serviceName, typeName, fieldName),
						selectionField.Name,
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "REQUIRES_FIELDS_MISSING_ON_BASE"
					errors = append(errors, gErr)
				}
			}
		}
	}

	return errors
}

// The fields argument can not select fields that were overwritten by another service
func keyFieldsMissingOnBase(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		namedType := schema.Types[typeName]
		// Only object types have fields
		if namedType.Kind != ast.Object {
			continue
		}

		typeFederationMetadata := metadata.FederationTypeMap.Get(namedType)

		if len(typeFederationMetadata.Keys) == 0 {
			continue
		}

		allFieldsInType := namedType.Fields

		serviceNames := make([]string, 0, len(typeFederationMetadata.Keys))
		for serviceName := range typeFederationMetadata.Keys {
			serviceNames = append(serviceNames, serviceName)
		}
		sort.Strings(serviceNames)
		for _, serviceName := range serviceNames {
			selectionSets := typeFederationMetadata.Keys[serviceName]

			for _, selectionSet := range selectionSets {
				for _, selection := range selectionSet {
					field := selection.(*ast.Field)

					name := field.Name

					// find corresponding field for each selected field
					matchingField := allFieldsInType.ForName(name)

					// NOTE: We don't need to warn if there is no matching field.
					// keyFieldsSelectInvalidType already does that :)
					if matchingField != nil {
						typeNode := findTypeNodeInServiceList(typeName, serviceName, serviceList)
						fieldNode := typeNode.Fields.ForName(name)

						fieldFederationMetadata := metadata.FederationFieldMap.Get(matchingField)

						// warn if not from base type OR IF IT WAS OVERWITTEN
						if fieldFederationMetadata.ServiceName != "" {
							gErr := gqlerror.ErrorPosf(
								fieldNode.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
								"%s A @key selects %s, but %s.%s was either created or overwritten by %s, not %s",
								logServiceAndType(serviceName, typeName, ""),
								name,
								typeName,
								name,
								fieldFederationMetadata.ServiceName,
								serviceName,
							)
							if gErr.Extensions == nil {
								gErr.Extensions = make(map[string]interface{})
							}
							gErr.Extensions["code"] = "KEY_FIELDS_MISSING_ON_BASE"
							errors = append(errors, gErr)
						}
					}
				}
			}
		}
	}

	return errors
}

// The fields argument can not have root fields that result in a list.
// The fields argument can not have root fields that result in an interface.
// The fields argument can not have root fields that result in a union type.
func keyFieldsSelectInvalidType(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		namedType := schema.Types[typeName]
		// Only object types have fields
		if namedType.Kind != ast.Object {
			continue
		}

		typeFederationMetadata := metadata.FederationTypeMap.Get(namedType)

		if len(typeFederationMetadata.Keys) == 0 {
			continue
		}

		allFieldsInType := namedType.Fields

		serviceNames := make([]string, 0, len(typeFederationMetadata.Keys))
		for serviceName := range typeFederationMetadata.Keys {
			serviceNames = append(serviceNames, serviceName)
		}
		sort.Strings(serviceNames)
		for _, serviceName := range serviceNames {
			selectionSets := typeFederationMetadata.Keys[serviceName]

			for _, selectionSet := range selectionSets {
				for _, selection := range selectionSet {
					field := selection.(*ast.Field)

					name := field.Name

					// find corresponding field for each selected field
					matchingField := allFieldsInType.ForName(name)

					typeNode := findTypeNodeInServiceList(typeName, serviceName, serviceList)
					fieldNode := typeNode.Fields.ForName(name)

					if matchingField == nil {
						gErr := gqlerror.ErrorPosf(
							fieldNode.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
							"%s A @key selects %s, but %s.%s could not be found",
							logServiceAndType(serviceName, typeName, ""),
							name,
							typeName,
							name,
						)
						if gErr.Extensions == nil {
							gErr.Extensions = make(map[string]interface{})
						}
						gErr.Extensions["code"] = "KEY_FIELDS_SELECT_INVALID_TYPE"
						errors = append(errors, gErr)
					}

					if matchingField != nil {
						if schema.Types[matchingField.Type.Name()].Kind == ast.Interface {
							gErr := gqlerror.ErrorPosf(
								fieldNode.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
								"%s A @key selects %s.%s, which is an interface type. Keys cannot select interfaces.",
								logServiceAndType(serviceName, typeName, ""),
								typeName,
								name,
							)
							if gErr.Extensions == nil {
								gErr.Extensions = make(map[string]interface{})
							}
							gErr.Extensions["code"] = "KEY_FIELDS_SELECT_INVALID_TYPE"
							errors = append(errors, gErr)
						}

						if schema.Types[matchingField.Type.Name()].Kind == ast.Union {
							gErr := gqlerror.ErrorPosf(
								fieldNode.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
								"%s A @key selects %s.%s, which is a union type. Keys cannot select union types.",
								logServiceAndType(serviceName, typeName, ""),
								typeName,
								name,
							)
							if gErr.Extensions == nil {
								gErr.Extensions = make(map[string]interface{})
							}
							gErr.Extensions["code"] = "KEY_FIELDS_SELECT_INVALID_TYPE"
							errors = append(errors, gErr)
						}
					}
				}
			}
		}
	}

	return errors
}

// for every field in a @provides, there should be a matching @external
func providesFieldsMissingExternal(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		namedType := schema.Types[typeName]
		// Only object types have fields
		if namedType.Kind != ast.Object {
			continue
		}

		// for each field, if there's a requires on it, check that there's a matching
		// @external field, and that the types referenced are from the base type
		for _, field := range namedType.Fields {
			fieldName := field.Name
			fieldFederationMetadata := metadata.FederationFieldMap.Get(field)
			serviceName := fieldFederationMetadata.ServiceName

			// serviceName should always exist on fields that have @provides federation data, since
			// the only case where serviceName wouldn't exist is on a base type, and in that case,
			// the `provides` metadata should never get added to begin with. This should be caught in
			// composition work. This kind of error should be validated _before_ composition.
			if serviceName == "" {
				continue
			}

			fieldType := schema.Types[field.Type.Name()]
			if fieldType.Kind != ast.Object {
				continue
			}

			fieldTypeFederationMetadata := metadata.FederationTypeMap.Get(fieldType)

			externalFieldsOnTypeForService := fieldTypeFederationMetadata.Externals[serviceName]

			selections := fieldFederationMetadata.Provides

			for _, selection := range selections {
				field := selection.(*ast.Field)

				var foundMatchingExternal bool
				for _, ext := range externalFieldsOnTypeForService {
					if ext.Field.Name == field.Name {
						foundMatchingExternal = true
						break
					}
				}

				if !foundMatchingExternal {
					typeNode := findTypeNodeInServiceList(typeName, serviceName, serviceList)
					var target *ast.FieldDefinition
					for _, typeField := range typeNode.Fields {
						if typeField.Name == field.Name {
							target = typeField
						}
					}

					gErr := gqlerror.ErrorPosf(
						target.Position,
						"%s provides the field `%s` and requires %s.%s to be marked as @external.",
						logServiceAndType(serviceName, typeName, fieldName),
						field.Name,
						fieldType.Name,
						field.Name,
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "PROVIDES_FIELDS_MISSING_EXTERNAL"
					errors = append(errors, gErr)
				}
			}
		}
	}

	return errors
}

// The fields argument can not have root fields that result in a list.
// The fields argument can not have root fields that result in an interface.
// The fields argument can not have root fields that result in a union type.
func providesFieldsSelectInvalidType(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		namedType := schema.Types[typeName]
		// Only object types have fields
		if namedType.Kind != ast.Object {
			continue
		}

		// for each field, if there's a provides on it, check the type of the field
		// it references
		for _, field := range namedType.Fields {
			fieldName := field.Name
			fieldFederationMetadata := metadata.FederationFieldMap.Get(field)
			serviceName := fieldFederationMetadata.ServiceName

			// serviceName should always exist on fields that have @provides federation data, since
			// the only case where serviceName wouldn't exist is on a base type, and in that case,
			// the `provides` metadata should never get added to begin with. This should be caught in
			// composition work. This kind of error should be validated _before_ composition.
			if serviceName == "" {
				continue
			}

			fieldType := schema.Types[field.Type.Name()]
			if fieldType.Kind != ast.Object {
				continue
			}

			allFields := fieldType.Fields

			if len(fieldFederationMetadata.Provides) == 0 {
				continue
			}

			selections := fieldFederationMetadata.Provides

			for _, selection := range selections {
				selectionField := selection.(*ast.Field)
				name := selectionField.Name
				matchingField := allFields.ForName(name)

				if matchingField == nil {
					gErr := gqlerror.ErrorPosf(
						field.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
						"%s A @provides selects %s, but %s.%s could not be found",
						logServiceAndType(serviceName, typeName, fieldName),
						name,
						fieldType.Name,
						name,
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "PROVIDES_FIELDS_SELECT_INVALID_TYPE"
					errors = append(errors, gErr)
					continue
				}

				if matchingField.Type.Elem != nil {
					gErr := gqlerror.ErrorPosf(
						field.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
						"%s A @provides selects %s.%s, which is a list type. A field cannot @provide lists.",
						logServiceAndType(serviceName, typeName, fieldName),
						fieldType.Name,
						name,
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "PROVIDES_FIELDS_SELECT_INVALID_TYPE"
					errors = append(errors, gErr)
				}
				if schema.Types[matchingField.Type.Name()].Kind == ast.Interface {
					gErr := gqlerror.ErrorPosf(
						field.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
						"%s A @provides selects %s.%s, which is an interface type. A field cannot @provide interfaces.",
						logServiceAndType(serviceName, typeName, fieldName),
						fieldType.Name,
						name,
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "PROVIDES_FIELDS_SELECT_INVALID_TYPE"
					errors = append(errors, gErr)
				}
				if schema.Types[matchingField.Type.Name()].Kind == ast.Union {
					gErr := gqlerror.ErrorPosf(
						field.Position, // TODO エラーを出力する箇所が厳密に元の実装を踏襲していない directiveのvalueのposはstripされていてわからなくなってしまっているため
						"%s A @provides selects %s.%s, which is a union type. A field cannot @provide union types.",
						logServiceAndType(serviceName, typeName, fieldName),
						fieldType.Name,
						name,
					)
					if gErr.Extensions == nil {
						gErr.Extensions = make(map[string]interface{})
					}
					gErr.Extensions["code"] = "PROVIDES_FIELDS_SELECT_INVALID_TYPE"
					errors = append(errors, gErr)
				}
			}
		}
	}

	return errors
}

// Provides directive can only be added to return types that are entities
func providesNotOnEntity(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		namedType := schema.Types[typeName]
		// Only object types have fields
		if namedType.Kind != ast.Object {
			continue
		}

		// for each field, if there's a provides on it, check that the containing
		// type has a `key` field under the federation metadata.
		for _, field := range namedType.Fields {
			fieldName := field.Name
			fieldFederationMetadata := metadata.FederationFieldMap.Get(field)
			serviceName := fieldFederationMetadata.ServiceName

			// serviceName should always exist on fields that have @provides federation data, since
			// the only case where serviceName wouldn't exist is on a base type, and in that case,
			// the `provides` metadata should never get added to begin with. This should be caught in
			// composition work. This kind of error should be validated _before_ composition.
			if serviceName == "" &&
				len(fieldFederationMetadata.Provides) != 0 &&
				!fieldFederationMetadata.BelongsToValueType {
				return []error{pkgerrors.New("Internal Consistency Error: field with provides information does not have service name.")}
			}
			if serviceName == "" {
				continue
			}

			baseType := schema.Types[field.Type.Name()]

			// field has a @provides directive on it
			if len(fieldFederationMetadata.Provides) == 0 {
				continue
			}

			typeNode := findTypeNodeInServiceList(typeName, serviceName, serviceList)
			fieldNode := typeNode.Fields.ForName(fieldName)

			if baseType.Kind != ast.Object {
				gErr := gqlerror.ErrorPosf(
					fieldNode.Position,
					"%s uses the @provides directive but `%s.%s` returns `%s`, which is not an Object or List type. @provides can only be used on Object types with at least one @key, or Lists of such Objects.",
					logServiceAndType(serviceName, typeName, fieldName),
					typeName,
					fieldName,
					field.Type.String(),
				)
				if gErr.Extensions == nil {
					gErr.Extensions = make(map[string]interface{})
				}
				gErr.Extensions["code"] = "PROVIDES_NOT_ON_ENTITY"
				errors = append(errors, gErr)

				continue
			}

			fieldType := schema.Types[baseType.Name]
			selectedFieldIsEntity := len(metadata.FederationTypeMap.Get(fieldType).Keys) != 0

			if !selectedFieldIsEntity {
				gErr := gqlerror.ErrorPosf(
					fieldNode.Position,
					"%s uses the @provides directive but `%s.%s` does not return a type that has a @key. Try adding a @key to the `%s` type.",
					logServiceAndType(serviceName, typeName, fieldName),
					typeName,
					fieldName,
					field.Type.String(),
				)
				if gErr.Extensions == nil {
					gErr.Extensions = make(map[string]interface{})
				}
				gErr.Extensions["code"] = "PROVIDES_NOT_ON_ENTITY"
				errors = append(errors, gErr)

			}
		}
	}

	return errors
}

// All custom directives with executable locations must be implemented in every
// service. This validator is not responsible for ensuring the directives are an
// ExecutableDirective, however composition ensures this by filtering out all
// TypeSystemDirectiveLocations.
func executableDirectivesInAllServices(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	var customExecutableDirectives ast.DirectiveDefinitionList
	directiveNames := make([]string, 0, len(schema.Directives))
	for directiveName := range schema.Directives {
		directiveNames = append(directiveNames, directiveName)
	}
	sort.Strings(directiveNames)
	for _, directiveName := range directiveNames {
		directive := schema.Directives[directiveName]
		if isApolloTypeSystemDirective(directive.Name) {
			continue
		}
		if isFederationDirective(directive.Name) {
			continue
		}
		customExecutableDirectives = append(customExecutableDirectives, directive)
	}

	for _, directive := range customExecutableDirectives {
		directiveFederationMetadata := metadata.FederationDirectiveMap.Get(directive)
		if len(directiveFederationMetadata.DirectiveDefinitions) == 0 {
			continue
		}

		allServiceNames := make([]string, 0, len(serviceList))
		for _, service := range serviceList {
			allServiceNames = append(allServiceNames, service.Name)
		}

		var serviceNamesWithDirective []string
		for serviceName := range directiveFederationMetadata.DirectiveDefinitions {
			serviceNamesWithDirective = append(serviceNamesWithDirective, serviceName)
		}
		sort.Strings(serviceNamesWithDirective)

		var serviceNamesWithoutDirective []string
	OUTER:
		for _, serviceName := range allServiceNames {
			for _, serviceName2 := range serviceNamesWithDirective {
				if serviceName2 == serviceName {
					continue OUTER
				}
			}
			serviceNamesWithoutDirective = append(serviceNamesWithoutDirective, serviceName)
		}

		if len(serviceNamesWithoutDirective) > 0 {
			gErr := gqlerror.ErrorPosf(
				// TODO (Issue #705): when we can associate locations to service names, we should expose
				// locations of the services where this directive is not used
				directive.Position,
				"%s Custom directives must be implemented in every service. The following services do not implement the @%s directive: %s.",
				logDirective(directive.Name),
				directive.Name,
				strings.Join(serviceNamesWithoutDirective, ", "),
			)
			if gErr.Extensions == nil {
				gErr.Extensions = make(map[string]interface{})
			}
			gErr.Extensions["code"] = "EXECUTABLE_DIRECTIVES_IN_ALL_SERVICES"
			errors = append(errors, gErr)
		}
	}

	return errors
}

// A custom directive must be defined identically across all services. This means
// they must have the same name and same locations. Locations are the "on" part of
// a directive, for example:
//    directive @stream on FIELD | QUERY
func executableDirectivesIdentical(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	var customDirectives ast.DirectiveDefinitionList
	directiveNames := make([]string, 0, len(schema.Directives))
	for directiveName := range schema.Directives {
		directiveNames = append(directiveNames, directiveName)
	}
	sort.Strings(directiveNames)
	for _, directiveName := range directiveNames {
		directive := schema.Directives[directiveName]
		if isApolloTypeSystemDirective(directive.Name) {
			continue
		}
		if graphql.IsSpecifiedDirective(directive.Name) {
			continue
		}
		customDirectives = append(customDirectives, directive)
	}

	for _, directive := range customDirectives {
		directiveFederationMetadata := metadata.FederationDirectiveMap.Get(directive)
		if len(directiveFederationMetadata.DirectiveDefinitions) == 0 {
			continue
		}

		serviceNames := make([]string, 0, len(directiveFederationMetadata.DirectiveDefinitions))
		for serviceName := range directiveFederationMetadata.DirectiveDefinitions {
			serviceNames = append(serviceNames, serviceName)
		}
		sort.Strings(serviceNames)

		// Side-by-side compare all definitions of a single directive, if there's a
		// discrepancy in any of those diffs, we should provide an error.
		var targetDefinition *ast.DirectiveDefinition
		for index, serviceName := range serviceNames {
			definition := directiveFederationMetadata.DirectiveDefinitions[serviceName]

			// Skip the non-comparison step
			if index == 0 {
				continue
			}

			previousDefinition := directiveFederationMetadata.DirectiveDefinitions[serviceNames[index-1]]
			if !directiveTypeNodesAreEquivalent(definition, previousDefinition) {
				targetDefinition = previousDefinition
				break
			}
		}

		if targetDefinition != nil {
			var lines []string
			for _, serviceName := range serviceNames {
				definition := directiveFederationMetadata.DirectiveDefinitions[serviceName]
				// TODO ここのdefinitionの出力形式はoriginalに比べるとだいぶ貧弱
				lines = append(lines, fmt.Sprintf("\t%s: %s", serviceName, definition.Name))
			}

			gErr := gqlerror.ErrorPosf(
				targetDefinition.Position, // TODO originalの出力とは異なる
				"%s custom directives must be defined identically across all services. See below for a list of current implementations:\n%s",
				logDirective(directive.Name),
				strings.Join(lines, "\n"),
			)
			if gErr.Extensions == nil {
				gErr.Extensions = make(map[string]interface{})
			}
			gErr.Extensions["code"] = "EXECUTABLE_DIRECTIVES_IDENTICAL"
			errors = append(errors, gErr)
		}
	}

	return errors
}

// 1. KEY_MISSING_ON_BASE - Originating types must specify at least 1 @key directive
// 2. MULTIPLE_KEYS_ON_EXTENSION - Extending services may not use more than 1 @key directive
// 3. KEY_NOT_SPECIFIED - Extending services must use a valid @key specified by the originating type
func keysMatchBaseService(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	typeNames := make([]string, 0, len(schema.Types))
	for typeName := range schema.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)
	for _, parentTypeName := range typeNames {
		parentType := schema.Types[parentTypeName]

		// Only object types have fields
		if parentType.Kind != ast.Object {
			continue
		}

		typeFederationMetadata := metadata.FederationTypeMap.Get(parentType)
		serviceName := typeFederationMetadata.ServiceName
		keys := typeFederationMetadata.Keys

		if typeFederationMetadata.ServiceName == "" || len(keys) == 0 {
			continue
		}

		if len(keys[serviceName]) == 0 {
			typeNode := findTypeNodeInServiceList(parentTypeName, serviceName, serviceList)

			gErr := gqlerror.ErrorPosf(
				typeNode.Position,
				"%s appears to be an entity but no @key directives are specified on the originating type.",
				logServiceAndType(serviceName, parentTypeName, ""),
			)
			if gErr.Extensions == nil {
				gErr.Extensions = make(map[string]interface{})
			}
			gErr.Extensions["code"] = "KEY_MISSING_ON_BASE"
			errors = append(errors, gErr)
			continue
		}

		availableKeys := make([]string, 0, len(keys[serviceName]))
		for _, key := range keys[serviceName] {
			availableKeys = append(availableKeys, printSelectionSet(key))
		}

		serviceNames := make([]string, 0, len(keys))
		for serviceName := range keys {
			serviceNames = append(serviceNames, serviceName)
		}
		sort.Strings(serviceNames)
		for _, extendingService := range serviceNames {
			// No need to validate that the owning service matches its specified keys
			if extendingService == serviceName {
				continue
			}

			keyFields := keys[extendingService]

			// Extensions can't specify more than one key
			extendingServiceTypeNode := findTypeNodeInServiceList(parentTypeName, extendingService, serviceList)

			if len(keyFields) > 1 {
				gErr := gqlerror.ErrorPosf(
					extendingServiceTypeNode.Position,
					"%s is extended from service %s but specifies multiple @key directives. Extensions may only specify one @key.",
					logServiceAndType(extendingService, parentTypeName, ""),
					serviceName,
				)
				if gErr.Extensions == nil {
					gErr.Extensions = make(map[string]interface{})
				}
				gErr.Extensions["code"] = "MULTIPLE_KEYS_ON_EXTENSION"
				errors = append(errors, gErr)
				continue
			}

			// This isn't representative of an invalid graph, but it is an existing
			// limitation of the query planner that we want to validate against for now.
			// In the future, `@key`s just need to be "reachable" through a number of
			// services which can link one key to another via "joins".
			extensionKey := printSelectionSet(keyFields[0])
			var foundKeys bool
			for _, availableKey := range availableKeys {
				if availableKey == extensionKey {
					foundKeys = true
					break
				}
			}
			if !foundKeys {
				lines := make([]string, 0, len(availableKeys))
				for _, fieldSet := range availableKeys {
					lines = append(lines, fmt.Sprintf(`@key(fields: "%s")`, fieldSet))
				}

				gErr := gqlerror.ErrorPosf(
					extendingServiceTypeNode.Position,
					"%s extends from %s but specifies an invalid @key directive. Valid @key directives are specified by the originating type. Available @key directives for this type are:\n\t%s",
					logServiceAndType(extendingService, parentTypeName, ""),
					serviceName,
					strings.Join(lines, "\n\t"),
				)
				if gErr.Extensions == nil {
					gErr.Extensions = make(map[string]interface{})
				}
				gErr.Extensions["code"] = "KEY_NOT_SPECIFIED"
				errors = append(errors, gErr)
				continue
			}
		}
	}

	return errors
}
