package planner

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
)

func checkGoldenFile(t *testing.T, actual []byte, expectFilePath string) {
	t.Helper()

	expectFileDir := path.Dir(expectFilePath)

	expect, err := ioutil.ReadFile(expectFilePath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(expectFileDir, 0755)
		if err != nil {
			t.Fatal(err)
		}
		err = ioutil.WriteFile(expectFilePath, actual, 0444)
		if err != nil {
			t.Fatal(err)
		}
		return
	} else if err != nil {
		t.Error(err)
		return
	}

	if string(expect) != string(actual) {
		diff := difflib.UnifiedDiff{
			A:       difflib.SplitLines(string(expect)),
			B:       difflib.SplitLines(string(actual)),
			Context: 5,
		}
		d, err := difflib.GetUnifiedDiffString(diff)
		if err != nil {
			t.Fatal(err)
		}
		t.Error(d)
	}
}
