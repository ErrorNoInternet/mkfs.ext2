package filesystem

import (
	"errors"
	"fmt"
	"time"

	"github.com/ErrorNoInternet/mkfs.ext2/bgdt"
	"github.com/ErrorNoInternet/mkfs.ext2/device"
	"github.com/ErrorNoInternet/mkfs.ext2/superblock"
	"github.com/google/uuid"
)

func Make(fileName string, blockSize, numBlocks int) error {
	if blockSize != 1024 && blockSize != 2048 && blockSize != 4096 {
		return errors.New("invalid blockSize specified")
	}
	filesystemDevice, err := device.New(fileName, int64(blockSize*numBlocks))
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
			fmt.Println(bgNum, "bgNum")
			offset := int64((bgNum*superblockObject.NumBlocksPerGroup + superblockObject.FirstBlockId) * blockSize)
			shadowSb, err := superblock.New(offset, filesystemDevice, bgNum, blockSize, numBlocks, currentTime, volumeIdBytes)
			if err != nil {
				return err
			}
			bgdt.New(bgNum, shadowSb, filesystemDevice)
		}
	}

	rootInodeOffset := bgdtObject.Entries[0].InodeTableLocation*superblockObject.BlockSize + superblockObject.InodeSize
	fmt.Println(rootInodeOffset, "offset")
	filesystemDevice.Unmount()
	return nil
}
