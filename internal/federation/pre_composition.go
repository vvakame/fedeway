package federation

func preCompositionValidators() []func(*ServiceDefinition) []error {
	return []func(definition *ServiceDefinition) []error{
		// TODO 実装すること
		// externalUsedOnBase,
		// requiresUsedOnBase,
		// keyFieldsMissingExternal,
		// reservedFieldUsed,
		// duplicateEnumOrScalar,
		// duplicateEnumValue,
	}
}
