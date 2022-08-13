package superblock

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/ErrorNoInternet/mkfs.ext2/device"
	binary_pack "github.com/roman-kachanovsky/go-binary-pack/binary-pack"
)

type Superblock struct {
	BgNum                      int
	NumFreeBlocks              int
	NumFreeInodes              int
	NumMountsSinceCheck        int
	NumInodesPerGroup          int
	NumResBlocks               int
	NumBlocksPerGroup          int
	NumBlockGroups             int
	NumBlocks                  int
	NumInodes                  int
	NumFragsPerGroup           int
	NumMountsMax               int
	FirstInodeIndex            int
	FirstBlockId               int
	LastBgId                   int
	InodeSize                  int
	BlockSize                  int
	BgdtBlocks                 int
	InodeTableBlocks           int
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
	TimeLastMount              int64
	TimeLastWrite              int64
	TimeLastCheck              int64
	TimeBetweenCheck           int64
	SaveCopies                 bool
	LastMountPath              string
	VolumeName                 string
	VolumeId                   [16]byte
	CopyBlockGroupIds          []int
	Device                     *device.Device
}

func (superblock *Superblock) SetNumFreeBlocks(numFreeBlocks int) error {
	superblock.NumFreeBlocks = numFreeBlocks
	bp := new(binary_pack.BinaryPack)
	bytes, err := bp.Pack([]string{"I"}, []interface{}{numFreeBlocks})
	if err != nil {
		return err
	}
	superblock.WriteData(12, bytes)
	return nil
}

func (superblock *Superblock) SetNumFreeInodes(numFreeInodes int) error {
	superblock.NumFreeInodes = numFreeInodes
	bp := new(binary_pack.BinaryPack)
	bytes, err := bp.Pack([]string{"I"}, []interface{}{numFreeInodes})
	if err != nil {
		return err
	}
	superblock.WriteData(16, bytes)
	return nil
}

func (superblock *Superblock) SetTimeLastMount(timeLastMount int64) error {
	superblock.TimeLastMount = timeLastMount
	bp := new(binary_pack.BinaryPack)
	bytes, err := bp.Pack([]string{"I"}, []interface{}{timeLastMount})
	if err != nil {
		return err
	}
	superblock.WriteData(44, bytes)
	return nil
}

func (superblock *Superblock) SetTimeLastWrite(timeLastWrite int64) error {
	superblock.TimeLastWrite = timeLastWrite
	bp := new(binary_pack.BinaryPack)
	bytes, err := bp.Pack([]string{"I"}, []interface{}{timeLastWrite})
	if err != nil {
		return err
	}
	superblock.WriteData(48, bytes)
	return nil
}

func (superblock *Superblock) SetNumMountsSinceCheck(numMountsSinceCheck int) error {
	superblock.NumMountsSinceCheck = numMountsSinceCheck
	bp := new(binary_pack.BinaryPack)
	bytes, err := bp.Pack([]string{"I"}, []interface{}{numMountsSinceCheck})
	if err != nil {
		return err
	}
	superblock.WriteData(52, bytes)
	return nil
}

func (superblock *Superblock) SetVolumeName(volumeName string) error {
	superblock.VolumeName = volumeName
	bp := new(binary_pack.BinaryPack)
	volumeNameBytes, err := bp.Pack([]string{fmt.Sprintf("%vs", len(volumeName))}, []interface{}{volumeName})
	if err != nil {
		return err
	}
	buffer := make([]byte, 1)
	binary.PutVarint(buffer, 0)
	superblock.WriteData(120, bytes.Join([][]byte{volumeNameBytes, buffer}, []byte("")))
	return nil
}

