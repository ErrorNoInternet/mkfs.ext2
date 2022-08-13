package superblock

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/ErrorNoInternet/mkfs.ext2/device"
	binary_pack "github.com/roman-kachanovsky/go-binary-pack/binary-pack"
)

type Superblock struct {
	BgNum                      int
	FirstInodeIndex            int
	InodeSize                  int
	NumInodesPerGroup          int
	NumResBlocks               int
	NumBlocksPerGroup          int
	NumBlockGroups             int
	BlockSize                  int
	NumBlocks                  int
	FirstBlockId               int
	BgdtBlocks                 int
	InodeTableBlocks           int
	NumFreeBlocks              int
	LastBgId                   int
	NumInodes                  int
	NumFreeInodes              int
	NumFragsPerGroup           int
	NumMountsSinceCheck        int
	NumMountsMax               int
	MagicNum                   int
	State                      int
	ErrorAction                int
	RevMinor                   int
	CreatorOs                  int
	RevLevel                   int
	DefResUid                  int
	DefResGid                  int
	FeaturesCompatible         int
	FeaturesIncompatible       int
	FeaturesReadOnlyCompatible int
	LogBlockSize               int
	LogFragSize                int
	TimeLastWrite              int64
	TimeLastMount              int64
	TimeLastCheck              int64
	TimeBetweenCheck           int64
	LastMountPath              string
	VolName                    string
	VolumeId                   [16]byte
	CopyBlockGroupIds          []int
}

