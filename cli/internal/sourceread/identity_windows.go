package sourceread

import (
	"fmt"
	"os"
	"syscall"
)

func nativeIdentity(f *os.File, _ os.FileInfo) string {
	var native syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &native); err != nil {
		return "unavailable"
	}
	return fmt.Sprintf("%d:%d:%d", native.VolumeSerialNumber, native.FileIndexHigh, native.FileIndexLow)
}
