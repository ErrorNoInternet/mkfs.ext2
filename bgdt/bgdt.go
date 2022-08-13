package bgdt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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
	fmt.Println("BGDT START")

	bgdt := &Bgdt{}
	bgdt.StartPos = (bgNumCopy*superblockObject.NumBlocksPerGroup + superblockObject.FirstBlockId + 1) * superblockObject.BlockSize
	bgdt.NumBgdtBlocks = int(math.Ceil(float64(superblockObject.NumBlockGroups*32) / float64(superblockObject.BlockSize)))
	bgdt.InodeTableBlocks = int(math.Ceil(float64(superblockObject.NumInodesPerGroup*superblockObject.InodeSize) / float64(superblockObject.BlockSize)))
	fmt.Println("InodeTableBlocks", bgdt.InodeTableBlocks)

	bgdtBytes := []byte("")
	fmt.Println("superBlockGroups", superblockObject.NumBlockGroups)
	superblockObject.CopyBlockGroupIds = append(superblockObject.CopyBlockGroupIds, 0)
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
			blockBitmap := []uint64{}
			for i := 0; i < superblockObject.BlockSize; i++ {
				blockBitmap = append(blockBitmap, 0)
			}
			bitmapIndex := 0
			fmt.Println("superBlockSize", superblockObject.BlockSize)
			fmt.Println(bgdt.NumUsedBlocks, "AaA")
			for i := 0; i < bgdt.NumUsedBlocks; i++ {
				blockBitmap[bitmapIndex] <<= 1
				blockBitmap[bitmapIndex] |= 1
				if (i+1)%8 == 0 {
					bitmapIndex += 1
				}
			}
			fmt.Println("block", len(blockBitmap))
			padBitIndex := bgdt.NumTotalBlocksInGroup
			for padBitIndex < superblockObject.BlockSize {
				blockBitmap[padBitIndex>>8] |= (1 << (padBitIndex & 0x07))
				padBitIndex += 1
			}
			blockBitmapBytes := []byte("")
			for _, item := range blockBitmap {
				newByte := make([]byte, 2)
				binary.PutUvarint(newByte, item)
				blockBitmapBytes = bytes.Join([][]byte{blockBitmapBytes, newByte[:1]}, []byte(""))
			}
			filesystemDevice.Write(
				int64(bgdt.BlockBitmapLocation*superblockObject.BlockSize),
				blockBitmapBytes,
			)

			inodeBitmap := []uint64{}
			for i := 0; i < superblockObject.BlockSize; i++ {
				inodeBitmap = append(inodeBitmap, 0)
			}
			bitmapIndex = 0
			for i := 0; i < bgdt.NumUsedBlocks; i++ {
				inodeBitmap[bitmapIndex] <<= 1
				inodeBitmap[bitmapIndex] |= 1
				if (i+1)%8 == 0 {
					bitmapIndex += 1
				}
			}
			inodeBitmapBytes := []byte("")
			for _, item := range inodeBitmap {
				newByte := make([]byte, 2)
				binary.PutUvarint(newByte, item)
				inodeBitmapBytes = bytes.Join([][]byte{inodeBitmapBytes, newByte[:1]}, []byte(""))
			}
			fmt.Println("inode", len(inodeBitmapBytes))
			filesystemDevice.Write(
				int64(bgdt.InodeBitmapLocation*superblockObject.BlockSize),
				inodeBitmapBytes,
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
	}
	filesystemDevice.Write(int64(bgdt.StartPos), bgdtBytes)
	return bgdt, nil
}
