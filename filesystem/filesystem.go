package filesystem

import (
	"errors"
	"fmt"
	"time"

	"github.com/ErrorNoInternet/mkfs.ext2/device"
	"github.com/ErrorNoInternet/mkfs.ext2/superblock"
	"github.com/google/uuid"
)

func Make(fileName string, blockSize, blocks int) error {
	if blockSize != 1024 && blockSize != 2048 && blockSize != 4096 {
		return errors.New("invalid blockSize specified")
	}
	filesystemDevice, err := device.Make(fileName, int64(blockSize*blocks))
	if err != nil {
		return err
	}

	currentTime := time.Now().UnixNano()
	volumeId := uuid.New().String()
	firstSuperblock, err := superblock.New(
		1024,
		filesystemDevice,
		0,
		blockSize,
		blocks,
		currentTime,
		volumeId,
	)
	fmt.Println(firstSuperblock)
	if err != nil {
		return err
	}
	return nil
}
