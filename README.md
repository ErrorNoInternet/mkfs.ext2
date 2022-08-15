# mkfs.ext2
A mkfs.ext2 implementation in Go

## Usage
```
# Print all flags
mkfs.ext2 -help

# Create file with filesystem (automatically determine blocks, if file.ext2 doesn't exist then it's 100 MB)
mkfs.ext2 -device file.ext2

# Create filesystem on real device (200 MB)
mkfs.ext2 -device /dev/sdX -blocks 51200

# Create filesystem on real device (400 MB)
mkfs.ext2 -device /dev/sdX -blocks 102400
```

## Objects
- [x] Superblock
- [x] Device
- [x] Bgdt
  - [x] BgdtEntry
- [x] Inode (partial, only used to create root inode)
- [x] Filesystem

