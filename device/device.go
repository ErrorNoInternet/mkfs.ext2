package device

import (
	"encoding/binary"
	"os"
)

type Device struct {
	ImageFile *os.File
	Mounted   bool
}

func (device *Device) Unmount() {
	device.ImageFile.Sync()
	device.ImageFile.Close()
	device.Mounted = false
}

func (device *Device) Write(position int64, bytes []byte) {
	if !device.Mounted {
		panic("device isn't mounted")
	}

	device.ImageFile.Seek(position, 0)
	device.ImageFile.Write(bytes)
}

func (device *Device) Read(position, size int64) []byte {
	if !device.Mounted {
		panic("device isn't mounted")
	}

	data := make([]byte, size)
	device.ImageFile.Seek(position, 0)
	device.ImageFile.Read(data)
	return data
}

func New(file *os.File, bytes int64) (*Device, error) {
	file.Seek(bytes-1, 0)
	buffer := make([]byte, 1)
	binary.PutVarint(buffer, 0)
	_, err := file.Write(buffer)
	if err != nil {
		return nil, err
	}
	return &Device{
		ImageFile: file,
		Mounted:   true,
	}, nil
}
