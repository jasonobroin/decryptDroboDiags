// DecryptDiags.go
//
// Copyright (c) 2016 Drobo Inc. All rights reserved
//
// Decrypt diags from a zip file
//
// This utility will decrypt diags from a zip file, into a new zip file.
//
// It aims to be faster than previous solutions, by using a few techniques.
//
// a) copy the original zip file, and decrypt required files in place (many files inside a diags zip are not encrypted
// and currently get copied in/out of a zip incurring a decompression/compression cycle.
// NOT DONE: The zip library doesn't obviously allow modification to existing zip files
//
// b) Decrypt diags in paralllel using go's concurrency mechanisms
//
// c) Have improved handling for corrupted diags. These are currently detected by looking for the top bit of a character being
// set as this indicates a non-printable character. The old algorithm can be confused by genuine instances of top-bit usage
// (for example, memory corruption in the logs as a result of failed battery backup) which don't reflect an error in diags
// encryption, and a highly aggressive form of resyncing to find sections of the log which isn't corrupted.
//
// Incidences of genuine diag corruption appear to be rare these days, so we shouldn't solve for this in the general case
// (perhaps add a special option to do aggressive diag recovery); most recent cases of apparent diag corruption are top-bits
// set in crash diags which are valid, if unhelpful.
//
// In addition, access to decrypted diags is blocked by efforts to repair corrupted diags which blocks analysis. Running
// decrypts in parallel and copying the diags zip to a new 'decrypted zip' ahead of time should allow access to other decrypted
// files as soon as they have been created.
//
// d) Have better understanding of files that don't need decrypting - encode these into the utility so they can be skipped
// immediately.
//
// e) Have a framework to support handling of binary files - it would be very convienient to allow upload of binary data and
// have the decrypt tool generate analytical content based on this data. In some cases this may simply be generating text based
// analysis of the binary file (so the binary file has simply reduced the diag upload time and data quantity by offloading to
// the decrypt tool - for example, uploading a copy of the zone table, or uploading performance graph binary data), but in
// other cases we may want to build some ancillery capabilities into the core decrypt engine to generate analytical info based on
// further qualifies (for example, generate performance graphs based on time ranges, or sort data on different keys)
//
// Further improvements
//
// Additional ideas are
//
// 1. Provide a web interface for uploading diags and generating decrypted diags, possibly running on a central server (possibly a Drobo)
//
// 2. Allow automatic upload of decrypted diags to a JIRA ticket
//
// 3. Allow access to previously decrypted diags
//
// 4. Allow additional graphical analysis of diags, such a performance graphs
//
// 5. Allow comparision between diags
//
// 6. Interface with other analytical tools, such as gdb for core dump analysis
//
// 7. Allow access to diags on something like an S3 server automatically uploaded from customer systems
//
// 8. Generate summary diags of key system state/configuration to assist support levels with quick understanding and analysis of key
// aspects of a customer system. For example, Product type, serial number, types of disks, capacity, disk errors, protection type,
// load levels, performance, possible red-flag issues etc.
//
// 9. Some level of integration into support/registration systems
//
// 10. Diag output for easy navigation - previously we've provided web links to different parts of the output. Can this be better.
// Are there different organizations and cross-linking that would provide more insights/analytical info.
// Similarly, allowing better organization access to live vs crash related info could help
//
// 11. Improved searchability - better ways of cross referencing data from different domains (e.g. Vx, Lx and host). Overlay diag outputs on
// common timeline with different colours for different domains, ability to switch on/off domains and/or roll up sections not interested in
// for current analysis. Increase or decrease log level output shown or focus in on particularly traces?
//
//
// Limitations - this utility will NOT decrypt old diags from Drobo Gen1, Gen2, Pro and FS
//
// OTHER NOTES
//
// 1. There might be a bug in the encryption process on the Drobo. We're seeing a regular corruption at offset 32727, and need to attempt to
//    run the seed rotation/xor processor 32727 to get back in sync... that sounds like we've failed to move the encryption seed on at some
//    point. IIRC the upload buffer is 32K, and the corruption point is very close to 32K (its 0x7FD7, so 0x29 - 41 bytes off. Suspicious)
//    I note further that we do appear to have lost part of a line, quite possibly 41 bytes. An additional note is that we only get one
//    instance of corruption in a file of ~ 750KB. Perhaps only the first upload buffer is having an issue (because it has a header?) and we're
//    reseting the seed (once?) on the subsequent buffers?
// 	  Another observation. This only happens with the lockeddiags file, and not with other files uploaded via the encryption mechanism
//

// NOTE: Decode speed is ~ 4 times speed of old decryptzip/decryptdiag, although probably due to inefficient uncompress/compress of non-encrypted files

// ISSUES

// 1. Files with large amounts of "corruption" seem to stall - they are doing the same resync that the current code does (which at least says what's going on).
//    This algorithm requires review. In many cases these aren't actually corrupted or incorrectly generated diag files, they just reflect non-printable
//    characters in the source. Perhaps we should assume this, and provide agressive recovery as an selectable option.
//    FIXED [heroic recovery not implemented)
//

package main

import (
	"decryptDiags/binary"
	_ "decryptDiags/binary/eventlog"
	_ "decryptDiags/binary/perfLog"
	_ "decryptDiags/binary/zoneTable"

	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/toqueteos/webbrowser"
)

// ------ Flags definitions --------

const shorthand = " (shorthand)"

