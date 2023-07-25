// analyzer.go
//
// Analyze diag files after they have been decrypted
//
// General notes
// =============
//
// Different types of files will have different analyzers. Many of the text files will have a straight forward string analyzer which will look for
// certain demarcation strings so that an indexed HTML output can be generated with links to the various subsections; other binary files will be
// parsed or decoded with custom logic that understands their data structure
//
// The top level approach is to map filename (or possibly regex of filename) to a particular parser. As an initial pass, its expected there will
// be no more than one parser per file; the output will generate an ioWriter, so the parsed file can be displayed directly to a web page, or to a file.
// Its undecided whether the file will be run through the template mechanism here or later (or not at all)
//
// HTML link parser
// ================
//
// The HTML link parser will assume that all demarcation strings are at the start of a line, and that there are a small set of valid unique demarcation
// strings. The set of strings will be different for different type of files. This makes reading each line of the file and testing against each valid
// demarcation string a viable approach.
//
// Note: We can improve demaracation analysis by being able to filter out timestamps
//
// An HTML index with links to the various demarcated subsections will be built up, with links back to the top at each demarcation point.
// Some pretty segmentation of the output will be added.
//
// The demarcation string will need to be understood sufficiently to make a readable index, e.g.
//
//  ******** Diags for the CAT Manager
//
// will need to be able to generate a Diags for the CAT Manager link
//
// Some files, such as crash logs may have multiple 'domains' embeded in them, including Vx & Lx diags, which may make parsing tricky or more expensive.
//
// This parser mechanism is intended for diags which are naturally segmented into different subsections, such as lockeddiags
//
// Output stream file parser
// =====================
//
// Some diags represent contiguous output, rather than segmented sections - for example, live log output, nasd logs, Dashboard output, and DroboApps log files.
// In a few cases there may be some segmentation that can be discovered with the HTML (segmented) link parser.
//
// Alternative ideas here are to color code key words (for example, using the XML file L2 support developed), or having some ability to sub-focus on particularly
// sections of the trace. For example, thread IDs could be used to show/collapse particular sections of the trace
//
// Text Highlighting
// =================
//
// Highlighter.js (https://highlightjs.org/) is used to highlight strings in each section of the diags.
// A (number of) Drobo specific highlighter classes have been developed for use with different sub sections of the diags.
//
// By default, highlighter.js will parse the section and work out which highlighter to use. This can be overriden in
// the LOOKUP_ELEMENT definitions.
//
// To turn off highlighting completely, use "nohighlight", although this prevents the currently selected style from being applied.
//
// Currently supported: xml, json, drobo, nohighlight
//
// Binary file parser
// ==================
//
// The intent here is to allow binary files to be uploaded into diags and parsed by modules that understand the binary format. For example, the event log
// could move to this model. Additional examples include uploading the zone table, uploading performance data, and parsing core files.
//
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

const LINKED_TEMPLATE = "linked.html"

type TRANSFORM struct {
	Method       func(input string, trans TRANSFORM, templateInfo ANALYZED_TEMPLATE_INFO, diagLine int) (output string)
	RegexSearch  string
	RegexReplace string
}

type LOOKUP_ELEMENT struct {
	searchString string
	IndentLevel  int
	// Transform function
	Transform TRANSFORM
	// Specific highlighting class to use; if empty, use default
	Highlighter string
}

type PARSED_ELEMENT struct {
	LineNum       int
	SearchElement int // element in the search array
	IndentLevel   int
	Anchor        string
	AnchorText    string
	Previous      int // Previous line number
	Next          int // Next line number
}

var VX_LOCKED_DIAGS_SEARCH_KEYS []LOOKUP_ELEMENT
var VX_LX_CRASH_LOG_SEARCH_KEYS []LOOKUP_ELEMENT
var VX_LIVE_LOG_SEARCH_KEYS []LOOKUP_ELEMENT
var LX_LOG_ROTATED_SEARCH_KEYS []LOOKUP_ELEMENT
var LX_ISCSI_DIAGS_SEARCH_KEYS []LOOKUP_ELEMENT
var LX_SYSTEMINFO []LOOKUP_ELEMENT
var VX_PERFLOG []LOOKUP_ELEMENT
var BASE_SEARCH_KEYS []LOOKUP_ELEMENT

