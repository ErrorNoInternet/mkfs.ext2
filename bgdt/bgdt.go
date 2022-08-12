package bgdt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"

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
}

func New(
	bgNumCopy int,
	superblockObject *superblock.Superblock,
	filesystemDevice *device.Device,
) (*Bgdt, error) {
	bgdt := &Bgdt{}
	bgdt.StartPos = (bgNumCopy*superblockObject.NumBlocksPerGroup + superblockObject.FirstBlockId + 1) * superblockObject.BlockSize
	bgdt.NumBgdtBlocks = int(math.Ceil(float64(superblockObject.NumBlockGroups*32) / float64(superblockObject.BlockSize)))
	bgdt.InodeTableBlocks = int(math.Ceil(float64(superblockObject.NumInodesPerGroup*superblockObject.InodeSize) / float64(superblockObject.BlockSize)))

	bgdtBytes := ""
	for bgroupNum := 0; bgroupNum < superblockObject.NumBlockGroups; bgroupNum++ {
		bgroupStartBid := bgroupNum*superblockObject.NumBlocksPerGroup + superblockObject.FirstBlockId
		bgdt.BlockBitmapLocation = bgroupStartBid
		bgdt.InodeBitmapLocation = bgroupStartBid + 1
		bgdt.InodeTableLocation = bgroupStartBid + 2
		bgdt.NumInodesAsDirs = 0

		bgdt.NumUsedBlocks = 2 + bgdt.InodeTableBlocks
		in := false
		for _, groupId := range superblockObject.CopyBlockGroupIds {
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
			bgdt.NumUsedInodes += (superblockObject.FirstInodeIndex - 1)
		}
		bgdt.NumFreeInodes = superblockObject.NumInodesPerGroup - bgdt.NumUsedInodes

		if bgroupNum != superblockObject.NumBlockGroups-1 {
			bgdt.NumTotalBlocksInGroup = superblockObject.NumBlocksPerGroup
		} else {
			bgdt.NumTotalBlocksInGroup = superblockObject.NumBlocks - bgroupStartBid
		}
		bgdt.NumFreeBlocks = bgdt.NumTotalBlocksInGroup - bgdt.NumUsedBlocks

		if bgdt.NumFreeBlocks < 0 {
			return bgdt, errors.New("not enough blocks specified")
		}

		if bgNumCopy == 0 {
			blockBitmap := []int{}
			for i := 0; i < superblockObject.BlockSize; i++ {
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
			for padBitIndex < superblockObject.BlockSize {
				blockBitmap[padBitIndex>>8] |= (1 << (padBitIndex & 0x07))
				padBitIndex += 1
			}
			blockBitmapBytes := make([]byte, superblockObject.BlockSize)
			for _, item := range blockBitmap {
				binary.PutVarint(blockBitmapBytes, int64(item))
			}
			filesystemDevice.Write(
				int64(bgdt.BlockBitmapLocation*superblockObject.BlockSize),
				blockBitmapBytes,
			)
		}
		format := []string{"I", "I", "I", "H", "H", "H"}
		bp := new(binary_pack.BinaryPack)
		entryBytes, err := bp.Pack(
			format,
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
		format = []string{}
		for i := 0; i < 14; i++ {
			format = append(format, "B")
		}
		zeroFill := []interface{}{}
		for i := 0; i < 14; i++ {
			zeroFill = append(zeroFill, 0)
		}
		bp = new(binary_pack.BinaryPack)
		emptyBytes, err := bp.Pack(format, zeroFill)
		newBytes := bytes.Join([][]byte{[]byte(bgdtBytes), entryBytes, emptyBytes}, []byte(""))
		filesystemDevice.Write(int64(bgdt.StartPos), newBytes)
	}
	return bgdt, nil
}
