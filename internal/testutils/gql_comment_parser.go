package testutils

import (
	"fmt"
	"regexp"
)

func FindSchemaFileName(t TestingT, source string) string {
	t.Helper()

	re, err := regexp.Compile("(?m)^# schema:\\s*([^\\s]+)$")
	if err != nil {
		t.Fatal(err)
	}

	ss := re.FindStringSubmatch(source)
	if len(ss) != 2 {
		t.Fatal("schema file directive mismatch")
	}

	return ss[1]
}

func FindOptionString(t TestingT, optionName, source string) string {
	t.Helper()

	pattern := fmt.Sprintf("(?m)^# option:%s:\\s*([^\\s]+)$", optionName)
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatal(err)
	}

	ss := re.FindStringSubmatch(source)
	if len(ss) != 2 {
		t.Logf("option %s value is not found", optionName)
		return ""
	}

	return ss[1]
}

func FindOptionBool(t TestingT, optionName, source string) bool {
	t.Helper()

	pattern := fmt.Sprintf("(?m)^# option:%s:\\s*([^\\s]+)$", optionName)
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatal(err)
	}

	ss := re.FindStringSubmatch(source)
	if len(ss) != 2 {
		t.Logf("option %s value is not found", optionName)
		return false
	}

	return ss[1] == "true"
}
