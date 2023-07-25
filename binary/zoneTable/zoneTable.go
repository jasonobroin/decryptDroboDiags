// eventlog.go
//
// Copyright (c) 2016 Drobo Inc. All rights reserved
//
// Methods for decoding binary zone table files from Drobo diagnostics
//
package eventlog

import (
	binDecode "decryptDiags/binary"
	"encoding/binary"
	"fmt"

	"io"
)

const (
	REGIONS_PER_ZONE_DEFAULT = 24
	MAX_REGIONS_PER_ZONE     = REGIONS_PER_ZONE_DEFAULT * 3
)

type ZoneNumber uint32
type RedundancyType uint32
type LogicalDisk uint16
type RegionNumber uint32
type ZoneFlags uint32

type ZoneTableEntry struct {
	ZoneNum        ZoneNumber
	Redundancy     RedundancyType
	LogicalDisks   [MAX_REGIONS_PER_ZONE]LogicalDisk
	Regions        [MAX_REGIONS_PER_ZONE]RegionNumber
	Flags          ZoneFlags
	WriteTimestamp uint32
	IoCount        uint32
	BlockSize      uint32
}

// RedundancyType consts
const (
	None RedundancyType = iota
	SelfMirrored
	Mirrored
	VStripe3
	VStripe4
	VStripe5
	VStripe7
	HStripe3
	HStripe4
	HStripe5
	HStripe7
	HStripe9
	Mirrored3
	DRStripe4
	DRStripe5
	DRStripe6
	DRStripe8
	DRStripe10
	MStripe4
	MStripe6
	MStripe8
	MStripe12
	M3Stripe6
	M3Stripe9
	M3Stripe12
	PQStripe4
	PQStripe5
	PQStripe6
	PQStripe8
	PQStripe10
	MaxRedundancyType
)

type ZoneTypeData struct {
	name  string
	width uint32
}

var (
	RedundancyTypeInfo = [...]ZoneTypeData{
		None:              {"None", 1},
		SelfMirrored:      {"SelfMirrored", 2},
		Mirrored:          {"Mirrored", 2},
		VStripe3:          {"VStripe3", 0},
		VStripe4:          {"VStripe4", 0},
		VStripe5:          {"VStripe5", 0},
		VStripe7:          {"VStripe7", 0},
		HStripe3:          {"HStripe3", 3},
		HStripe4:          {"HStripe4", 4},
		HStripe5:          {"HStripe5", 5},
		HStripe7:          {"HStripe7", 7},
		HStripe9:          {"HStripe9", 9},
		Mirrored3:         {"Mirrored3", 3},
		DRStripe4:         {"DStripe4", 4},
		DRStripe5:         {"DStripe5", 5},
		DRStripe6:         {"DStripe6", 6},
		DRStripe8:         {"DStripe8", 8},
		DRStripe10:        {"DStripe10", 10},
		MStripe4:          {"MStripe4", 2},
		MStripe6:          {"MStripe6", 2},
		MStripe8:          {"MStripe8", 2},
		MStripe12:         {"MStripe12", 2},
		M3Stripe6:         {"M3Stripe6", 3},
		M3Stripe9:         {"M3Stripe9", 3},
		M3Stripe12:        {"M3Stripe12", 3},
		PQStripe4:         {"PQStripe4", 4},
		PQStripe5:         {"PQStripe5", 5},
		PQStripe6:         {"PQStripe6", 6},
		PQStripe8:         {"PQStripe8", 8},
		PQStripe10:        {"PQStripe10", 10},
		MaxRedundancyType: {"MaxRedundancyType", 0},
	}
)

// Endian issue here!
const (
	MirrorOnly ZoneFlags = iota
	Metadata
	InUse
	PreInit
	InitComplete
	InitInProgress
	Transactional
	RelayoutNeeded
	UnusedZoneTableFlag
)

// BitFlip swaps the order of bits
//
// ZoneFlags are a bit field, so on big endian architectures, the bits are ordered starting with bit31, not bit0
// so byteswapping is insufficient
func (flags *ZoneFlags) BitFlip() {
	var temp uint32

	var i uint32
	for i = 0; i < 32; i++ {
		if uint32(*flags)&(1<<i) != 0 {
			temp |= 1 << (31 - i)
		}
	}
	*flags = ZoneFlags(temp)
}

type ZoneFlagsDecode struct {
	Name  string
	Sense bool // Display flag if true or false
}

