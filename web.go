// *** Webserver ***
//
// Copyright (c) 2016 Drobo Inc. All rights reserved
//
// Main webserver methods for decryptDiags

// Overall plan
//
// On receipt of diags (as a zip file), we should load the whole zip into memory
// store the zip filename and pointer to memory in memory
// Display the contents of the zip file. Web page link should have zipfile name in it
// Provide links to contents - link should include zipfile name/filename
// Decode (once) when going to subpage - store in memory and display from memory
// Have a page with a list of all cached zip files
// Have ability to save uncompressed file
// Options to display HTML version, etc.

// Encoding scheme
// Currently
// http://host/command/zippath/filename
//
// Proposed change
//
// http://host/command?zipfile=path&filename=file
//
// Alternative 1
// http://host/command?id=num&file=filename (with database of num -> zip file mappings)
//
// Has the advantage that we can keep database info, such as whether the file is decrypted or not, whether it is somewhere
// else (such as JIRA), and perhaps some useful state (such as notes)

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
)

const HTML_TEMPLATES_DIR = "templates"
const HTML_MAIN_INDEX = "main.html"
const HTML_ABOUT_FILE = "about.html"
const HTML_ZIP_FILE = "zip.html"
const HTML_DISPLAY_FILE = "display.html"
const HTML_ASSETS_PATH = "assets/"
const UPLOAD_PATH = "uploads"
const HTML_STYLES_PATH = HTML_ASSETS_PATH + "styles/"

var globalWebMutex sync.Mutex

var uploadDir string

// fileDataOrder implements sort.Interface for []os.FileInfo based on
// the time.time field.
type fileDateOrder []os.FileInfo

// Forward request for length
func (p fileDateOrder) Len() int {
	return len(p)
}

// Define compare - we want out list to be most recent time first
func (p fileDateOrder) Less(i, j int) bool {
	return p[i].ModTime().After(p[j].ModTime())
}

// Define swap over an array
func (p fileDateOrder) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

// Webpage templates
type templateHandler struct {
	once     sync.Once // Only instantiated once
	filename string
	templ    *template.Template
}

// Data structure passed to our HTML template
type webPageInfo struct {
	// Note elements must start with a Captial letter to be accessed from a template
	Body        bytes.Buffer
	Filename    string // filename of a log file within a zip file
	ZipFilepath string // full pathname of zipfile
	ZipFilename string // Filename of zip file (no path)
	Path        string
	Dirlist     fileDateOrder
	Filelist    []string
	Version     string
	UploadDir   string
	JiraCookie  JIRA_LOGIN_STATE
	JiraBugID   string
}

func GetActionAndFilename(r *http.Request) (action string, filename string) {
	if r.URL.Path != "" {
		segs := strings.SplitN(r.URL.Path, "/", 3)
		if len(segs) > 1 {
			action = segs[1]
		}
		if len(segs) > 2 {

			// On Windows, the path will be an absolute pathname, starting with a drive letter.
			// On UNIX based systems (Mac, Linux etc), the path will also be an absolute path, starting with /
			// however, the / has been removed by the split operation. We can't just add it back in, because that
			// makes no sense on Windows
			// We can determine if the file is absolute - if not, we can infer its a UNIX system, and add a leading /
			// alternatively use runtime.GOOS

			filename = filepath.Join(segs[2])

			if !filepath.IsAbs(filename) {
				filename = string(os.PathSeparator) + filename
			}
		}
	}

	log.Println(r.URL.Path, "=", action, filename)

	return action, filename
}

var styleList []string

//var styleList []os.FileInfo

func GetStyleList() error {
	styles, err := ioutil.ReadDir(HTML_STYLES_PATH)
	if err != nil {
		log.Println(err)
		//		io.WriteString(w, err.Error())
		return err
	}

	// ioutil.ReadDir() returned filenames sorted by filename, so no need for further sorting

	for _, name := range styles {
		styleList = append(styleList, strings.TrimSuffix(name.Name(), ".css"))
	}

	//	for _, name := range styleList {
	//		log.Println(name)
	//	}

	return nil
}

