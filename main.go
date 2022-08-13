package main

import (
	"flag"
	"fmt"

	"github.com/ErrorNoInternet/mkfs.ext2/filesystem"
)

func main() {
	var devicePath string
	var blockSize, blocks int
	flag.StringVar(&devicePath, "device", "", "The device you want to create a filesystem on")
	flag.IntVar(&blockSize, "blockSize", 1024, "The block size of each block in the filesystem")
	flag.IntVar(&blocks, "blocks", 32768, "The amount of blocks to create in the filesystem")
	flag.Parse()
	if devicePath == "" {
		flag.Usage()
	}

	err := filesystem.Make(devicePath, blockSize, blocks)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