func New(
	byteOffset int64,
	filesystemDevice *device.Device,
	bgNum int,
	blockSize int,
	numBlocks int,
	currentTime int64,
	volumeId [16]byte,
) (*Superblock, error) {
	superblock := &Superblock{
		BgNum:     bgNum,
		BlockSize: blockSize,
		NumBlocks: numBlocks,
		VolumeId:  volumeId,
	}

	superblock.FirstInodeIndex = 11
	superblock.InodeSize = 128
	superblock.NumInodesPerGroup = superblock.BlockSize * 8
	superblock.NumResBlocks = int(float64(superblock.NumBlocks) * 0.05)
	superblock.NumBlocksPerGroup = superblock.BlockSize * 8
	superblock.NumBlockGroups = int(math.Ceil(float64(superblock.NumBlocks) / float64(superblock.NumBlocksPerGroup)))
	fmt.Println(superblock.NumBlockGroups)

	if superblock.BlockSize > 1024 {
		superblock.FirstBlockId = 0
	} else {
		superblock.FirstBlockId = 1
	}

	superblock.CopyBlockGroupIds = []int{}
	if superblock.NumBlockGroups > 1 {
		superblock.CopyBlockGroupIds = append(superblock.CopyBlockGroupIds, 1)
		last3 := 3
		for last3 < superblock.NumBlockGroups {
			superblock.CopyBlockGroupIds = append(superblock.CopyBlockGroupIds, last3)
			last3 *= 3
		}
		last5 := 5
		for last5 < superblock.NumBlockGroups {
			superblock.CopyBlockGroupIds = append(superblock.CopyBlockGroupIds, last5)
			last5 *= 5
		}
		last7 := 7
		for last7 < superblock.NumBlockGroups {
			superblock.CopyBlockGroupIds = append(superblock.CopyBlockGroupIds, last7)
			last7 *= 7
		}
	}
	fmt.Println(superblock.CopyBlockGroupIds)

	superblock.BgdtBlocks = int(math.Ceil(float64(superblock.NumBlockGroups*32) / float64(superblock.BlockSize)))
	superblock.InodeTableBlocks = int(math.Ceil(float64(superblock.NumInodesPerGroup*superblock.InodeSize) / float64(superblock.BlockSize)))
	superblock.NumFreeBlocks = (superblock.NumBlocks - superblock.FirstBlockId - superblock.InodeTableBlocks*superblock.NumBlockGroups - 2*superblock.NumBlockGroups - (1+superblock.BgdtBlocks)*(len(superblock.CopyBlockGroupIds)+1))
	fmt.Println("a", superblock.BgdtBlocks, superblock.InodeTableBlocks, superblock.NumFreeBlocks)

	superblock.LastBgId = superblock.NumBlockGroups - 1
	overhead := 2 + superblock.InodeTableBlocks
	targetIndex := -1
	for index, item := range superblock.CopyBlockGroupIds {
		if superblock.LastBgId == item {
			targetIndex = index
		}
	}
	if targetIndex != -1 {
		overhead += (1 + superblock.BgdtBlocks)
	}
	if overhead > superblock.NumBlocks-(superblock.LastBgId*superblock.NumBlocksPerGroup+superblock.FirstBlockId) {
		if targetIndex != -1 {
			fmt.Println(len(superblock.CopyBlockGroupIds))
			superblock.CopyBlockGroupIds = append(
				superblock.CopyBlockGroupIds[:targetIndex],
				superblock.CopyBlockGroupIds[targetIndex+1:]...,
			)
			fmt.Println(len(superblock.CopyBlockGroupIds))
		}
		superblock.NumBlockGroups -= 1
		superblock.NumBlocks = superblock.NumBlockGroups * superblock.NumBlocksPerGroup
		superblock.BgdtBlocks = int(math.Ceil(float64(superblock.NumBlockGroups*32) / float64(superblock.BlockSize)))
		superblock.NumFreeBlocks = (superblock.NumBlocks - superblock.FirstBlockId - superblock.InodeTableBlocks*superblock.NumBlockGroups - 2*superblock.NumBlockGroups - (1+superblock.BgdtBlocks)*(len(superblock.CopyBlockGroupIds)+1))
		fmt.Println("FREE", superblock.NumFreeBlocks)
	}
	if superblock.NumFreeBlocks < 10 {
		return superblock, errors.New("not enough blocks specified")
	}

	superblock.NumInodes = superblock.NumInodesPerGroup * superblock.NumBlockGroups
	superblock.NumFreeInodes = superblock.NumInodes - (superblock.FirstInodeIndex - 1)

	superblock.LogBlockSize = superblock.BlockSize >> 11
	superblock.LogFragSize = superblock.BlockSize >> 11
	superblock.NumFragsPerGroup = superblock.NumBlocksPerGroup
	if superblock.NumBlocks < superblock.NumBlocksPerGroup {
		superblock.NumBlocksPerGroup = superblock.NumBlocks
		superblock.NumFragsPerGroup = superblock.NumBlocks
	}
	superblock.TimeLastMount = currentTime
	superblock.TimeLastWrite = currentTime
	superblock.NumMountsSinceCheck = 0
	superblock.NumMountsMax = 25
	superblock.MagicNum = 0xEF53
	superblock.State = 1
	superblock.ErrorAction = 1
	superblock.RevMinor = 0
	superblock.TimeLastCheck = currentTime
	superblock.TimeBetweenCheck = 15552000
	superblock.CreatorOs = 0
	superblock.RevLevel = 1
	superblock.DefResUid = 0
	superblock.DefResGid = 0
	superblock.FeaturesCompatible = 0
	superblock.FeaturesIncompatible = 2
	superblock.FeaturesReadOnlyCompatible = 1

	buffer := make([]byte, 1)
	binary.PutVarint(buffer, 0)
	superblock.VolName = string(buffer)
	superblock.LastMountPath = "/" + string(buffer)

	format := []string{"I", "I", "I", "I", "I", "I", "I", "i", "I", "I", "I", "I", "I", "H", "H", "H", "H", "H", "H", "I", "I", "I", "I", "H", "H", "I", "H", "H", "I", "I", "I", "16s", "16s", "64s"}
	values := []interface{}{superblock.NumInodes, superblock.NumBlocks, superblock.NumResBlocks, superblock.NumFreeBlocks, superblock.NumFreeInodes,
		superblock.FirstBlockId, superblock.LogBlockSize, superblock.LogFragSize, superblock.NumBlocksPerGroup, superblock.NumFragsPerGroup,
		superblock.NumInodesPerGroup, int(superblock.TimeLastMount), int(superblock.TimeLastWrite), superblock.NumMountsSinceCheck, superblock.NumMountsMax,
		superblock.MagicNum, superblock.State, superblock.ErrorAction, superblock.RevMinor, int(superblock.TimeLastCheck), int(superblock.TimeBetweenCheck), superblock.CreatorOs,
		superblock.RevLevel, superblock.DefResUid, superblock.DefResGid, superblock.FirstInodeIndex, superblock.InodeSize, superblock.BgNum, superblock.FeaturesCompatible,
		superblock.FeaturesIncompatible, superblock.FeaturesReadOnlyCompatible, string(superblock.VolumeId[:]), superblock.VolName, superblock.LastMountPath}
	bp := new(binary_pack.BinaryPack)
	sbBytes, err := bp.Pack(format, values)
	if err != nil {
		return superblock, errors.New("unable to pack bytes: " + err.Error())
	}
	emptyBytes := []byte("")
	for i := 0; i < 824; i++ {
		emptyBytes = binary.AppendVarint(emptyBytes, 0)
	}
	newBytes := bytes.Join([][]byte{sbBytes, emptyBytes}, []byte(""))
	filesystemDevice.Write(byteOffset, newBytes)
	return superblock, nil
}