// Serve HTML pages

// Method associated with the templateHandler struct
//
// Right now this is going to handle multiple pages. Would be better if we could have specific handlers. Not sure exactly how to do
// that yet and generate unique behaviors

func (t *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Instantiate the templates - this will only be done once
	t.once.Do(func() {
		t.templ = template.Must(template.ParseFiles(filepath.Join(HTML_TEMPLATES_DIR, t.filename)))
	})

	// Work out what page is being handled
	action, filename := GetActionAndFilename(r)

	// Populate variables we want to pass to the webpage
	var webpage webPageInfo
	webpage.Version = versionString
	webpage.JiraCookie = JiraCookie

	switch action {
	case "zip":
		log.Println("zip case")
		//		// A buffer to build up our unique content

		// Open the file file and get the filelist

		// Should we handle already decrypted files somehow? A flag, or just use the _d in the name?

		var err error
		webpage.Filelist, err = decryptZipFilelist(filename)
		if err != nil {
			log.Println("Failed to open zip", filename, err)
			io.WriteString(w, err.Error())
			// return a specific error here
			return
		}
		//		log.Println("zipfile", filename, "contains", webpage.Filelist)

		webpage.Filename = filename

	case "del":
		log.Println("del case")

		// Might want to add a confirmation dialog

		err := os.Remove(filename)
		if err != nil {
			log.Println("Failed to delete zip", filename, err)
			io.WriteString(w, err.Error())
			// return a specific error here
			return
		}
		log.Println("Deleted", filename)

		// Use this form instead? Probably not as we're executing a template later so incompatible?
		//	log.Println("Redirect to", "/")
		//	http.Redirect(w, req, "/", http.StatusFound)

		w.Header()["Location"] = []string{"/"}
		w.WriteHeader(http.StatusTemporaryRedirect)

	case "save":
		log.Println("save case")

		// Open the requested file

		reader, err := os.Open(filename)
		if err != nil {
			log.Println("Failed to open file for saving", filename, err)
			io.WriteString(w, err.Error())
			// return a specific error here
			return
		}

		defer reader.Close()

		// Remove path from filename

		savename := filepath.Base(filename)
		log.Println("saving", filename, "to", savename)

		//copy the relevant headers. Content-Type and Content-Length should be based on the file
		w.Header().Set("Content-Disposition", "attachment; filename="+savename)

		ext := filepath.Ext(filename)
		log.Println("MIME type", mime.TypeByExtension(ext))
		w.Header().Set("Content-Type", mime.TypeByExtension(ext))

		fileinfo, err := os.Stat(filename)
		if err != nil {
			log.Println("Failed to sate file", filename, err)
			// No reason to fail - we'll simply not have a Content-Length
		} else {
			log.Println("filesize", fileinfo.Size())
			w.Header().Set("Content-Length", strconv.FormatInt(fileinfo.Size(), 10))
		}
		//stream the body to the client without fully loading it into memory
		io.Copy(w, reader)

	case "decryptzip":
		log.Println("decryptzip case")

		// Create an ioWriter that we can decrypt diags into and then pass as to the templating code to generate as HTML
		decryptWriter := bufio.NewWriter(&webpage.Body)

		// split filename into zip file name, and file within the zip
		segments := strings.SplitAfter(filename, ".zip")

		//		log.Println("segments", segments)

		webpage.ZipFilepath = segments[0]
		filesplit := strings.Split(segments[0], string(os.PathSeparator))

		webpage.ZipFilename = filesplit[len(filesplit)-1] // Get the zip filename without path
		webpage.Filename = strings.TrimPrefix(segments[1], string(os.PathSeparator))

		// Should we handle already decrypted files somehow? A flag, or just use the _d in the name?
		// Right now decryptZipSpecificFile looks at the filename for _d

		decryptZipSpecificFile(webpage.ZipFilepath, webpage.Filename, decryptWriter)
		//		w.Header().Set("Content-Type", "text/plain")

	case "":
		log.Println("empty case - i.e. /")

		// We should generate a list of previously uploaded diags here and display with links before the upload new diags list

		// TODO: Get the directory list of the uploads directory and supply to template which can range of it like the zip handler

		filelist, err := ioutil.ReadDir(uploadDir)
		if err != nil {
			log.Println(err)
			io.WriteString(w, err.Error())
			return
		}

		webpage.UploadDir = uploadDir

		// sort the list by date so we display the most recently added first.
		webpage.Dirlist = filelist
		sort.Sort(webpage.Dirlist)

	case "delete_all":
		log.Println("delete_all case")

		filelist, err := ioutil.ReadDir(uploadDir)
		if err != nil {
			log.Println(err)
			io.WriteString(w, err.Error())
			return
		}

		// Might want to add a confirmation dialog

		for _, filename := range filelist {

			delName := filepath.Join(uploadDir, filename.Name())

			err := os.Remove(delName)
			if err != nil {
				log.Println("Failed to delete zip", delName, err)
				io.WriteString(w, err.Error())
				// return a specific error here
				return
			}
			log.Println("Deleted", delName)
		}

		// Use this form instead? Probably not as we're executing a template later so incompatible?
		//	log.Println("Redirect to", "/")
		//	http.Redirect(w, req, "/", http.StatusFound)

		w.Header()["Location"] = []string{"/"}
		w.WriteHeader(http.StatusTemporaryRedirect)

	case "oldhandler": // no longer supported, but keeping for reference
		log.Println("default case")

		// TODO: We should generate an error/unknown page here

		// Create an ioWriter that we can decrypt diags into and then pass as to the templating code to generate as HTML

		decryptWriter := bufio.NewWriter(&webpage.Body)

		// create a pipe so we can read out of the io.Writer
		//	decryptReader, decryptWriter := io.Pipe()

		//	log.Println("io pipe created")

		reader, err := os.Open(r.URL.Path)
		if err != nil {
			log.Println(err)
			io.WriteString(w, err.Error())
			//			globalWebMutex.Unlock()
			// In this case we don't display anything. We should display an error page.
			// How do we do that given we've been given a specific template when we were called? Just look up a different template?
			return
		}
		defer reader.Close()

		// Decrypt directly to the http response ioWriter

		decryptFile(reader, decryptWriter)
		// Make sure we close the writer, or the reader will never complete

		//	reader.Close()
		//	decryptWriter.Close()

		//	var decryptReader io.Reader
		//	t.templ.Execute(w, decryptReader)

		webpage.Filename = filename

	case "about":
	// drop through and avoid the default case

	default:
		w.WriteHeader(http.StatusNotFound) // 404
		log.Println("default - no special handling of", action, "on page", r.URL)
		fmt.Fprintf(w, "No such page: %s\n", r.URL)

		// Return here - we don't want to display the regular / page.
		// Really we want to pick up a template for an error handling page

		// We could redirect to one...
		return
		// Nothing special
	}
	t.templ.Execute(w, webpage)

	//	mainWebServerHandler(writer, r)

}

