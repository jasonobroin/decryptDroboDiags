// analyzer_test.go
package main

import (
	"os"
	"testing"
)

const TEST_FILE_DIR = "test/DroboDiag__DRB125101A00192_20160331_163359_d/"
const LOCKED_DIAGS_FILE = "vxLockedDiags.txt"

func TestAnalyzer(t *testing.T) {

	reader, err := os.Open(TEST_FILE_DIR + "/" + LOCKED_DIAGS_FILE)
	if err != nil {
		t.Fatal("TestAnalyzer", err)
		return
	}
	defer reader.Close()
	//	_ = fileGenerateHtmlMarkup(reader)

}
