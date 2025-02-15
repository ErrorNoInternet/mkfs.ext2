package bgdt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"time"

	"github.com/ErrorNoInternet/mkfs.ext2/device"
	"github.com/ErrorNoInternet/mkfs.ext2/superblock"
	binary_pack "github.com/roman-kachanovsky/go-binary-pack/binary-pack"
)

type Bgdt struct {
	StartPos              int
	NumBgdtBlocks         int
	BlockBitmapLocation   int
	InodeBitmapLocation   int
	InodeTableLocation    int
	InodeTableBlocks      int
	NumTotalBlocksInGroup int
	NumInodesAsDirs       int
	NumUsedBlocks         int
	NumFreeBlocks         int
	NumUsedInodes         int
	NumFreeInodes         int
	Entries               []*BgdtEntry
}

type BgdtEntry struct {
	StartPos            int
	BlockBitmapLocation int
	InodeBitmapLocation int
	InodeTableLocation  int
	InodeTableBlocks    int
	NumFreeBlocks       int
	NumFreeInodes       int
	NumInodesAsDirs     int
	Device              *device.Device
	Superblock          *superblock.Superblock
}

func (bgdtEntry *BgdtEntry) SetNumFreeBlocks(numFreeBlocks int) error {
	bgdtEntry.NumFreeBlocks = numFreeBlocks
	bp := new(binary_pack.BinaryPack)
	bytes, err := bp.Pack([]string{"H"}, []interface{}{numFreeBlocks})
	if err != nil {
		return err
	}
	bgdtEntry.WriteData(12, bytes)
	return nil
}

func (bgdtEntry *BgdtEntry) SetNumFreeInodes(numFreeInodes int) error {
	bgdtEntry.NumFreeInodes = numFreeInodes
	bp := new(binary_pack.BinaryPack)
	bytes, err := bp.Pack([]string{"H"}, []interface{}{numFreeInodes})
	if err != nil {
		return err
	}
	bgdtEntry.WriteData(14, bytes)
	return nil
}

func (bgdtEntry *BgdtEntry) SetNumInodesAsDirs(numInodesAsDirs int) error {
	bgdtEntry.NumInodesAsDirs = numInodesAsDirs
	bp := new(binary_pack.BinaryPack)
	bytes, err := bp.Pack([]string{"H"}, []interface{}{numInodesAsDirs})
	if err != nil {
		return err
	}
	bgdtEntry.WriteData(16, bytes)
	return nil
}

func (bgdtEntry *BgdtEntry) WriteData(offset int64, data []byte) {
	for _, groupId := range bgdtEntry.Superblock.CopyBlockGroupIds {
		groupStart := groupId * bgdtEntry.Superblock.NumBlocksPerGroup * bgdtEntry.Superblock.BlockSize
		tableStart := groupStart + (bgdtEntry.Superblock.BlockSize * (bgdtEntry.Superblock.FirstBlockId + 1))
		bgdtEntry.Device.Write(int64(tableStart+bgdtEntry.StartPos)+offset, data)
		if !bgdtEntry.Superblock.SaveCopies {
			break
		}
	}
	bgdtEntry.Superblock.TimeLastWrite = time.Now().Unix()
}

