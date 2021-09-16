package federation

func validateServicesBeforeComposition(services []*ServiceDefinition) []error {
	var warningsOrErrors []error

	for _, serviceDefinition := range services {
		for _, validator := range preCompositionValidators() {
			warningsOrErrors = append(warningsOrErrors, validator(serviceDefinition)...)
		}
	}

	return warningsOrErrors
}
