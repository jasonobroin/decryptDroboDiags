// *** decrypt.go ***
//
// Copyright (c) 2016 Drobo Inc. All rights reserved
//

// Methods for decrypting an encrypted diags file

// Decrypt works by first identifying the appropriate key to use. It does this by decrypting the
// whole file and then scanning for an expected string. If it doesn't fine this string the program
// does not decrypt and exits. If the string is found, the key is set and the data is unencrypted back
// to original content.

// Once the proper key is set the program loops through the file a second time only decrypting the
// sections with a non-ascii value. The keys are such that these sections are guaranteed to be encrypted.
// If there is a section of characters that are ascii, the programm skips these sections.

package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

// ------- Decryption Algorithms -------

// Encryption scheme identification strings
const (
	decryptedString   = "Diags decrypted using DecryptDiags"
	unencryptedString = "DataRobotics Unencrypted Data File Format"
	v1encryptedString = "DataRobotics Encrypted Data File Format: v1"
	v2encryptedString = "DataRobotics Encrypted Data File Format: v2"
)

// Encryption scheme identifier

type EncryptionScheme int

const (
	Decrypted EncryptionScheme = iota
	Unencrypted
	v1Encrypted
	v2Encrypted
	UnknownEncryption
)

// checkHeader
//
// Check for a valid Drobo diags header indicating whether file has already been decrypted, or which encryption
// mechanism has been used.
//
// This function will only look at the start of the file, although previous decryption code had the ability to
// search forward in the file to find the header.
//
// The function could also take the filename and use that to decide whether to skip encyption (for example, host
// log files) although doesn't today.
//
// A file without an encryption header is assumed to be v2 encryption
//
// returns: offset into the file after encryption header, encryption type, error value (nil == success)

func checkHeader(bs []byte) (int, EncryptionScheme, error) {
	checkLen := len(v2encryptedString)
	checkStr := string(bs[0:checkLen])

	// Current implementation only supports v2 header

	if strings.HasPrefix(checkStr, v2encryptedString) {
		fmt.Println("v2 string found")
		// Add 1 to offset to account for newline at end of header string
		return checkLen + 1, v2Encrypted, nil
	}

	return 0, Unencrypted, nil
}

const v2Seed uint32 = 0x137b12a4

const ERROR_INDICATOR byte = 0x18 // CANcel

// Generate random stream - pass by value so we update the seed as we go

func RAND32(seed *uint32) uint32 {
	const mult uint32 = 1664525
	const add uint32 = 1013904223
	*seed = mult**seed + add
	return *seed
}

func RotateLeft(value byte, bits uint8) byte {
	bits %= 8

	if bits == 0 {
		return value
	} else {
		result := (value << bits) | (value >> (8 - bits))
		return result
	}
}

// Many corruptions require 32726 skips to resync (connected to block size?)
const maxRecoverySteps int = 32726 * 10
const maxRecoveryAttempt int = 20