func New(
	bgNumCopy int,
	sb *superblock.Superblock,
	dev *device.Device,
) (*Bgdt, error) {
	bgdt := &Bgdt{}
	bgdt.Entries = []*BgdtEntry{}
	bgdt.StartPos = (bgNumCopy*sb.NumBlocksPerGroup + sb.FirstBlockId + 1) * sb.BlockSize
	bgdt.NumBgdtBlocks = int(math.Ceil(float64(sb.NumBlockGroups*32) / float64(sb.BlockSize)))
	bgdt.InodeTableBlocks = int(math.Ceil(float64(sb.NumInodesPerGroup*sb.InodeSize) / float64(sb.BlockSize)))

	bgdtBytes := []byte("")
	for bgroupNum := 0; bgroupNum < sb.NumBlockGroups; bgroupNum++ {
		bgroupStartBid := bgroupNum*sb.NumBlocksPerGroup + sb.FirstBlockId
		bgdt.BlockBitmapLocation = bgroupStartBid
		bgdt.InodeBitmapLocation = bgroupStartBid + 1
		bgdt.InodeTableLocation = bgroupStartBid + 2
		bgdt.NumInodesAsDirs = 0

		bgdt.NumUsedBlocks = 2 + bgdt.InodeTableBlocks
		in := false
		for _, groupId := range sb.CopyBlockGroupIds {
			if bgroupNum == groupId {
				in = true
			}
		}
		if in {
			bgdt.NumUsedBlocks += (1 + bgdt.NumBgdtBlocks)
			bgdt.BlockBitmapLocation += (1 + bgdt.NumBgdtBlocks)
			bgdt.InodeBitmapLocation += (1 + bgdt.NumBgdtBlocks)
			bgdt.InodeTableLocation += (1 + bgdt.NumBgdtBlocks)
		}

		bgdt.NumUsedInodes = 0
		if bgroupNum == 0 {
			bgdt.NumUsedInodes += (sb.FirstInodeIndex - 1)
		}
		bgdt.NumFreeInodes = sb.NumInodesPerGroup - bgdt.NumUsedInodes

		if bgroupNum != sb.NumBlockGroups-1 {
			bgdt.NumTotalBlocksInGroup = sb.NumBlocksPerGroup
		} else {
			bgdt.NumTotalBlocksInGroup = sb.NumBlocks - bgroupStartBid
		}
		bgdt.NumFreeBlocks = bgdt.NumTotalBlocksInGroup - bgdt.NumUsedBlocks

		if bgdt.NumFreeBlocks < 0 {
			return bgdt, errors.New("not enough blocks specified")
		}

		if bgNumCopy == 0 {
			blockBitmap := []uint8{}
			for i := 0; i < sb.BlockSize; i++ {
				blockBitmap = append(blockBitmap, 0)
			}
			bitmapIndex := 0
			for i := 0; i < bgdt.NumUsedBlocks; i++ {
				blockBitmap[bitmapIndex] <<= 1
				blockBitmap[bitmapIndex] |= 1
				if (i+1)%8 == 0 {
					bitmapIndex += 1
				}
			}
			padBitIndex := bgdt.NumTotalBlocksInGroup
			for padBitIndex < sb.BlockSize {
				blockBitmap[padBitIndex>>8] |= (1 << (padBitIndex & 0x07))
				padBitIndex += 1
			}
			dev.Write(
				int64(bgdt.BlockBitmapLocation*sb.BlockSize),
				[]byte(blockBitmap),
			)

			inodeBitmap := []uint8{}
			for i := 0; i < sb.BlockSize; i++ {
				inodeBitmap = append(inodeBitmap, 0)
			}
			bitmapIndex = 0
			for i := 0; i < bgdt.NumUsedInodes; i++ {
				inodeBitmap[bitmapIndex] <<= 1
				inodeBitmap[bitmapIndex] |= 1
				if (i+1)%8 == 0 {
					bitmapIndex += 1
				}
			}
			dev.Write(
				int64(bgdt.InodeBitmapLocation*sb.BlockSize),
				[]byte(inodeBitmap),
			)
		}
		bp := new(binary_pack.BinaryPack)
		entryBytes, err := bp.Pack(
			[]string{"I", "I", "I", "H", "H", "H"},
			[]interface{}{
				bgdt.BlockBitmapLocation,
				bgdt.InodeBitmapLocation,
				bgdt.InodeTableLocation,
				bgdt.NumFreeBlocks,
				bgdt.NumFreeInodes,
				bgdt.NumInodesAsDirs,
			},
		)
		if err != nil {
			return bgdt, errors.New("unable to pack bytes: " + err.Error())
		}
		emptyBytes := []byte("")
		for i := 0; i < 14; i++ {
			emptyBytes = binary.AppendVarint(emptyBytes, 0)
		}
		bgdtBytes = bytes.Join([][]byte{bgdtBytes, entryBytes, emptyBytes}, []byte(""))

		startPos := bgroupNum * 32
		entry := &BgdtEntry{
			StartPos:            startPos,
			Device:              dev,
			Superblock:          sb,
			BlockBitmapLocation: bgdt.BlockBitmapLocation,
			InodeBitmapLocation: bgdt.InodeBitmapLocation,
			InodeTableLocation:  bgdt.InodeTableLocation,
		}
		entry.SetNumFreeBlocks(bgdt.NumFreeBlocks)
		entry.SetNumFreeInodes(bgdt.NumFreeInodes)
		entry.SetNumInodesAsDirs(bgdt.NumInodesAsDirs)
		bgdt.Entries = append(bgdt.Entries, entry)
	}
	dev.Write(int64(bgdt.StartPos), bgdtBytes)

	return bgdt, nil
}
