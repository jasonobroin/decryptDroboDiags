// jira.go
//
// Copyright (c) 2016 Drobo Inc. All rights reserved
//
// Main JIRA integration methods for decryptDiags
//
// Look at moving to a open-source library, like https://github.com/andygrunwald/go-jira
// although that seems to fairly limited, the basic model looks good.
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	Jira "github.com/jasonob/go-jira"
)

const HTML_JIRA_DOWNLOAD_FILE = "jiradownload.html"

type JIRA_TEMPLATE_INFO struct {
	BugId string
	Issue *Jira.Issue
	// These entry are in the general webPageInfo structure in web.go - should we composite?
	Filename    string // filename of a log file within a zip file
	ZipFilepath string // full pathname of zipfile
	ZipFilename string // Filename of zip file (no path)
}

// Store the cookie we get back from logging into Jira - only in memory for now

var JiraCookie JIRA_LOGIN_STATE

func (s JIRA_LOGIN_STATE) GetCookie() string {
	return s.Cookie.Name + "=" + s.Cookie.Value
}

func (s JIRA_LOGIN_STATE) IsCookieValid() bool {
	if s.Cookie.Name != "" && s.Cookie.Value != "" {
		return true
	} else {
		return false
	}
}

func (s JIRA_LOGIN_STATE) GetUsername() string {
	return s.Username
}

// Global JIRA client; stores authentication and session information
var jiraClient *Jira.Client

func uploadFileToJira(bugId string, filename string) error {
	// Add the decrypted zip file
	f, err := os.Open(filename)
	if err != nil {
		log.Println("failed to open file in JIRA upload", err)
		return err
	}
	defer f.Close()

	// Need to ask the user for the bug number to post to

	uploadname := filepath.Base(filename)

	_, resp, err := jiraClient.Issue.PostAttachment(bugId, f, uploadname)
	if err != nil {
		log.Println("UploadFileToJira", resp, err)
		return err
	}
	return nil
}

func uploadToJira(filename string, bugid string, comment string) error {

	// Send it to the web server

	err := uploadFileToJira(bugid, filename)
	if err != nil {
		log.Println("file upload to", bugid, "failed - skip posting comment")
		return err
	}

	_, resp, err := jiraClient.Issue.AddComment(bugid, &Jira.Comment{Body: comment})

	if err != nil {
		log.Println("post comment to", bugid, "failed", resp, err)
		return err
	}

	return nil
}

// Request issue from JIRA.
// Return error and http status if appropriate
// This should return the JSON response
func JiraGetIssue(issue string) (*Jira.Issue, error, int) {
	if jiraClient == nil {
		return nil, fmt.Errorf("Not authenticated"), http.StatusUnauthorized
	}
	resp, _, err := jiraClient.Issue.Get(issue)
	if err != nil {
		log.Println("failed to get JIRA issue", issue, err)
		return nil, err, 0
	}

	for _, a := range resp.Fields.Attachments {
		log.Println("attachment", a.Filename)
	}

	return resp, nil, 0
}

// Handle JIRA login
//
func jiraloginHandler(w http.ResponseWriter, req *http.Request) {
	//	file, header, err := req.FormFile("zipFile")
	//	if err != nil {
	//		io.WriteString(w, err.Error())
	//		return
	//	}
	log.Println("JIRA login")

	// Get the user name and password

	username := req.FormValue("username")
	log.Println("username", username)

	password := req.FormValue("password")
	log.Println("password", password)

	// Login

	var err error
	jiraClient, err = Jira.NewClient(nil, JIRA_BASE_URL)
	if err != nil {
		log.Println("Failed to create JIRA client instance", err)
	}

	res, err := jiraClient.Authentication.AcquireSessionCookie(username, password)
	if err != nil || res == false {
		fmt.Printf("Result: %v\n", res)
	}

	//	JiraCookie.Cookie = sessionResponse.Session
	JiraCookie.Cookie.Name = username
	JiraCookie.Cookie.Value = "placeholder"
	JiraCookie.Username = username

	log.Println("Redirect to", "/")
	http.Redirect(w, req, "/", http.StatusFound)

}

