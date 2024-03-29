package filesystem

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"time"

	"github.com/ErrorNoInternet/mkfs.ext2/bgdt"
	"github.com/ErrorNoInternet/mkfs.ext2/device"
	"github.com/ErrorNoInternet/mkfs.ext2/superblock"
	"github.com/google/uuid"
	binary_pack "github.com/roman-kachanovsky/go-binary-pack/binary-pack"
)

func WriteToBlock(
	filesystemDevice *device.Device,
	superblockObject *superblock.Superblock,
	bid int,
	offset int64,
	data []byte,
) {
	filesystemDevice.Write(offset+int64(bid)*int64(superblockObject.BlockSize), data)
	superblockObject.SetTimeLastWrite(time.Now().Unix())
}

func ReadBlock(
	filesystemDevice *device.Device,
	superblockObject *superblock.Superblock,
	bid int,
	offset int64,
	count int,
) []byte {
	if count == 0 {
		count = superblockObject.BlockSize
	}
	block := filesystemDevice.Read(int64(bid*superblockObject.BlockSize), int64(count))
	return block
}

func Make(file *os.File, blockSize, numBlocks int) error {
	if blockSize != 1024 && blockSize != 2048 && blockSize != 4096 {
		return errors.New("invalid blockSize specified")
	}
	filesystemDevice, err := device.New(file, int64(blockSize*numBlocks))
	if err != nil {
		return err
	}

	currentTime := time.Now().Unix()
	volumeIdBytes := [16]byte(uuid.New())
	superblockObject, err := superblock.New(
		1024,
		filesystemDevice,
		0,
		blockSize,
		numBlocks,
		currentTime,
		volumeIdBytes,
	)
	if err != nil {
		return err
	}
	bgdtObject, err := bgdt.New(0, superblockObject, filesystemDevice)
	if err != nil {
		return err
	}
	if len(superblockObject.CopyBlockGroupIds) > 0 {
		for _, bgNum := range superblockObject.CopyBlockGroupIds[1:] {
			offset := int64((bgNum*superblockObject.NumBlocksPerGroup + superblockObject.FirstBlockId) * blockSize)
			shadowSb, err := superblock.New(offset, filesystemDevice, bgNum, blockSize, numBlocks, currentTime, volumeIdBytes)
			if err != nil {
				return err
			}
			bgdt.New(bgNum, shadowSb, filesystemDevice)
		}
	}

	rootInodeOffset := bgdtObject.Entries[0].InodeTableLocation*superblockObject.BlockSize + superblockObject.InodeSize
	emptyBytes := []byte("")
	for i := 0; i < superblockObject.InodeSize-26; i++ {
		emptyBytes = binary.AppendVarint(emptyBytes, 0)
	}
	uid := 0
	gid := 0
	mode := 0
	mode |= 0x4000
	mode |= 0x0100
	mode |= 0x0080
	mode |= 0x0040
	mode |= 0x0020
	mode |= 0x0008
	mode |= 0x0004
	mode |= 0x0001
	format := []string{"H", "H", "I", "I", "I", "I", "I", "H"}
	values := []interface{}{mode, uid, 0, int(currentTime), int(currentTime), int(currentTime), 0, gid}
	bp := new(binary_pack.BinaryPack)
	rootInodeBytes, err := bp.Pack(format, values)
	if err != nil {
		return err
	}
	rootInodeBytes = bytes.Join([][]byte{rootInodeBytes, emptyBytes}, []byte(""))
	filesystemDevice.Write(int64(rootInodeOffset), rootInodeBytes)

	superblockObject.SaveCopies = true
	bgdtObject.Entries[0].SetNumInodesAsDirs(bgdtObject.Entries[0].NumInodesAsDirs + 1)

	bitmapSize := superblockObject.NumBlocksPerGroup / 8
	var bitmapStartPos int = -1
	var groupNum int
	var bgdtEntry *bgdt.BgdtEntry
	for groupNum, bgdtEntry = range bgdtObject.Entries {
		if bgdtEntry.NumFreeBlocks > 0 {
			bitmapStartPos = bgdtEntry.BlockBitmapLocation * superblockObject.BlockSize
			break
		}
	}
	if bitmapStartPos == -1 {
		return errors.New("no free blocks")
	}

	bitmapBytes := filesystemDevice.Read(int64(bitmapStartPos), int64(bitmapSize))
	if len(bitmapBytes) < bitmapSize {
		return errors.New("invalid block bitmap")
	}
	bitmap := []uint8(bitmapBytes)

	var rootBid int = -1
	for byteIndex, byteObject := range bitmap {
		if rootBid != -1 {
			break
		}
		if byteObject != 255 {
			for i := 0; i < 8; i++ {
				if (1<<i)&byteObject == 0 {
					bid := (groupNum * superblockObject.NumBlocksPerGroup) + (byteIndex * 8) + i + superblockObject.FirstBlockId
					filesystemDevice.Write(int64(bitmapStartPos+byteIndex), []byte([]uint8{byteObject | (1 << i)}))
					superblockObject.SetNumFreeBlocks(superblockObject.NumFreeBlocks - 1)
					bgdtEntry.SetNumFreeBlocks(bgdtEntry.NumFreeBlocks - 1)

					start := bid * superblockObject.BlockSize
					emptyBytes := []byte("")
					for i := 0; i < superblockObject.BlockSize; i++ {
						emptyBytes = binary.AppendVarint(emptyBytes, 0)
					}
					filesystemDevice.Write(int64(start), emptyBytes)

					superblockObject.SetTimeLastWrite(time.Now().Unix())
					rootBid = bid
					break
				}
			}
		}
	}
	defaultEntries1, err := bp.Pack([]string{"I", "H"}, []interface{}{2, 12})
	if err != nil {
		return err
	}
	defaultEntries2 := []byte([]uint8{1, 2})
	defaultEntries3, err := bp.Pack([]string{"1s"}, []interface{}{"."})
	if err != nil {
		return err
	}
	defaultEntries4 := []byte([]uint8{0, 0, 0})
	defaultEntries5, err := bp.Pack([]string{"I", "H"}, []interface{}{2, blockSize - 12})
	if err != nil {
		return err
	}
	defaultEntries6 := []byte([]uint8{2, 2})
	defaultEntries7, err := bp.Pack([]string{"2s"}, []interface{}{".."})
	if err != nil {
		return err
	}
	WriteToBlock(filesystemDevice, superblockObject, rootBid, 0, bytes.Join([][]byte{defaultEntries1, defaultEntries2, defaultEntries3, defaultEntries4, defaultEntries5, defaultEntries6, defaultEntries7}, []byte("")))

	inodeNum := 2
	bgroupNum := (inodeNum - 1) / superblockObject.NumInodesPerGroup
	bgroupIndex := (inodeNum - 1) % superblockObject.NumInodesPerGroup
	bgdtEntry = bgdtObject.Entries[bgroupNum]
	tableBid := bgdtEntry.InodeTableLocation + (bgroupIndex*superblockObject.InodeSize)/blockSize
	inodeTableOffset := (bgroupIndex * superblockObject.InodeSize) % blockSize

	data, err := bp.Pack([]string{"h"}, []interface{}{2})
	if err != nil {
		return err
	}
	WriteToBlock(filesystemDevice, superblockObject, tableBid, int64(inodeTableOffset+26), data)

	inodeBlocks := []int{}
	for i := 0; i < 15; i++ {
		inodeBlocks = append(inodeBlocks, 0)
	}
	size := 0
	inodeNumDataBlocks := int(math.Ceil(float64(size) / float64(superblockObject.BlockSize)))
	inodeNumDirectBlocks := 12
	if inodeNumDataBlocks < inodeNumDirectBlocks {
		inodeBlocks[inodeNumDataBlocks] = rootBid
		data, err = bp.Pack([]string{"I"}, []interface{}{rootBid})
		WriteToBlock(filesystemDevice, superblockObject, tableBid, int64(inodeTableOffset+(40+inodeNumDataBlocks*4)), data)
		if err != nil {
			return err
		}
		inodeNumDataBlocks += 1
		data, err = bp.Pack([]string{"I"}, []interface{}{inodeNumDataBlocks * (2 << superblockObject.LogBlockSize)})
		if err != nil {
			return err
		}
		WriteToBlock(filesystemDevice, superblockObject, tableBid, int64(inodeTableOffset+28), data)
	}

	inodeSize := superblockObject.BlockSize
	data, err = bp.Pack([]string{"I"}, []interface{}{inodeSize & 0xFFFFFFFF})
	if err != nil {
		return err
	}
	WriteToBlock(filesystemDevice, superblockObject, tableBid, int64(inodeTableOffset+4), data)
	if superblockObject.RevLevel > 0 && (mode&0x8000) != 0 {
		data, err = bp.Pack([]string{"I"}, []interface{}{inodeSize >> 32})
		if err != nil {
			return err
		}
		WriteToBlock(filesystemDevice, superblockObject, tableBid, int64(inodeTableOffset+108), data)
	}

	filesystemDevice.Unmount()
	return nil
}