// filename variable & flag decoder
var filename string

// Tie the command-line flag to the filename variable and set usage info
// We can have multiple init() functions which are called before main()
// ... we could also combine these init functions (with change of const name)

func init() {
	const (
		defaultFilename = ""
		usage           = "An encrypted diag filename. This is an individual file, not a zip file"
	)
	flag.StringVar(&filename, "f", defaultFilename, usage+shorthand)
	flag.StringVar(&filename, "filename", defaultFilename, usage)
}

var zipFilename string

// Tie the command-line flag to the zipFilename variable and set usage info
func init() {
	const (
		defaultFilename = ""
		usage           = "An encrypted zip filename."
	)
	flag.StringVar(&zipFilename, "z", defaultFilename, usage+shorthand)
	flag.StringVar(&zipFilename, "zipFilename", defaultFilename, usage)
}

var dataFilename string

// Tie the command-line flag to the dataFilename variable and set usage info
func init() {
	const (
		defaultFilename = ""
		usage           = "A data file filename."
	)
	flag.StringVar(&dataFilename, "d", defaultFilename, usage+shorthand)
	flag.StringVar(&dataFilename, "dataFilename", defaultFilename, usage)
}

var enableWebServer bool
var webServerPort int

func init() {
	const (
		defaultWebServerPort = 8000
		usage                = "Start a decryptDiags web server"
		usageWS              = "Port to use for the webserver"
	)

	//	func IntVar(p *int, name string, value int, usage string)
	//	func BoolVar(p *bool, name string, value bool, usage string)

	flag.BoolVar(&enableWebServer, "w", false, usage+shorthand)
	flag.BoolVar(&enableWebServer, "web", false, usage)
	flag.IntVar(&webServerPort, "wp", defaultWebServerPort, usageWS+shorthand)
	flag.IntVar(&webServerPort, "webport", defaultWebServerPort, usageWS)

}

// return a web URL where the filename is an absolute path
// if its not an absolute a path, add the CWD to the start
func absPathToOpen(filename string) string {

	if !filepath.IsAbs(filename) {
		dir, err := os.Getwd()
		if err != nil {
			/// do something
		}
		filename = dir + string(os.PathSeparator) + filename
	}

	return "http://localhost:" + strconv.Itoa(webServerPort) + "/zip/" + filename
}

func main() {

	// Print the command line
	fmt.Print("DecryptDiags " + versionString)

	for index, value := range os.Args {
		if index != 0 {
			fmt.Print(" ", value)
		}
	}

	fmt.Println()

	// Handle flags

	// Define the flags

	flag.Parse()

	fmt.Println("remainder of command line : ", flag.Args())

	// Web support
	//
	// Add new flag -web to generate a web server.
	// If given a filename or zipfile, it processes that file immediately and displays it, otherwise it waits for a drag of a file into the web server. How to do that?
	// We basically need to upload it locally, and then just process it in-situ, possibly with a save decrypted version option
	//
	// With a regular file, just decrypt and run decrypt to the web server as it meets io.Writer interface
	// With a zip file, display a list of the contents of the file. Clicking on a link will run decrypt to the web server as it meets io.Writer interface
	//
	// Channels will likely be the correct way to handle all of these

	// Web server warning - multiple http requests can be processes in parallel as separate go routines, so we need to use concurrency protection

	// If we've not been given a zip or file, see if there's any unconsumed arguments.
	// If .zip, treat as a zip, otherwise treat as a file
	// Note we could range across all arguments and process them as files to decrypt

	if filename == "" && zipFilename == "" && dataFilename == "" && len(flag.Args()) != 0 {
		if strings.HasSuffix(flag.Args()[0], ".zip") {
			zipFilename = flag.Args()[0]
		} else if strings.HasSuffix(flag.Args()[0], ".dat") {
			dataFilename = flag.Args()[0]
		} else {
			filename = flag.Args()[0]
		}
	}

	fmt.Println("Decrypting file", filename)
	fmt.Println("Decrypting zip", zipFilename)
	fmt.Println("Decoding datafile", dataFilename)

	var path string

	switch {
	case filename != "":
		var decryptFileSplit []string = strings.Split(filename, ".")
		decryptFileSplit[0] += "_d"
		var decryptFilename string = strings.Join(decryptFileSplit, ".")
		decryptDiagFile(filename, decryptFilename)
		// path = absPathToOpen(decryptFilename)
	case dataFilename != "":
		var decodeFileSplit []string = strings.Split(dataFilename, ".")
		decodeFileSplit[0] += "_txt"
		var decodeFilename string = strings.Join(decodeFileSplit, ".")
		binary.DecodeDataFile(dataFilename, decodeFilename)
		//		path = absPathToOpen(decodeFilename)
	case zipFilename != "":
		var decryptFileSplit []string = strings.Split(zipFilename, ".")
		decryptFileSplit[0] += "_d"
		var decryptFilename string = strings.Join(decryptFileSplit, ".")
		decryptZip(zipFilename, decryptFilename)
		// path is purely for use to automatically open a webpage
		path = absPathToOpen(decryptFilename)
	}

	if enableWebServer {
		// This won't return. For now, we can either process a file via the command line or start the webserver
		go createWebServer()

		if path != "" {
			// This library ends up executing a command, effectively terminating the process I think... need to change it to fork off a process for that purpose?
			webbrowser.Open(path)
			log.Println("webbrowser open")
		}

		// wait forever
		select {}
	}

}
