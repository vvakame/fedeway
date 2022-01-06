package federation

import (
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func postCompositionValidators() []func(*ast.Schema, *FederationMetadata, []*ServiceDefinition) []error {
	return []func(schema *ast.Schema, metadata *FederationMetadata, serviceList []*ServiceDefinition) []error{
		// TODO let's implements below rules!
		externalUnused,
		// externalMissingOnBase,
		// externalTypeMismatch,
		// requiresFieldsMissingExternal,
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