func (superblock *Superblock) WriteData(offset int64, data []byte) {
	for _, groupId := range superblock.CopyBlockGroupIds {
		sbStart := 1024 + groupId*superblock.NumBlocksPerGroup*superblock.BlockSize
		superblock.Device.Write(int64(sbStart)+offset, data)
		if !superblock.SaveCopies {
			break
		}
	}
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
		Device:    filesystemDevice,
	}

	superblock.FirstInodeIndex = 11
	superblock.InodeSize = 128
	superblock.NumInodesPerGroup = superblock.BlockSize * 8
	superblock.NumResBlocks = int(float64(superblock.NumBlocks) * 0.05)
	superblock.NumBlocksPerGroup = superblock.BlockSize * 8
	superblock.NumBlockGroups = int(math.Ceil(float64(superblock.NumBlocks) / float64(superblock.NumBlocksPerGroup)))

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

	superblock.BgdtBlocks = int(math.Ceil(float64(superblock.NumBlockGroups*32) / float64(superblock.BlockSize)))
	superblock.InodeTableBlocks = int(math.Ceil(float64(superblock.NumInodesPerGroup*superblock.InodeSize) / float64(superblock.BlockSize)))
	superblock.SetNumFreeBlocks(superblock.NumBlocks - superblock.FirstBlockId - superblock.InodeTableBlocks*superblock.NumBlockGroups - 2*superblock.NumBlockGroups - (1+superblock.BgdtBlocks)*(len(superblock.CopyBlockGroupIds)+1))

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
			superblock.CopyBlockGroupIds = append(
				superblock.CopyBlockGroupIds[:targetIndex],
				superblock.CopyBlockGroupIds[targetIndex+1:]...,
			)
		}
		superblock.NumBlockGroups -= 1
		superblock.NumBlocks = superblock.NumBlockGroups * superblock.NumBlocksPerGroup
		superblock.BgdtBlocks = int(math.Ceil(float64(superblock.NumBlockGroups*32) / float64(superblock.BlockSize)))
		superblock.SetNumFreeBlocks(superblock.NumBlocks - superblock.FirstBlockId - superblock.InodeTableBlocks*superblock.NumBlockGroups - 2*superblock.NumBlockGroups - (1+superblock.BgdtBlocks)*(len(superblock.CopyBlockGroupIds)+1))
	}
	if superblock.NumFreeBlocks < 10 {
		return superblock, errors.New("not enough blocks specified")
	}

	superblock.NumInodes = superblock.NumInodesPerGroup * superblock.NumBlockGroups
	superblock.SetNumFreeInodes(superblock.NumInodes - (superblock.FirstInodeIndex - 1))

	superblock.LogBlockSize = superblock.BlockSize >> 11
	superblock.LogFragSize = superblock.BlockSize >> 11
	superblock.NumFragsPerGroup = superblock.NumBlocksPerGroup
	if superblock.NumBlocks < superblock.NumBlocksPerGroup {
		superblock.NumBlocksPerGroup = superblock.NumBlocks
		superblock.NumFragsPerGroup = superblock.NumBlocks
	}
	superblock.SetTimeLastMount(currentTime)
	superblock.SetTimeLastWrite(currentTime)
	superblock.SetNumMountsSinceCheck(0)
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
	superblock.SetVolumeName(string(buffer))
	superblock.LastMountPath = "/" + string(buffer)

	format := []string{"I", "I", "I", "I", "I", "I", "I", "i", "I", "I", "I", "I", "I", "H", "H", "H", "H", "H", "H", "I", "I", "I", "I", "H", "H", "I", "H", "H", "I", "I", "I", "16s", "16s", "64s"}
	values := []interface{}{superblock.NumInodes, superblock.NumBlocks, superblock.NumResBlocks, superblock.NumFreeBlocks, superblock.NumFreeInodes,
		superblock.FirstBlockId, superblock.LogBlockSize, superblock.LogFragSize, superblock.NumBlocksPerGroup, superblock.NumFragsPerGroup,
		superblock.NumInodesPerGroup, int(superblock.TimeLastMount), int(superblock.TimeLastWrite), superblock.NumMountsSinceCheck, superblock.NumMountsMax,
		superblock.MagicNum, superblock.State, superblock.ErrorAction, superblock.RevMinor, int(superblock.TimeLastCheck), int(superblock.TimeBetweenCheck), superblock.CreatorOs,
		superblock.RevLevel, superblock.DefResUid, superblock.DefResGid, superblock.FirstInodeIndex, superblock.InodeSize, superblock.BgNum, superblock.FeaturesCompatible,
		superblock.FeaturesIncompatible, superblock.FeaturesReadOnlyCompatible, string(superblock.VolumeId[:]), superblock.VolumeName, superblock.LastMountPath}
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

	superblock.CopyBlockGroupIds = append(superblock.CopyBlockGroupIds, 0)
	sort.Ints(superblock.CopyBlockGroupIds)

	return superblock, nil
}