func init() {

	DiagHandlerTransform := TRANSFORM{
		TransformRegex,
		"(Invoking DiagnosticHandler function for )([[:word:]]*) ([[:print:]]*)",
		"${2} Diagnostics",
	}

	SectionTransform := TRANSFORM{
		TransformRegex,
		"([[:punct:]]* )([[:word:][:space:]]*)( [[:punct:]]*)",
		"${2}",
	}

	SectionWithPathTransform := TRANSFORM{
		TransformRegex,
		"([[:punct:]]* )([[:punct:][:word:][:space:]]*)( [[:punct:]]*)",
		"${2}",
	}

	iSCSIDiagnosticsTransform := TRANSFORM{
		TransformRegex,
		"([[:punct:]]* )(Diagnostics : )([[:word:][:space:]]*)( [[:punct:]]*)",
		"${3} Diagnostics",
	}

	NullTransform := TRANSFORM{
		modifyNull,
		"",
		"",
	}

	AMITTransform := TRANSFORM{
		TransformReplace,
		"",
		"AMIT Memory Test Results",
	}

	KernelInitTransform := TRANSFORM{
		TransformReplace,
		"",
		"Kernel Initialized",
	}

	NextLineTransform := TRANSFORM{
		ReturnNextLine,
		"([[:print:]]*)",
		"Crash ${1}",
	}

	VX_LOCKED_DIAGS_SEARCH_KEYS = []LOOKUP_ELEMENT{
		{"Invoking DiagnosticHandler function for", 2, DiagHandlerTransform, ""},
		{"-------------------- LOCKED DIAGS -----------------------", 1, SectionTransform, ""},
		{"----------------------- EVENT LOG -----------------------", 2, SectionTransform, "nohighlight"},
		{"--------------------- DISK EVENT LOG --------------------", 2, SectionTransform, ""},
		{"-------------------- KERNEL DIAGS -----------------------", 1, SectionTransform, ""},
		{"Contents of", 2, NullTransform, ""},
	}

	VX_LX_CRASH_LOG_SEARCH_KEYS = []LOOKUP_ELEMENT{
		{"-------------------- CRASH LOG FLASH FILE START --------------------", 1, NextLineTransform, ""},
		{"KERNEL FULLY INITIALIZED", 2, KernelInitTransform, ""},
		{"Vx Kernel (A)utomated (M)emory (I)ntegrity (T)est ...", 2, AMITTransform, ""},
		{"--- Diagnostics", 2, iSCSIDiagnosticsTransform, ""},
		{"--- iSCSI Target Log File", 1, SectionTransform, ""},
		{"Invoking DiagnosticHandler function for", 3, DiagHandlerTransform, ""},
		{"-------------------- LOCKED DIAGS -----------------------", 1, SectionTransform, ""},
		{"----------------------- EVENT LOG -----------------------", 2, SectionTransform, ""},
		{"--------------------- DISK EVENT LOG --------------------", 2, SectionTransform, ""},
		{"-------------------- KERNEL DIAGS -----------------------", 1, SectionTransform, ""},
		{"Contents of", 2, NullTransform, ""},
		{"Assertion failed", 2, NullTransform, ""},
		{"---------------- LX CRASH LOG FILE START : (copy of previous boot log)  -------------------", 2, SectionTransform, ""},
		{"<!----- Log starts -------!>", 3, SectionTransform, ""},
	}

	VX_LIVE_LOG_SEARCH_KEYS = []LOOKUP_ELEMENT{
		{"========== LIVE CONSOLE OUTPUT START =======", 1, SectionTransform, ""},
		{"KERNEL FULLY INITIALIZED", 2, KernelInitTransform, ""},
		{"Vx Kernel (A)utomated (M)emory (I)ntegrity (T)est ...", 2, AMITTransform, ""},
	}

	LX_LOG_ROTATED_SEARCH_KEYS = []LOOKUP_ELEMENT{
		{"### ", 2, SectionWithPathTransform, ""},
	}

	LX_ISCSI_DIAGS_SEARCH_KEYS = []LOOKUP_ELEMENT{
		{"/bin", 2, NullTransform, ""},
		{"/sbin", 2, NullTransform, ""},
		{"/var", 2, NullTransform, ""},
		{"/tmp", 2, NullTransform, ""},
		{"/etc", 2, NullTransform, ""},
		{"<!----- Log starts -------!>", 1, SectionTransform, ""},
		{"--- Diagnostics", 2, iSCSIDiagnosticsTransform, ""},
		{"--- iSCSI Target Log File", 1, SectionTransform, ""},
	}

	LX_SYSTEMINFO = []LOOKUP_ELEMENT{
		{"/bin", 2, NullTransform, ""},
		{"/sbin", 2, NullTransform, ""},
		{"/var", 2, NullTransform, ""},
		{"/tmp", 2, NullTransform, ""},
		{"/etc", 2, NullTransform, ""},
		{"/mnt", 2, NullTransform, ""},
		{"/.ash_history", 2, NullTransform, ""},
	}

	VX_PERFLOG = []LOOKUP_ELEMENT{
		{"Statistic", 2, NullTransform, ""},
	}

}