// Handle JIRA login
//
// Upload file to specified JIRA bug, and also post comment
//
// Comment is updated to provide a JIRA cross-link to the file that has been uploaded
//
func jirapostHandler(w http.ResponseWriter, r *http.Request) {
	//	file, header, err := req.FormFile("zipFile")
	//	if err != nil {
	//		io.WriteString(w, err.Error())
	//		return
	//	}
	log.Println("JIRA post")

	filename := r.FormValue("filename")
	log.Println("filename", filename)

	// On Windows, the path will be an absolute pathname, starting with a drive letter.
	// On UNIX based systems (Mac, Linux etc), the path will also be an absolute path, starting with /
	// however, the / has been removed by the split operation. We can't just add it back in, because that
	// makes no sense on Windows
	// We can determine if the file is absolute - if not, we can infer its a UNIX system, and add a leading /
	// alternatively use runtime.GOOS

	if !filepath.IsAbs(filename) {
		filename = string(os.PathSeparator) + filename
	}

	bugid := r.FormValue("bugid")
	log.Println("bugid", bugid)

	// Add a link to the file we're uploading to the comment
	uploadname := filepath.Base(filename)
	comment := r.FormValue("comment") + "\n\n" + "Uploaded diags file: [^" + uploadname + "]"
	log.Println("comment", comment)

	// We could fail if we're not logged in (don't have a cookie)

	err := uploadToJira(filename, bugid, comment)
	if err != nil {
		log.Println("Jira upload of ", filename, "failed")
	}

	log.Println("Redirect to", "/")
	http.Redirect(w, r, "/", http.StatusFound)

}

// Method to get files associated with a JIRA issue
//func JiraGetIssueFiles(issue string) (error, int) {
// Use JiraGetIssue, and pull out the attachment fields
//
// Method to download a specific file associated with a JIRA issue
//func downloadFoJira(filename string, bugid string) error {

// List attachments associated with a particular JIRA Bug
//
func jiraDownloadHandler(w http.ResponseWriter, req *http.Request) {
	//	file, header, err := req.FormFile("zipFile")
	//	if err != nil {
	//		io.WriteString(w, err.Error())
	//		return
	//	}
	log.Println("JIRA Download Handler")

	var templateInfo JIRA_TEMPLATE_INFO

	// Get the user name and password

	templateInfo.BugId = req.FormValue("BugId")
	log.Println("bugId", templateInfo.BugId)

	issue, err, val := JiraGetIssue(templateInfo.BugId)
	if err != nil {
		log.Println("Failed to get JIRA issue", err, val)
	} else {
		templateInfo.Issue = issue
	}

	var output = template.Must(template.ParseFiles(filepath.Join(HTML_TEMPLATES_DIR, HTML_JIRA_DOWNLOAD_FILE)))

	if err := output.Execute(w, templateInfo); err != nil {
		fmt.Println("template generation failed", err)
	}
}

func GetAttachmentIdAndFilename(r *http.Request) (attachmentId string, filename string) {
	if r.URL.Path != "" {
		segs := strings.SplitN(r.URL.Path, "/", 4)
		if len(segs) > 2 {
			attachmentId = segs[2]
		}
		if len(segs) > 3 {

			// On Windows, the path will be an absolute pathname, starting with a drive letter.
			// On UNIX based systems (Mac, Linux etc), the path will also be an absolute path, starting with /
			// however, the / has been removed by the split operation. We can't just add it back in, because that
			// makes no sense on Windows
			// We can determine if the file is absolute - if not, we can infer its a UNIX system, and add a leading /
			// alternatively use runtime.GOOS

			filename = filepath.Join(segs[3])

			if !filepath.IsAbs(filename) {
				filename = string(os.PathSeparator) + filename
			}
		}
	}

	log.Println(r.URL.Path, "=", attachmentId, filename)

	return attachmentId, filename
}

// Download a specified attachment for an JIRA Bug
//
func jiraDownloadAttachment(w http.ResponseWriter, req *http.Request) {

	// Work out what file to download and its name
	attachmentId, filename := GetAttachmentIdAndFilename(req)

	res, err := jiraClient.Issue.DownloadAttachment(attachmentId)
	if err != nil {
		log.Println("failed to download JIRA attachment", attachmentId, err)
		return
	}
	defer res.Body.Close()

	file := filepath.Join(uploadDir, filename)
	log.Println("download attachment to", file)

	fw, err := os.Create(file)
	if err != nil {
		panic(err)
	}
	defer fw.Close()

	// Copy the file
	if _, err = io.Copy(fw, res.Body); err != nil {
		log.Println("failed to copy file in JIRA upload", err)
	}

	//	fw.Close()

	log.Println("Redirect to", "/")
	http.Redirect(w, req, "/", http.StatusFound)
}