// Handle upload of files
//
// We don't know the path of the file - just its name, so upload it and store it locally before passing it to /decryptzip
// As a first pass, we'll capture the filename and pass that to /decryptzip
// Later we can injest and cache locally (ideally in memory)

func uploaderHandler(w http.ResponseWriter, req *http.Request) {
	file, header, err := req.FormFile("zipFile")
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	log.Println("Upload", header.Filename)

	// Read the file into memory
	data, err := ioutil.ReadAll(file)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	// Write it out somewhere local
	tmpFile, err := ioutil.TempFile(uploadDir, header.Filename)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	defer os.Remove(tmpFile.Name()) // clean up

	log.Println("uploading to", tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		log.Println("Failed to write temporary file for encrypted zip", err)
		return
	}
	if err := tmpFile.Close(); err != nil {
		log.Fatal(err)
	}

	// Now decrypt - with some refactoring, we could probably do the load and decrypt as a single operation

	filename := filepath.Join(uploadDir, header.Filename)
	var decryptFileSplit []string = strings.Split(filename, ".")
	decryptFileSplit[0] += "_d"
	var decryptFilename string = strings.Join(decryptFileSplit, ".")

	log.Println("decrypt to", decryptFilename)
	decryptZip(tmpFile.Name(), decryptFilename)

	// Now redirect to the decryptzip page with the uploaded file

	absPath, err := filepath.Abs(decryptFilename)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	// Work out path to file, and send to the zip handler
	// We do the join this way because we only want the separator between the cwd and the filename to be OS specific

	// Indicate somehow that we've already decrypted this file

	log.Println("Redirect to", "/zip/"+absPath)
	http.Redirect(w, req, "/zip/"+absPath, http.StatusFound)

}

