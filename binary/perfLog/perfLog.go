// perflog.go
//
// Copyright (c) 2016 Drobo Inc. All rights reserved
//
// Methods for decoding binary zone table files from Drobo diagnostics
//
package perflog

import (
	"bytes"
	binDecode "decryptDiags/binary"
	"encoding/binary"
	"fmt"

	"io"
	"time"
)

const (
	NUM_LOG_ENTRIES = 900
	NAME_LEN        = 128
	// Might make these runtime controllable
	ENTRIES_PER_LINE = 5
)

type PerfLogEntry struct {
	Name         [NAME_LEN]byte
	Desc         [NAME_LEN]byte
	LogEntrySize uint32
	LogBytes     uint32
	Log          [NUM_LOG_ENTRIES]uint64
}

// ARM and MIPS have different sizes for long, and the TimeTns/Ts is essentially a uint64, so has different orderings
type EntryTimeARM struct {
	FastTicksVal uint32
	TimeTns      uint32
	TimeTs       uint32
}

type EntryTimeMIPS struct {
	FastTicksVal uint64
	TimeTs       uint32
	TimeTns      uint32
}

// Number of entries in each log entry
type PerfLogHeaderARM struct {
	Name          [NAME_LEN]byte
	PauseReason   uint64
	RecordEntries int32
	NextLogIndex  int32
	EntryTimes    [NUM_LOG_ENTRIES]EntryTimeARM
}

// We'll use this as the core version
type PerfLogHeaderMIPS struct {
	Name          [NAME_LEN]byte
	PauseReason   uint64
	RecordEntries int32
	NextLogIndex  int32
	EntryTimes    [NUM_LOG_ENTRIES]EntryTimeMIPS
}

func (hdr *PerfLogHeaderMIPS) convertArmHdrToMips(armHdr *PerfLogHeaderARM) {
	for i := 0; i < NUM_LOG_ENTRIES; i++ {
		hdr.EntryTimes[i].FastTicksVal = uint64(armHdr.EntryTimes[i].FastTicksVal)
		hdr.EntryTimes[i].TimeTs = armHdr.EntryTimes[i].TimeTs
		hdr.EntryTimes[i].TimeTns = armHdr.EntryTimes[i].TimeTns
	}
}

func ByteToString(b []byte, max int) string {
	l := bytes.IndexByte(b, 0) // find the EOL
	if l < 0 {
		l = max
	}

	return string(b[:l])
}

type PerfLogDecoder struct{}

var perfLogDecoder PerfLogDecoder

// Register decoder function
func init() {
	binDecode.RegisterDecoder(binDecode.BinaryFile_PerfLog, &perfLogDecoder)
}

// We find the first non 0 timestamp for each record, but the current upload mechanism means this is common
// for all logs, so we could pass that information in
// Alternatively, we could upload the timestamp table for each record as they are uploaded at different times
// and the upload could straddle a number of seconds
func (perflog *PerfLogDecoder) DumpRecord(hdr PerfLogHeaderMIPS, ple PerfLogEntry, w io.Writer) {

	fmt.Fprintln(w, "Statistic '", ByteToString(ple.Name[:], NAME_LEN), "' :", ByteToString(ple.Desc[:], NAME_LEN), "log")
	fmt.Fprintln(w, "Entry size", ple.LogEntrySize, "LogBytes", ple.LogBytes)

	var oldestIndex int = int(hdr.NextLogIndex % NUM_LOG_ENTRIES)
	var index int = oldestIndex
	entriesLogged := 0
	exit := false

	for !exit {
		skip := false
		if entriesLogged%ENTRIES_PER_LINE == 0 {

			var t time.Time

			// Handle EntryTime == 0. Are there any valid times on this line?
			if hdr.EntryTimes[index].TimeTs == 0 {
				count := 0
				for i := 0; i < ENTRIES_PER_LINE; i++ {
					offIndex := (index + i) % NUM_LOG_ENTRIES
					if hdr.EntryTimes[offIndex].TimeTs == 0 {
						count++
					} else {
						if t.IsZero() {
							// Find the first time we recorded samples
							t = time.Unix(int64(hdr.EntryTimes[offIndex].TimeTs), 0)
							// and adjust back to start of line
							t = t.Add(-time.Duration(count) * time.Second)
							// Not entirely sure why I need to do this
							//							t = t.Add(7 * time.Hour)
						}
					}
				}

				if count == ENTRIES_PER_LINE {
					skip = true
				}
			} else {
				//			fmt.Println(hdr.EntryTimes[index].FastTicksVal, hdr.EntryTimes[index].TimeTs, hdr.EntryTimes[index].TimeTns)
				t = time.Unix(int64(hdr.EntryTimes[index].TimeTs), 0)
				// Not entirely sure why I need to do this
				//				t = t.Add(7 * time.Hour)
			}

			if !skip {
				fmt.Fprintf(w, "\n%s:\t", t.UTC().Format(time.UnixDate))
			}
		}
		if !skip {
			fmt.Fprintf(w, "%12d ", ple.Log[index])
			index = (index + 1) % NUM_LOG_ENTRIES
			entriesLogged++
		} else {
			skip = false
			index = (index + ENTRIES_PER_LINE) % NUM_LOG_ENTRIES
		}
		if index == oldestIndex {
			exit = true
		}
	}
	fmt.Fprintln(w)
}

func (perflog *PerfLogDecoder) Decoder(b binDecode.BinaryHdr, w io.Writer, r io.Reader) error {

	var hdr PerfLogHeaderMIPS
	var byteOrder binary.ByteOrder
	byteOrder = binary.LittleEndian
	if b.Endianness != 0 {
		byteOrder = binary.BigEndian
	}

	var err error

	// Need to do different things with the header depending on Architecture... could do this based on endianness,
	// but the problem is not just byte ordering, but also word length, so based to do it on architecture
	//
	// This approach loads the header as the appropriate architeture, and if it is ARM will convert it to the MIPS
	// layout. Note the ideal implementation. I imagine interfaces is the way to solve this better

	if b.Architecture == binDecode.BinaryFile_ArchMIPS {
		fmt.Println("MIPS header")
		err = binary.Read(r, byteOrder, &hdr)
	} else {
		fmt.Println("ARM header")
		var tmpHdr PerfLogHeaderARM
		err = binary.Read(r, byteOrder, &tmpHdr)
		hdr.convertArmHdrToMips(&tmpHdr)
	}
	if err == io.EOF {
		return nil
	}
	if err != nil {
		fmt.Println("Bad perflog header", err)
		return err
	}

	fmt.Fprintln(w, "PerfLog:", ByteToString(hdr.Name[:], NAME_LEN), "PauseReason", hdr.PauseReason, "Entries per record", hdr.RecordEntries)

	var rec PerfLogEntry
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

		perflog.DumpRecord(hdr, rec, w)
		count++
	}

	return nil
}
