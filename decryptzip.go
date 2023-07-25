// DecryptZip.go
//
// Copyright (c) 2016 Drobo Inc. All rights reserved
//
// Process a whole zip file. Many of the files within the zip are not encrypted and don't need uncompressing/decoding/compressing, in
// which case they are simply copied from one zip to the other.
//
// 1. Process each file within the zip in turn.
//    a. Identify each file in turn, either based on file type, or by looking for a header at the start of the file
//    b. It would be nice to have an index file (possibly JSON) describing files within the zip and their encoding mechanism to
//       allow the whole process to be better automated

package main

import (
	"archive/zip"
	"decryptDiags/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type Flags uint

// Additional action could be to create a CSV file
const (
	FlagCopy Flags = 1 << iota
	FlagDecrypt
	FlagDecode
)

type FileHandlingTable struct {
	searchKey string
	flags     Flags
}

var handlingTable []FileHandlingTable

// The search string is a prefix search only
func init() {
	handlingTable = []FileHandlingTable{
		{"VX", FlagDecrypt},
		{"LXDMESG", FlagDecrypt},
		{"DROBODIAG_", FlagDecrypt},
		{"EVENTLOG", FlagDecode},
		{"DISKLOG", FlagDecode},
    {"FLASHLOG", FlagDecode},
		{"PERFLOG", FlagDecode | FlagCopy},
		{"ZONETABLE", FlagDecode | FlagCopy},
	}
}

func decryptZipFilelist(filename string) ([]string, error) {

	// Open a zip archive for reading.
	r, err := zip.OpenReader(filename)
	if err != nil {
		//		log.Fatal(err)
		return nil, err
	}
	defer r.Close()

	var zipContent []string

	// Iterate through the files in the archive, generating a list of names
	for _, f := range r.File {
		zipContent = append(zipContent, f.Name)
	}
	//	log.Println(zipContent)

	return zipContent, nil
}

// decryptZipSpecificFile
//
// decrypt a specific file within a zipfile to an io.Writer
//
// The core behavior acts on a file; a binary file is decoded; an encrypted file is decrypted.
// It is not possible to chain actions (e.g decrypt then decode)
func decryptZipSpecificFile(zipFilename string, filename string, writer io.Writer) error {
	// Open a zip archive for reading.
	log.Println("decryptZipSpecificFilename", zipFilename, filename)
	r, err := zip.OpenReader(zipFilename)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer r.Close()

	// Work out if zipfile name ends with _d
	var decryptFileSplit []string = strings.Split(zipFilename, ".")
	var skipDecode = strings.HasSuffix(decryptFileSplit[0], "_d")

	// Iterate through the files in the archive, decrypting the ones we need to, and copying the others to the new archive
	// printing some of their contents.
	for _, f := range r.File {
		if f.Name == filename {
			reader, err := f.Open()
			if err != nil {
				log.Fatal(err)
			}
			defer reader.Close()

			// Decode some of the files in the zip - many are in plaintext

			switch {
			case skipDecode:
				// Copy unchanged to the io.Writer
				fmt.Printf("copying: ")

				_, err = io.Copy(writer, reader)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("complete\n")

			case strings.HasPrefix(strings.ToUpper(f.Name), "VX"),
				strings.HasPrefix(strings.ToUpper(f.Name), "LXDMESG"),
				strings.HasPrefix(strings.ToUpper(f.Name), "DROBODIAG_"):
				// Any file name starting with vx or Vx should be decrypted
				fmt.Printf("decrypting: ")
				// Decrypt file and output into io.Writer

				decryptFile(reader, writer)

				fmt.Printf("complete\n")
			case strings.HasPrefix(strings.ToUpper(f.Name), "EVENTLOG"),
				strings.HasPrefix(strings.ToUpper(f.Name), "DISKLOG"),
        strings.HasPrefix(strings.ToUpper(f.Name), "FLASHLOG"),
				strings.HasPrefix(strings.ToUpper(f.Name), "PERFLOG"),
				strings.HasPrefix(strings.ToUpper(f.Name), "ZONETABLE"):
				// Decode binary files
				fmt.Printf("decoding: ")

				binary.DecodeFile(reader, writer)

				fmt.Printf("complete\n")
			default:
				// Copy unchanged to the io.Writer
				fmt.Printf("copying: ")

				_, err = io.Copy(writer, reader)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("complete\n")
			}
		}
	}
	return nil
}

//decryptZip
//
// Decrypt a whole zipfile to a new zipfile
// Multiple rules can be applied to process each file in the zip, such as decrypting, decoding and copying
// Note that currently actions can't be changed. i.e. you can't decrypt then decode
func decryptZip(filename string, decryptFilename string) {

	//	copyFile(filename, filename+"d")

	// Open a zip archive for reading.
	r, err := zip.OpenReader(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	// Open another one for writing

	zipfile, err := os.Create(decryptFilename)
	if err != nil {
		return // err
	}
	defer zipfile.Close()
	fmt.Println("Decrypting to", decryptFilename)

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	fmt.Println("Files in ", filename)
	// Iterate through the files in the archive, decrypting the ones we need to, and copying the others to the new archive
	// printing some of their contents.
	for _, f := range r.File {
		fmt.Printf("%s: ", f.Name)

		reader, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		defer reader.Close()

		// Open new file inside archive

		// Do all the fun zip header stuff - use the header from the source file

		header := f.FileHeader

		// Lookup file in our file  handling table and work out what to do with it

		found := false
		for _, entry := range handlingTable {
			if strings.HasPrefix(strings.ToUpper(f.Name), entry.searchKey) {

				found = true

				// We've found a match. Work out what ways we need to process it

				if entry.flags&FlagDecrypt == FlagDecrypt {
					// Decrypt file and output into new zip

					// May need to adjust name if we decrypt it

					writer, err := archive.CreateHeader(&header)
					if err != nil {
						fmt.Println("Error (decrypt) creating archive header ", header.Name, err)
						return // err
					}

					// Decrypt file and output into io.Writer
					fmt.Println("copying to", header.Name)
					decryptFile(reader, writer)
				}
				if entry.flags&FlagDecode == FlagDecode {
					// Decode binary files

					// Adjust the name to change or add a .log suffix
					var decodeFileSplit []string = strings.Split(header.Name, ".")
					decodeHeader := header
					decodeHeader.Name = decodeFileSplit[0] + ".txt"

					// May need to adjust name if we decode it

					writer, err := archive.CreateHeader(&decodeHeader)
					if err != nil {
						fmt.Println("Error (decode) creating archive header", decodeHeader.Name, err)
						return // err
					}

					fmt.Println("decoding to", decodeHeader.Name)
					binary.DecodeFile(reader, writer)
				}
				if entry.flags&FlagCopy == FlagCopy {
					// Copy unchanged to the decrypted archive file
					fmt.Println("copying to", header.Name)

					writer, err := archive.CreateHeader(&header)
					if err != nil {
						fmt.Println("Error (copy) creating archive header", header.Name, err)
						return // err
					}
					//        defer writer.Close()

					_, err = io.Copy(writer, reader)
					if err != nil {
						log.Fatal(err)
					}
					fmt.Printf("complete\n")
				}

				break
			}
		}

		if !found {
			// Copy unchanged to the decrypted archive file
			fmt.Printf("copying: ")

			writer, err := archive.CreateHeader(&header)
			if err != nil {
				fmt.Println("Error (copy2) creating archive header", header.Name, err)
				return // err
			}
			//        defer writer.Close()

			_, err = io.Copy(writer, reader)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("complete\n")
		}
	}

	archive.Close()
	fmt.Println("Decryptzip complete")
}
