package main

import (
	"fmt"

	"github.com/ErrorNoInternet/mkfs.ext2/filesystem"
)

func main() {
	fmt.Println(filesystem.Make("test.ext2", 1024, 32768))
}
