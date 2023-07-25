// convert.go
// Convert a data file into a binary package with header, because on command line flags
package main

import (
	binDecode "decryptDiags/binary"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

const shorthand = " (shorthand)"

var dataFilename string
var binaryType uint

// Tie the command-line flag to the dataFilename variable and set usage info
func init() {
	const (
		defaultFilename = ""
		usage           = "A data file filename."
	)
	flag.StringVar(&dataFilename, "d", defaultFilename, usage+shorthand)
	flag.StringVar(&dataFilename, "dataFilename", defaultFilename, usage)
}

func init() {
	const (
		defaultType = 0
		usage       = "The binary type"
	)
	flag.UintVar(&binaryType, "b", defaultType, usage+shorthand)
	flag.UintVar(&binaryType, "binaryType", defaultType, usage)

}

func convertDataFile(filename string, convertFilename string) {
	reader, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer reader.Close()

	writer, err := os.Create(convertFilename)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer writer.Close()
	fmt.Println("Convert to", convertFilename)

	convertFile(reader, writer)
}

// This function should take parameters from the command line to setup the binaryHdr
//
// convertFile reads the whole file into memory, and writes out a binaryHdr followed by the original file
func convertFile(reader io.Reader, writer io.Writer) {

	var binHdr binDecode.BinaryHdr

	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		fmt.Println(err)
		return
	}

	binHdr.HeaderVersion = 0xdeadbeef
	binHdr.DiagBinaryType = uint32(binaryType)
	binHdr.ImageSize = uint32(binary.Size(bs))

	// Write out header

	err = binary.Write(writer, binary.LittleEndian, &binHdr)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = writer.Write(bs)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func main() {
	// Define the flags

	flag.Parse()

	var convertFileSplit []string = strings.Split(dataFilename, ".")
	convertFileSplit[0] += ".bin"
	var convertFilename string = strings.Join(convertFileSplit, ".")

	fmt.Println("Convert", dataFilename, "to", convertFilename, "with binaryType", binaryType)

	convertDataFile(dataFilename, convertFilename)
	//		path = pathToOpen(decodeFilename)

}