// IDEA: Change the DiagLines into a structure, which contains strings, where we can indicate if a line is an anchor; then we can
// range over all DiagLines in the template, and know when we need to insert anchors.
// One question is how we generate array of strings into a new data structure?

type DIAG_LINE struct {
	DiagLine     string
	AnchorNeeded bool
}

type ANALYZED_TEMPLATE_INFO struct {
	decryptedFile bytes.Buffer // Not strictly needed here, but keep it all together
	FoundKeys     []PARSED_ELEMENT
	SearchKeys    []LOOKUP_ELEMENT
	// DiagsLines & AnchorNeeded are the same size, and refer to the same diag line; ideally they would be a common structure
	DiagLines    []string
	AnchorNeeded []*PARSED_ELEMENT
	// These entry are in the general webPageInfo structure in web.go - should we composite?
	Filename    string   // filename of a log file within a zip file
	ZipFilepath string   // full pathname of zipfile
	ZipFilename string   // Filename of zip file (no path)
	StyleList   []string // List of styles
}

// Search string transformation functions
//
// These functions convert particular search strings into more appropriate output for an index table

// Null transformation func - just return input
func modifyNull(input string, trans TRANSFORM, templateInfo ANALYZED_TEMPLATE_INFO, diagLine int) string {
	return input
}

// Return the next line
func ReturnNextLine(input string, trans TRANSFORM, templateInfo ANALYZED_TEMPLATE_INFO, diagLine int) string {
	return TransformRegex(templateInfo.DiagLines[diagLine+1], trans, templateInfo, diagLine)
}

// This transform simply replaces input text with a fixed output
func TransformReplace(input string, trans TRANSFORM, templateInfo ANALYZED_TEMPLATE_INFO, diagLine int) string {
	return trans.RegexReplace
}

// Apply a Regex search/replace transform to the input string
func TransformRegex(input string, trans TRANSFORM, templateInfo ANALYZED_TEMPLATE_INFO, diagLine int) string {
	var comp = regexp.MustCompile(trans.RegexSearch)
	output := comp.ReplaceAllString(input, trans.RegexReplace)
	//	log.Println(output)
	return output

}

// Work out which set of search strings to use for a particular file
func whichSearchStringSet(filename string) (searchKeys []LOOKUP_ELEMENT) {

	log.Println("Search keys filename", filename)
	switch {
	case strings.HasPrefix(strings.ToUpper(filename), "VXLOCKEDDIAGS"):
		log.Println("Search keys: VXLOCKEDDIAGS")
		return VX_LOCKED_DIAGS_SEARCH_KEYS

	case strings.HasPrefix(strings.ToUpper(filename), "VXLXCLOG"):
		log.Println("Search keys: VXLXCLOG")
		return VX_LX_CRASH_LOG_SEARCH_KEYS

	case strings.HasPrefix(strings.ToUpper(filename), "VXLIVELOG"):
		log.Println("Search keys: VXLIVELOG")
		return VX_LIVE_LOG_SEARCH_KEYS

	case strings.HasPrefix(strings.ToUpper(filename), "DAPPS"):
		log.Println("Search keys: DAPPS")
		return LX_LOG_ROTATED_SEARCH_KEYS

		// Catch any specific case that hasn't already been handled
	case strings.HasSuffix(strings.ToUpper(filename), ".LOG"):
		log.Println("Search keys: .log")
		return LX_LOG_ROTATED_SEARCH_KEYS

	case strings.HasPrefix(strings.ToUpper(filename), "LXDMESGISCSI"):
		log.Println("Search keys: LxDmesgISCSI")
		return LX_ISCSI_DIAGS_SEARCH_KEYS

	case strings.HasPrefix(strings.ToUpper(filename), "LXSYSTEMINFO"):
		log.Println("Search keys: LxSystemInfo")
		return LX_SYSTEMINFO

	case strings.HasPrefix(strings.ToUpper(filename), "PERFLOG"):
		log.Println("Search keys: Perflog")
		return VX_PERFLOG
	}

	// We should return some sensible default, or error; a null array ought to be valid

	log.Println("Search keys: NO filename match")
	return BASE_SEARCH_KEYS
}

