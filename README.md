# mkfs.ext2
A mkfs.ext2 implementation in Go\
This is still experimental and has issues, use at your own risk!

## Usage
```
# Print all flags
mkfs.ext2 -help

# Create file with filesystem
mkfs.ext2 -device file.ext2

# Create file with filesystem (100 MB)
mkfs.ext2 -device file.ext2 -blocks 102400

# Create filesystem on real device
mkfs.ext2 -device /dev/sdX
```

## Objects
- [x] Superblock
- [x] Device
- [x] Bgdt
  - [x] BgdtEntry
- [x] Inode (partial, only used to create root inode)
- [x] Filesystem