var (
	ZoneFlagsStrings = [...]ZoneFlagsDecode{
		MirrorOnly:          {"MirrorOnly", true},
		Metadata:            {"Metadata", true},
		InUse:               {"NotInUse", false},
		PreInit:             {"PreInitialized", true},
		InitComplete:        {"InitializationIncomplete", false},
		InitInProgress:      {"Initializing", true},
		Transactional:       {"Transactional", true},
		RelayoutNeeded:      {"RelayoutNeeded", true},
		UnusedZoneTableFlag: {"UnusedZoneTableFlag", true},
	}
)

func (flags ZoneFlags) InUse() bool {
	if flags&(1<<InUse) == (1 << InUse) {
		return true
	}
	return false
}

func (zte *ZoneTableEntry) HasRegions() bool {
	if zte.LogicalDisks[0] == 0 && zte.Regions[0] == 0 && zte.LogicalDisks[1] == 0 && zte.Regions[1] == 0 {
		return false
	}
	return true
}

type ZoneTableDecoder struct{}

var zoneTableDecoder ZoneTableDecoder

// Register decoder function
func init() {
	binDecode.RegisterDecoder(binDecode.BinaryFile_ZoneTable, &zoneTableDecoder)
}

func (zoneTable *ZoneTableDecoder) GetRegionCount(zte ZoneTableEntry) uint32 {
	var regions uint32
	switch zte.Redundancy {
	case SelfMirrored:
		regions = 2 * REGIONS_PER_ZONE_DEFAULT
	case MStripe4, MStripe6, MStripe8, MStripe12, Mirrored:
		regions = 2 * REGIONS_PER_ZONE_DEFAULT
	case M3Stripe6, M3Stripe9, M3Stripe12, Mirrored3:
		regions = 3 * REGIONS_PER_ZONE_DEFAULT
	case HStripe3, HStripe4, HStripe5, HStripe7, HStripe9:
		regions = REGIONS_PER_ZONE_DEFAULT / (RedundancyTypeInfo[zte.Redundancy].width - 1) * RedundancyTypeInfo[zte.Redundancy].width
	case DRStripe4, DRStripe5, DRStripe6, DRStripe8, DRStripe10, PQStripe4, PQStripe5, PQStripe6, PQStripe8, PQStripe10:
		regions = REGIONS_PER_ZONE_DEFAULT / (RedundancyTypeInfo[zte.Redundancy].width - 2) * RedundancyTypeInfo[zte.Redundancy].width
	default:
		regions = REGIONS_PER_ZONE_DEFAULT
	}

	return regions
}

func (zonetable *ZoneTableDecoder) DumpRecord(zte ZoneTableEntry, w io.Writer) {

	if zte.Flags.InUse() {

		regions := zonetable.GetRegionCount(zte)

		fmt.Fprintf(w, "TableEntry: Zone= %d Redundancy:%s flags= 0x%x", zte.ZoneNum,
			RedundancyTypeInfo[zte.Redundancy].name, zte.Flags)

		// Zone flags output
		var bit ZoneFlags = 0
		for ; bit < UnusedZoneTableFlag; bit++ {
			if zte.Flags&(1<<bit) == (1 << bit) {
				if ZoneFlagsStrings[bit].Sense {
					fmt.Fprintf(w, " %s", ZoneFlagsStrings[bit].Name)
				}
			} else {
				if !ZoneFlagsStrings[bit].Sense {
					fmt.Fprintf(w, " %s", ZoneFlagsStrings[bit].Name)
				}
			}
		}

		fmt.Fprintf(w, "\n  LastWrittenTimestamp = %d Small IOCount = %d block size = %d", zte.WriteTimestamp,
			zte.IoCount, zte.BlockSize)

		if zte.HasRegions() {
			var region uint32
			for region = 0; region < regions; region++ {
				if region%12 == 0 {
					fmt.Fprintf(w, "\n     ")
				}
				fmt.Fprintf(w, "%d:%d ", zte.LogicalDisks[region], zte.Regions[region])
			}
		} else {
			fmt.Fprintf(w, "\n     Has no Regions allocated")
		}
		fmt.Fprintf(w, "\n\n")
	}
}

func (zoneTable *ZoneTableDecoder) Decoder(b binDecode.BinaryHdr, w io.Writer, r io.Reader) error {

	var zte ZoneTableEntry
	var zone int = 0
	var byteOrder binary.ByteOrder
	byteOrder = binary.LittleEndian
	if b.Endianness != 0 {
		byteOrder = binary.BigEndian
	}

	for true {
		err := binary.Read(r, byteOrder, &zte)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			fmt.Println("ZTE#", zone, ":", err)
			return err
		}

		// The ZoneFlags don't appear be getting byte swapped, so force it by hand
		if b.Endianness != 0 {
			zte.Flags.BitFlip()
		}

		zoneTable.DumpRecord(zte, w)
		zone++
	}

	return nil
}