// Process a text file, looking for matches in the array of search strings; generate an HTML marked up version with an index to the found search strings
//
// A basic assumption is that the search strings will always be found at the start of a line of text
//
// Search strings might have a regex character, to allow the links to be more descriptive
//
// Should this integrate with a template? Or multiple templates. Perhaps we have a template for the index, and a template for each sub-section of diags
func fileGenerateHtmlMarkup(w http.ResponseWriter, req *http.Request) {

	//func fileGenerateHtmlMarkup(r io.Reader) []string {
	//func fileGenerateHtmlMarkup(r io.Reader /* io.Writer, search []string */) [][]byte {

	var templateInfo ANALYZED_TEMPLATE_INFO

	_, filename := GetActionAndFilename(req)

	// Create an ioWriter that we can decrypt diags into and then pass into the templating code to generate as HTML
	decryptWriter := bufio.NewWriter(&templateInfo.decryptedFile)

	// split filename into zip file name, and file within the zip
	segments := strings.SplitAfter(filename, ".zip")

	//		log.Println("segments", segments)

	templateInfo.ZipFilepath = segments[0]
	filesplit := strings.Split(segments[0], string(os.PathSeparator))

	templateInfo.ZipFilename = filesplit[len(filesplit)-1] // Get the zip filename without path
	templateInfo.Filename = strings.TrimPrefix(segments[1], string(os.PathSeparator))

	// Should we handle already decrypted files somehow? A flag, or just use the _d in the name?
	// Right now decryptZipSpecificFile looks at the filename for _d

	decryptZipSpecificFile(templateInfo.ZipFilepath, templateInfo.Filename, decryptWriter)

	templateInfo.DiagLines = strings.Split(string(templateInfo.decryptedFile.Bytes()), "\n")

	log.Println("number of lines in file", len(templateInfo.DiagLines))

	templateInfo.SearchKeys = whichSearchStringSet(templateInfo.Filename)

	// Check each line against each entry in the search string, starting at the beginning of each line

	var found *PARSED_ELEMENT = nil
	var previous *PARSED_ELEMENT = nil
	for line, n := range templateInfo.DiagLines {
		found = nil
		for searchElement, searchkey := range templateInfo.SearchKeys {
			if strings.HasPrefix(n, searchkey.searchString) {
				parseElement := PARSED_ELEMENT{line, searchElement, templateInfo.SearchKeys[searchElement].IndentLevel, strconv.Itoa(line),
					templateInfo.SearchKeys[searchElement].Transform.Method(n, templateInfo.SearchKeys[searchElement].Transform, templateInfo, line),
					0, 0}
				templateInfo.FoundKeys = append(templateInfo.FoundKeys, parseElement)
				found = &parseElement

				if previous != nil {
					previous.Next = line
					parseElement.Previous = previous.LineNum
				}

				previous = &parseElement
				break
			}
		}
		templateInfo.AnchorNeeded = append(templateInfo.AnchorNeeded, found)
	}

	// Dump the list of found elements

	//	for _, element := range templateInfo.FoundKeys {
	//		fmt.Println("line", element.LineNum, "type", element.searchElement, "indent", templateInfo.SearchKeys[element.SearchElement].IndentLevel, ":", templateInfo.DiagLines[element.LineNum])
	//	}

	// Generate a list of offsets into the slice of found search strings
	// Generate output : HTML index, and HTML marked up contents

	var output = template.Must(template.ParseFiles(filepath.Join(HTML_TEMPLATES_DIR, LINKED_TEMPLATE)))

	templateInfo.StyleList = styleList

	if err := output.Execute(w, templateInfo); err != nil {
		fmt.Println("template generation failed", err)
	}
}
