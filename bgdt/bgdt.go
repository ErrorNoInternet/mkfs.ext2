package bdgt

import (
	"github.com/ErrorNoInternet/mkfs.ext2/device"
	"github.com/ErrorNoInternet/mkfs.ext2/superblock"
)

type Bgdt struct {
}

func Make(bgNumCopy int, targetSuperblock *superblock.Superblock, filesystemDevice *device.Device) {
	startPos := (bgNumCopy*targetSuperblock.NumBlocksPerGroup + superblcok.FirstDataBlockId + 1) * targetSuperblock.BlockSize
}
