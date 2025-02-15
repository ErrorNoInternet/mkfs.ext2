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
	dev *device.Device,
	sb *superblock.Superblock,
	bid int,
	offset int64,
	data []byte,
) {
	dev.Write(offset+int64(bid)*int64(sb.BlockSize), data)
	sb.SetTimeLastWrite(time.Now().Unix())
}

func ReadBlock(
	dev *device.Device,
	sb *superblock.Superblock,
	bid int,
	offset int64,
	count int,
) []byte {
	if count == 0 {
		count = sb.BlockSize
	}
	block := dev.Read(int64(bid*sb.BlockSize), int64(count))
	return block
}

func Make(file *os.File, blockSize, numBlocks int) error {
	if blockSize != 1024 && blockSize != 2048 && blockSize != 4096 {
		return errors.New("unsupported blockSize specified")
	}

	dev, err := device.New(file, int64(blockSize*numBlocks))
	if err != nil {
		return err
	}

	currentTime := time.Now().Unix()
	volumeIdBytes := [16]byte(uuid.New())
	sb, err := superblock.New(
		1024,
		dev,
		0,
		blockSize,
		numBlocks,
		currentTime,
		volumeIdBytes,
	)
	if err != nil {
		return err
	}
	dt, err := bgdt.New(0, sb, dev)
	if err != nil {
		return err
	}
	if len(sb.CopyBlockGroupIds) > 0 {
		for _, bgNum := range sb.CopyBlockGroupIds[1:] {
			offset := int64((bgNum*sb.NumBlocksPerGroup + sb.FirstBlockId) * blockSize)
			shadowSb, err := superblock.New(offset, dev, bgNum, blockSize, numBlocks, currentTime, volumeIdBytes)
			if err != nil {
				return err
			}
			bgdt.New(bgNum, shadowSb, dev)
		}
	}

	rootInodeOffset := dt.Entries[0].InodeTableLocation*sb.BlockSize + sb.InodeSize
	emptyBytes := []byte("")
	for i := 0; i < sb.InodeSize-26; i++ {
		emptyBytes = binary.AppendVarint(emptyBytes, 0)
	}
	uid := 0
	gid := 0
	mode := 0x4000 | 0x0100 | 0x0080 | 0x0040 | 0x0020 | 0x0008 | 0x0004 | 0x0001
	format := []string{"H", "H", "I", "I", "I", "I", "I", "H"}
	values := []interface{}{mode, uid, 0, int(currentTime), int(currentTime), int(currentTime), 0, gid}
	bp := new(binary_pack.BinaryPack)
	rootInodeBytes, err := bp.Pack(format, values)
	if err != nil {
		return err
	}
	rootInodeBytes = bytes.Join([][]byte{rootInodeBytes, emptyBytes}, []byte(""))
	dev.Write(int64(rootInodeOffset), rootInodeBytes)

	sb.SaveCopies = true
	dt.Entries[0].SetNumInodesAsDirs(dt.Entries[0].NumInodesAsDirs + 1)

	bitmapSize := sb.NumBlocksPerGroup / 8
	var bitmapStartPos int = -1
	var groupNum int
	var bgdtEntry *bgdt.BgdtEntry
	for groupNum, bgdtEntry = range dt.Entries {
		if bgdtEntry.NumFreeBlocks > 0 {
			bitmapStartPos = bgdtEntry.BlockBitmapLocation * sb.BlockSize
			break
		}
	}
	if bitmapStartPos == -1 {
		return errors.New("no free blocks")
	}

	bitmapBytes := dev.Read(int64(bitmapStartPos), int64(bitmapSize))
	if len(bitmapBytes) < bitmapSize {
		return errors.New("invalid block bitmap")
	}
	bitmap := []uint8(bitmapBytes)

	var rootBid int = -1
	for idx, val := range bitmap {
		if rootBid != -1 {
			break
		}
		if val != 255 {
			for i := 0; i < 8; i++ {
				if (1<<i)&val == 0 {
					bid := (groupNum * sb.NumBlocksPerGroup) + (idx * 8) + i + sb.FirstBlockId
					dev.Write(int64(bitmapStartPos+idx), []byte([]uint8{val | (1 << i)}))
					sb.SetNumFreeBlocks(sb.NumFreeBlocks - 1)
					bgdtEntry.SetNumFreeBlocks(bgdtEntry.NumFreeBlocks - 1)

					start := bid * sb.BlockSize
					emptyBytes := []byte("")
					for i := 0; i < sb.BlockSize; i++ {
						emptyBytes = binary.AppendVarint(emptyBytes, 0)
					}
					dev.Write(int64(start), emptyBytes)

					sb.SetTimeLastWrite(time.Now().Unix())
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
	WriteToBlock(dev, sb, rootBid, 0, bytes.Join([][]byte{defaultEntries1, defaultEntries2, defaultEntries3, defaultEntries4, defaultEntries5, defaultEntries6, defaultEntries7}, []byte("")))

	inodeNum := 2
	bgroupNum := (inodeNum - 1) / sb.NumInodesPerGroup
	bgroupIndex := (inodeNum - 1) % sb.NumInodesPerGroup
	bgdtEntry = dt.Entries[bgroupNum]
	tableBid := bgdtEntry.InodeTableLocation + (bgroupIndex*sb.InodeSize)/blockSize
	inodeTableOffset := (bgroupIndex * sb.InodeSize) % blockSize

	data, err := bp.Pack([]string{"h"}, []interface{}{2})
	if err != nil {
		return err
	}
	WriteToBlock(dev, sb, tableBid, int64(inodeTableOffset+26), data)

	inodeBlocks := []int{}
	for i := 0; i < 15; i++ {
		inodeBlocks = append(inodeBlocks, 0)
	}
	size := 0
	inodeNumDataBlocks := int(math.Ceil(float64(size) / float64(sb.BlockSize)))
	inodeNumDirectBlocks := 12
	if inodeNumDataBlocks < inodeNumDirectBlocks {
		inodeBlocks[inodeNumDataBlocks] = rootBid
		data, err = bp.Pack([]string{"I"}, []interface{}{rootBid})
		WriteToBlock(dev, sb, tableBid, int64(inodeTableOffset+(40+inodeNumDataBlocks*4)), data)
		if err != nil {
			return err
		}
		inodeNumDataBlocks += 1
		data, err = bp.Pack([]string{"I"}, []interface{}{inodeNumDataBlocks * (2 << sb.LogBlockSize)})
		if err != nil {
			return err
		}
		WriteToBlock(dev, sb, tableBid, int64(inodeTableOffset+28), data)
	}

	inodeSize := sb.BlockSize
	data, err = bp.Pack([]string{"I"}, []interface{}{inodeSize & 0xFFFFFFFF})
	if err != nil {
		return err
	}
	WriteToBlock(dev, sb, tableBid, int64(inodeTableOffset+4), data)
	if sb.RevLevel > 0 && (mode&0x8000) != 0 {
		data, err = bp.Pack([]string{"I"}, []interface{}{inodeSize >> 32})
		if err != nil {
			return err
		}
		WriteToBlock(dev, sb, tableBid, int64(inodeTableOffset+108), data)
	}

	dev.Unmount()
	return nil
}