// Main invocatation and configuration of the webserver. Runs as a go routine

func createWebServer() {
	fmt.Println("Webserver start on localhost:" + strconv.Itoa(webServerPort))

	// Ensure we have a location for uploading files. We'll use the current working directory as our base, but
	// TODO: this can be overridden by the command line

	var err error
	uploadDir, err = filepath.Abs(UPLOAD_PATH)
	if err != nil {
		log.Println(err.Error())
		return
	}
	log.Println("UploadDir = ", uploadDir)

	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		log.Println("Creating upload directory", uploadDir)
		err = os.Mkdir(uploadDir, 0777)
		if err != nil {
			log.Println(err.Error())
			return
		}
	}

	// Reference bootstrap assets

	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir(HTML_ASSETS_PATH))))

	//	http.HandleFunc("/", mainWebServerHandler)
	http.Handle("/", &templateHandler{filename: HTML_MAIN_INDEX})
	http.Handle("/del/", &templateHandler{filename: HTML_MAIN_INDEX})
	http.Handle("/save/", &templateHandler{filename: HTML_MAIN_INDEX})
	http.Handle("/delete_all/", &templateHandler{filename: HTML_MAIN_INDEX})
	http.Handle("/about/", &templateHandler{filename: HTML_ABOUT_FILE})
	http.Handle("/zip/", &templateHandler{filename: HTML_ZIP_FILE})
	http.Handle("/decryptzip/", &templateHandler{filename: HTML_DISPLAY_FILE})

	http.HandleFunc("/uploader", uploaderHandler)
	http.HandleFunc("/jiralogin", jiraloginHandler)
	http.HandleFunc("/jira/", jirapostHandler)
	http.HandleFunc("/decryptziphtml/", fileGenerateHtmlMarkup)
	http.HandleFunc("/jiradownload", jiraDownloadHandler)
	http.HandleFunc("/jiraattach/", jiraDownloadAttachment)

	// Cache list of styles
	GetStyleList()

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(webServerPort), nil))
}

// return a web URL relative to where the application is installed, as the upload directory is a subdirectory of that location
func pathToOpen(filename string) string {
	dir, err := os.Getwd()
	if err != nil {
		/// do something
	}
	return "http://localhost:" + strconv.Itoa(webServerPort) + "/zip/" + dir + string(os.PathSeparator) + filename
}

//func mainWebServerHandler(w http.ResponseWriter, r *http.Request) {
//	// For now, ensure serialization of webserver actions

//	globalWebMutex.Lock()
//	fmt.Println("Decrypt", r.URL.Path)

//	w.Header().Set("Content-Type", "text/text")

//	fmt.Fprintf(w, "URL.Path = %q\n", r.URL.Path)

//	// Assume the path is a filesystem path... ideally we'd like to pull the encrypted file/zip directly from
//	// the web interface

//	reader, err := os.Open(r.URL.Path)
//	if err != nil {
//		fmt.Println(err)
//		globalWebMutex.Unlock()
//		return
//	}
//	defer reader.Close()

//	// Decrypt directly to the http response ioWriter
//	decryptFile(reader, w)

//	globalWebMutex.Unlock()
//}
