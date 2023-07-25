// jira_test
package main

import (
	"net/http"
	"testing"
)

const ISSUE_TO_GET = "INF-871"
const FILE_TO_UPLOAD = "./jira_test.go"

//func TestUpload1(t *testing.T) {
//	err := uploadToJira(FILE_TO_UPLOAD)

//	if err != nil {
//		t.Error("failed to upload", FILE_TO_UPLOAD, err)
//	}

//}

// Assume this runs before we authenticated successfully

func TestGetIssueUnauthorized(t *testing.T) {
	_, err, statusCode := JiraGetIssue(ISSUE_TO_GET)

	if statusCode != http.StatusUnauthorized {
		t.Error("Failed unauthorized access test", err, statusCode)
	}

}

// Need to authenticate to run additional tests

// Test the JiraGetIssue function with no field filter
// Assume this runs after we authenticated successfully
func xTestGetIssue(t *testing.T) {
	_, err, statusCode := JiraGetIssue(ISSUE_TO_GET)

	if err != nil {
		t.Error("failed to get issue", err, statusCode)
	}

}

// Test upload
// Assume this runs after we authenticated successfully
func xTestUpload(t *testing.T) {
	err := uploadToJira(FILE_TO_UPLOAD, ISSUE_TO_GET, "jira_test.go")

	if err != nil {
		t.Error("failed to upload", FILE_TO_UPLOAD, err)
	}

}
