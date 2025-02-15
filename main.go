package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ErrorNoInternet/mkfs.ext2/filesystem"
)

func main() {
	var devicePath string
	var blockSize, blocks int
	flag.StringVar(&devicePath, "device", "", "The device you want to create a filesystem on")
	flag.IntVar(&blockSize, "blockSize", 4096, "The size (in bytes) of each block in the filesystem")
	flag.IntVar(&blocks, "blocks", 0, "The amount of blocks to create in the filesystem")
	flag.Parse()

	if devicePath == "" {
		flag.Usage()
		return
	}

	if blocks == 0 {
		deviceInformation, err := os.Stat(devicePath)
		if err != nil {
			blocks = 1024 * 256
		} else if deviceInformation.Size() == 0 {
			device, err := os.Open(devicePath)
			if err != nil {
				fmt.Printf("unable to open file: %v\n", err)
				return
			}
			position, err := device.Seek(0, io.SeekEnd)
			if err != nil {
				fmt.Printf("unable to seek to end of file: %v\n", err)
				return
			}
			blocks = int(position / int64(blockSize))
		} else {
			blocks = int(deviceInformation.Size() / int64(blockSize))
		}
	}

	file, err := os.Create(devicePath)
	if err != nil {
		fmt.Printf("unable to create file: %v\n", err)
		return
	}
	err = filesystem.Make(file, blockSize, blocks)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
