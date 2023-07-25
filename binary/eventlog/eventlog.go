// eventlog.go
//
// Copyright (c) 2016 Drobo Inc. All rights reserved
//
// Methods for decoding binary eventlog files from Drobo diagnostics
//
package eventlog

import (
	"bytes"
	binDecode "decryptDiags/binary"
	"encoding/binary"
	"fmt"

	"io"
	"time"
)

type eventLogVT struct {
	VTableHdr uint32 // Ignore
}

// This structure is only in the on-disk based EventLog, and not in the flash log
type eventLogHdr struct {
	NumEntries      uint32
	UnsafeBootCount uint32                            // number of unsafe boot counts - used to enable bootloop protection mode
	SoftwareVersion [ELM_SOFTWARE_VER_STRING_LEN]byte // Software version pack was created at
	PackVer         uint32                            // Pack version at creation time (i.e. ESA_LAYOUT_VERSION)
}

// Field size and versions constants
const (
	MAX_EL_STR                  = 120
	ELM_SOFTWARE_VER_STRING_LEN = 60
	EVENT_LOG_ENTRIES           = 1600 * 5
	EVENT_CACHE_ENTRIES         = 1600 * 5
	EVENT_FLASH_ENTRIES         = 1600 * 5
	EVENT_LOG_VERSION           = 0x0003 // only supported version

	PACK_STREAM_BITS = 16
	PACK_VER_MASK    = ((1 << PACK_STREAM_BITS) - 1)
)

type eventLogRecord struct {
	Timestamp uint32
	MessageID uint32           // 8 bits severity; 8 bits category; 16 bits template ID
	EventText [MAX_EL_STR]byte // was char
}

type FlashLogDecoder struct{}

var flashLogDecoder FlashLogDecoder

// Register decoder function
func init() {
	binDecode.RegisterDecoder(binDecode.BinaryFile_FlashEventLog, &flashLogDecoder)
	binDecode.RegisterDecoder(binDecode.BinaryFile_DiskEventLog, &flashLogDecoder)
	binDecode.RegisterDecoder(binDecode.BinaryFile_CachedEventLog, &flashLogDecoder)
}

func (flashlog *FlashLogDecoder) DumpRecords(rec eventLogRecord, w io.Writer) {
	l := bytes.IndexByte(rec.EventText[:], 0) // find the EOL
	if l < 0 {
		l = MAX_EL_STR
	}
	t := time.Unix(int64(rec.Timestamp), 0)
	if l > 0 {
		io.WriteString(w, t.UTC().Format(time.UnixDate)+":"+string(rec.EventText[:l])+"\n")
	}

}

func (flashlog *FlashLogDecoder) Decoder(b binDecode.BinaryHdr, w io.Writer, r io.Reader) error {

	var el eventLogHdr
	var byteOrder binary.ByteOrder
	byteOrder = binary.LittleEndian
	if b.Endianness != 0 {
		byteOrder = binary.BigEndian
	}
	err := binary.Read(r, byteOrder, &el)
	if err != nil {
		fmt.Println("loader hdr: ", err)
		return err
	}

	// Decode header

	l := bytes.IndexByte(el.SoftwareVersion[:], 0) // find the EOL
	if l < 0 {
		l = ELM_SOFTWARE_VER_STRING_LEN
	}

	fmt.Fprintln(w, "EventLog CREATED with s/w version :", string(el.SoftwareVersion[:l]),
		"with disk pack version :", (el.PackVer >> PACK_STREAM_BITS), "/", (el.PackVer & PACK_VER_MASK))

	fmt.Fprintln(w, "Unsafe bootcount :", el.UnsafeBootCount)
	io.WriteString(w, "\n")

	// Dump records from read offset, and wrap back to beginning

	var rec eventLogRecord
	count := 0
	for true {
		err := binary.Read(r, byteOrder, &rec)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			fmt.Println("end of records ", err, count)
			return err
		}

		flashlog.DumpRecords(rec, w)
		count++
	}
	return nil
}
