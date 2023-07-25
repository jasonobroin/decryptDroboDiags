// jira_struct.go
package main

const JIRA_BASE_URL = "http://jira/jira/"
const JIRA_REST_AUTH_URL_BASE = "http://jira/jira/rest/auth/1/"
const JIRA_REST_URL_BASE = "http://jira/jira/rest/api/2/"
const JIRA_REST_SESSION = "session"
const JIRA_REST_ISSUE = "issue"
const JIRA_REST_ATTACHMENTS = "attachments"
const JIRA_REST_COMMENT = "comment"
const JIRA_REST_FIELD_ATTACHMENT = "fields=attachment"
const JIRA_GET_ATTACHMENT = "http://jira/jira/secure/attachment/"

// Structures which will be turned into JSON request/responses

type JIRA_SESSION struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Internal state to track authenticated user

type JIRA_LOGIN_STATE struct {
	Cookie   JIRA_SESSION
	Username string
}
