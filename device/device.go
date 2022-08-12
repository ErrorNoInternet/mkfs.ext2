package device

import (
	"encoding/binary"
	"errors"
	"os"
)

type Device struct {
	FileName  string
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
	device.ImageFile.Sync()
}

func Make(fileName string, bytes int64) (*Device, error) {
	_, err := os.Stat(fileName)
	if err == nil {
		return nil, errors.New("specified image file already exists")
	}
	file, err := os.Create(fileName)
	if err != nil {
		return nil, errors.New("unable to create image file: " + err.Error())
	}

	file.Seek(bytes-1, 0)
	buffer := make([]byte, 1)
	binary.PutVarint(buffer, 0)
	file.Write(buffer)
	return &Device{
		FileName:  fileName,
		ImageFile: file,
		Mounted:   true,
	}, nil
}
