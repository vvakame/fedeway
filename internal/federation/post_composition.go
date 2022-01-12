package federation

import (
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func postCompositionValidators() []func(*ast.Schema, *FederationMetadata, []*ServiceDefinition) []error {
	return []func(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error{
		// TODO let's implements below rules!
		externalUnused,
		externalMissingOnBase,
		externalTypeMismatch,
		requiresFieldsMissingExternal,
		// requiresFieldsMissingOnBase,
		// keyFieldsMissingOnBase,
		// keyFieldsSelectInvalidType,
		// providesFieldsMissingExternal,
		// providesFieldsSelectInvalidType,
		// providesNotOnEntity,
		// executableDirectivesInAllServices,
		// executableDirectivesIdentical,
		// keysMatchBaseService,
	}
}

// for every @external field, there should be a @requires, @key, or @provides
// directive that uses it
func externalUnused(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error {
	var errors []error

	for parentTypeName, parentType := range schema.Types {
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
		for serviceName, externalFieldsForService := range typeFederationMetadata.Externals {
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

	for typeName, namedType := range schema.Types {
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
		for serviceName, externalFieldsForService := range typeFederationMetadata.Externals {
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

	for typeName, namedType := range schema.Types {
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
		for serviceName, externalFieldsForService := range typeFederationMetadata.Externals {
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

	for typeName, namedType := range schema.Types {
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