func decryptV2(bs []byte, offset int) (int, int, error) {
	currentSeed := v2Seed
	decryptLen := len(bs)
	potentialCorruption := 0
	failedRecovery := 0
	attemptRecovery := true

	// Decrypt data in-place

	for cursor := offset; cursor < decryptLen; cursor++ {
		// Reverse the encryption XOR/ROR.
		var xorVal uint8 = uint8((RAND32(&currentSeed) & 0xff000000) >> 24)
		var rotVal uint8 = uint8((RAND32(&currentSeed) & 0xff000000) >> 24)

		var decryptByte byte = RotateLeft(bs[cursor], rotVal) ^ xorVal

		if decryptByte&0x80 == 0x80 && attemptRecovery {
			// 			fmt.Println("potential corruption at offset", cursor, "value =", uint8(bs[cursor]), "encrypt value =", uint8(tmpbyte), "rotVal =", rotVal, "xorVal=", xorVal, "&0x7f = ", string(uint8(bs[cursor]) & 0x7f))

			// Skip forward and see if we can re-sync the decrypt
			var corruptOffset = cursor
			var correct bool = false

			oldSeed := currentSeed
			testSeed := v2Seed
			var badCount = 0
			var testDecrypt [32]byte
			var skipped = 0

			// Try to resync. We do this by walking the decrypt seeed forward one step,
			// saving that, and then doing a test decrypt of 32 characters. If there are no errors,
			// we've resynced, otherwise, we keep walking forward until we find success or give up

			for attemptRecovery && !correct && corruptOffset < decryptLen {
				// Reset seed to initial seed, and do one operation to reset
				currentSeed = testSeed
				var _ = uint8((RAND32(&currentSeed) & 0xff000000) >> 24)
				var _ = uint8((RAND32(&currentSeed) & 0xff000000) >> 24)
				testSeed = currentSeed
				skipped++

				badCount = 0
				for test := 0; test < 32; test++ {
					if corruptOffset+test >= decryptLen {
						fmt.Println("Trying to decode beyond end of diag buffer (", corruptOffset, test, ") ... break out")
						potentialCorruption++
						break
					}
					var xorVal uint8 = uint8((RAND32(&currentSeed) & 0xff000000) >> 24)
					var rotVal uint8 = uint8((RAND32(&currentSeed) & 0xff000000) >> 24)

					testDecrypt[test] = RotateLeft(bs[corruptOffset+test], rotVal) ^ xorVal

					if testDecrypt[test]&0x80 == 0x80 {
						badCount++
					}

					if badCount >= 1 {
						break
					}
				}

				// If we've found a good run, store the good character in memory and carry on as normal
				if badCount < 1 {

					fmt.Println("Successful resync after", skipped, "resyncs at offset", corruptOffset)

					// Even though we've found a good enough run of characters, the first character might still be corrupted
					if testDecrypt[0]&0x80 == 0x80 {
						bs[cursor] = ERROR_INDICATOR
					} else {
						bs[cursor] = testDecrypt[0]
					}

					currentSeed = testSeed
					var _ = uint8((RAND32(&currentSeed) & 0xff000000) >> 24)
					var _ = uint8((RAND32(&currentSeed) & 0xff000000) >> 24)

					correct = true
					potentialCorruption++
				}

				// Bail out if we're failing to sync for too long

				if skipped > maxRecoverySteps {
					fmt.Println("Failed to resync after", maxRecoverySteps, "resyncs at offset", corruptOffset)
					// If we've had too many sections we can't recover from, turn off recovery unless we're in heroic recovery mode
					failedRecovery++

					// We weren't able to recover, so reset the seed to the previous value on the assumption we've not lost decrypt sync
					currentSeed = oldSeed

					if failedRecovery == maxRecoveryAttempt {
						fmt.Println("Disable corruption recovery after", failedRecovery, "bad characters")
						attemptRecovery = false
					}
					break
				}
			}

			if !correct {
				bs[cursor] = ERROR_INDICATOR
				potentialCorruption++
			}

		} else {
			// We might have skipped recovery attempts - so double check if a printable character
			if decryptByte&0x80 == 0x80 {
				bs[cursor] = ERROR_INDICATOR
				potentialCorruption++
			} else {
				bs[cursor] = decryptByte
			}
		}

	}

	fmt.Println("Decrypted diags from offset", offset, "to", decryptLen, "with", potentialCorruption, "corrupted bytes")

	return offset, decryptLen, nil
}

func writeDecrypted(bs []byte, writer io.Writer, header bool) error {

	// Write the decrypted file to outfile

	// Created a buffered writer based on our io.writer so we can write strings and byte slices
	bwriter := bufio.NewWriter(writer)

	if header {
		// Write the header first
		_, err2 := bwriter.WriteString(decryptedString + versionString + "\n")
		if err2 != nil {
			fmt.Println("Failed to output decrypt header", err2)
			// return a specific error here
			return nil
		}
	}

	// Use a slice to write the decrypted buffer (is this comment correct?)

	_, err3 := bwriter.Write(bs)
	if err3 != nil {
		fmt.Println("Failed to output decrypted data", err3)
		// return a specific error here
		return nil
	}

	bwriter.Flush()

	return nil
}

func decryptFile(reader io.Reader, writer io.Writer) {
	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		fmt.Println(err)
		return
	}

	offset, encryptType, err := checkHeader(bs)

	if err != nil {
		// switch on error
		// Report if encryption type unsupported or not found and exit
		fmt.Println("Unsupported or non existed encryption type", err)
		return
	}

	fmt.Println("offset = ", offset, " encryptType = ", encryptType)

	switch encryptType {
	case v2Encrypted:
		decryptOffset, decryptLen, err := decryptV2(bs, offset)

		if err != nil {
			fmt.Println("decryption failed", err)
			return
		}

		err2 := writeDecrypted(bs[decryptOffset:decryptLen], writer, true)
		if err2 != nil {
			fmt.Println("Failed to write decrypted diags", err2)
		}

	default:
		// Nothing to decrypt - simply output the original file without a header
		err2 := writeDecrypted(bs, writer, false)
		if err2 != nil {
			fmt.Println("Failed to write unencrypted diags", err2)
		}
	}
}

func decryptDiagFile(filename string, decryptFilename string) {
	reader, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer reader.Close()

	writer, err := os.Create(decryptFilename)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer writer.Close()
	fmt.Println("Decrypting to", decryptFilename)

	decryptFile(reader, writer)
}

func copyFile(src string, dest string) {
	// open files r and w
	r, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	w, err := os.Create(dest)
	if err != nil {
		panic(err)
	}
	defer w.Close()

	// do the actual work
	n, err := io.Copy(w, r)
	if err != nil {
		panic(err)
	}
	log.Printf("Copied %v bytes\n", n)
}
