package federation

func preCompositionValidators() []func(*ServiceDefinition) []error {
	return []func(definition *ServiceDefinition) []error{
		// TODO let's implements below rules!
		// externalUsedOnBase,
		// requiresUsedOnBase,
		// keyFieldsMissingExternal,
		// reservedFieldUsed,
		// duplicateEnumOrScalar,
		// duplicateEnumValue,
	}
}
