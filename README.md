DecryptDiags 6.3.2

Redeveloped Drobo diag decrypt utility written in go

# Binaries

Pre-built binaries are checked into the repro

- Mac version: decryptDiags
- Windows version; decryptDiags.exe
- Linux version: decryptDiags-lx

# Simplest Usage

- decryptDiags -w -wp <port=8000>
- Browse to http://localhost:8000
- Add diags
- Browse diags

# Build Instructions

- go compiler needs to be downloaded from https://golang.org/dl/
- Code was originally developed with go version 1.6; most recently built with 1.20
- 'go build' will build the binary.
- Both Mac and Windows versions built and tested. No known OS incompatibilities
- buildall.sh will build Mac (decryptDiags), Windows 32 bit (decryptDiags.exe) and Linux x86 (decryptDiags-lx)

# Web Interface Support/Dependencies

- All dependencies currently kept locally
- bootstrap: v3.3.6
- jquery: v1.11.3
- highlight.js : v9.5.0, build with node.js v4.4.7 and npm 2.15.8

- Changes to highlight.js needs built with node.js

## Added files

- extra/highlight.js/src/languages/drobo.js
- extra/highlight.js/test/detect/drobo/default.txt

# Build steps

- git clone https://github.com/isagalaev/highlight.js
- cd highlight.js
- cp -r <decryptDiagsLocation>/extra/highlight.js/ .
- npm install
- node tools/build.js xml json drobo
- build/highlight.pack.js has required javascript code - copy to assets/js/

# Development

- Use 'go fmt' to keep code in correct go code format

# Instructions

- Will decrypt individual files or zip files
- decryptDiags [-f <filename> | -z <zip filename> -d <dataFilename>] <filename>
- If no command line option chosen, decryptDiags will look at the supplied filename suffix to work out what to do
- Generates a <filename>_d or <zip_filename>._d.zip file containing decrypted diags 

# Deployment

- Create a shortcut on Desktop to simplify decrypt process
- Add -w to the shortcut will automatically open web browser with the list of the contents of the decrypted diags

# Docker Deployment

- docker run -d --name dd -P decryptdiags

# Web Server

- decryptDiags -w -wp <port=8000>
- Browse to http://localhost:8000
- Need to copy templates and assets directory to same location as decryptDiags in order to provide access to HTML pages
- Upload either encrypted or previously decrypted zip files. Both are handled
- Web server allows JIRA login, and post of diags (with comment) to a JIRA bug [NO LONGER WORKS AS API CHANGED]
- Web server allows viewing of the decrypted diags files as plain textfile, or indexed based on sub-sections

# Limitations

- Only supports v2 diags (i.e. 5N, 5D(t), 5C, Gen3, B810n, B810i, B1200i)


# Version Info

6.3.2

* Fix ZoneTable decode to use the correct stripe width to calculate number of regions to display for the striped zone types

6.3.1

* Fix -w option to correctly open decrypted zip file passed via the command line

6.3.0

* Allow multiple actions when importing a zip file, such as decoding and copying the file. Binary decoded files are now
  given a .txt action, which JIRA handles well, and for the ZoneTable and Perflog, the original binary file is kept which
  would allow future processing on the binary data (for example, different display modes)
* Perflog decoding needs to cope with different word sizes on ARM and MIPS systems
* Recognize FLASHLOG as a binary file

6.2.7

* Output time as "UTC", which reports the corect time as it actually is in PDT... needs more work

6.2.6

* Add decoder for perflog; add section analysis for perflog

6.2.5

* New model for uploading event logs - reduced header; stream of event logs, which shouldn't include any null entries. Also includes pre-log entries

6.2.4

* Binary data header format is now in network byte order
* EventLog and ZoneTable decoders use the endianness field when decoding their data structures
* Bitflip the ZoneFlag bitfield when binary file is from a big endian system

6.2.3 

* Hook Eventlog and ZoneTable binary decoders into the zip file handling code

6.2.2

* Allow selection of which highlighting class is used for each different sub-section of diags
* Some initial handling for binary file decoding
* Decoder for eventlog and zonetable

6.2.1

* Initial addition of code highlighting for XML & JSON; always on
* Add ability to select code highlighting style
* Made top navigation bar fixed
* buildall reduces file size by stripping debug symbols
* Initial experiments with adding Drobo specific highlighting

6.1.14

* Improved indexing of LxDmesgiSCSId diags
* HTML escape indexed tags
* Allow individual diags sections to be open/closed when in indexing mode
* Allow all diag sections to be opened/closed
* Next/Prev links replaced with icons
* Change Windows build to generate a 32-bit binary

6.1.13

* Fix toggling so we don't lose a line of output on each section

6.1.12

* Add ability to toggle diag markup/indexing on/off

6.1.11

* Use JIRA access API library from github.com and refactor code to use that
* Add previous/next links on marked up diag display

6.1.10

* If -z and -w are used together, browser automatically opens to the decrypted diag contents page. Only works with zip files, not individual files
* Added ability to download a file from JIRA
* Re-org of main page to have file upload on the top navigation bar
* Dockerfile added
* Fixed about page handling
* Copied jquery.min.js locally and removed external links to .js and .css pages
* Some tidy up on HTML pages
* Added a buildall.sh script to build Mac, Windows & Linux executables

6.1.9

* Ensure that marked up text is displayed with HTML filter, so embedded XML docs are displayed correctly
* Correct search key for /.ash_history

6.1.8

* Transform search strings into user friendly index items

6.1.7

* Add indents levels to index

6.1.6

* Add first pass at parsing diags to generate HTML indexed version of files. These are generated on demand
* Table layout for files within a zip file, plus add link to generate the HTML indexed version

6.1.5

* Fix mechanism used to do ask backend to upload to JIRA to do POST correctly
* Display alert when uploading to show in progress, and hide on completion

6.1.4

* Listen on all IP addresses, not just localhost
* Added action to download decrypted diags from web interface to filing system
* Added ability to login to JIRA
* Added ability to post to a JIRA bug and add a comment
* Some code refactoring

6.1.3

* Sort upload file list by date (most recent first)
* Improved table for list of decrypt file
* Remove encrypted file after upload
* Close file correctly after uploading

6.1.2

* Very basic 404 (Not Found) page
* Table format for list of zip files
* Decrypt diag file on upload (both encrypted and decrypted versions added to uploads directory)
* Remove html filter when displaying decrypted diags - speeds things up, and not really needed

6.1.1

* Added ability to delete a zip file, and delete all zip files
* Fix some web page redirection issues

6.1.0

* Refactor code into multiple source files
* First pass on webserver model
* Attempt to decrypt DroboDiag_* files inside the zip (old naming model)
* Add bootstrap theme to webserver
* Ability to upload encrypted files to webserver, display and decode them, and work with previously uploaded diags

6.0.1

* Determine whether file is zip or not based on suffix if -z or -f options not supplied
* Fix handling of corrupted characters if we can't resync - return to next XOR seed in sequence

6.0.0

* First redeveloped version
