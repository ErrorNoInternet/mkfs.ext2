# mkfs.ext2
A mkfs.ext2 implementation in Go

## Usage
```
# Print all flags
mkfs.ext2 -help

# Create a file with a filesystem
# (automatically determine blocks, if file.ext2 doesn't exist then it's 100 MB)
mkfs.ext2 -device file.ext2

# Create a filesystem on a real device (automatically determine blocks)
mkfs.ext2 -device /dev/sdX
```

## Objects
- [x] Superblock
- [x] Device
- [x] Bgdt
  - [x] BgdtEntry
- [x] Inode (partial, only used to create root inode)
- [x] Filesystem

