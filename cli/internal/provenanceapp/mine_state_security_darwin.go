package provenanceapp

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

func checkpointSecurity(path string, info os.FileInfo) ([]byte, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("file ownership unavailable")
	}
	acl, err := checkpointDarwinACL(path)
	if err != nil {
		return nil, err
	}
	metadata := make([]byte, 12+len(acl))
	binary.LittleEndian.PutUint32(metadata, stat.Uid)
	binary.LittleEndian.PutUint32(metadata[4:], stat.Gid)
	binary.LittleEndian.PutUint32(metadata[8:], stat.Flags)
	copy(metadata[12:], acl)
	return metadata, nil
}

// getattrlist returns a length and an attrreference followed by the opaque ACL.
// REPORT_FULLSIZE prevents silent truncation; NOFOLLOW preserves the path check.
// Native ABI: Apple sys/attr.h and bsd/vfs/vfs_attrlist.c. No cgo is required.
func checkpointDarwinACL(path string) ([]byte, error) {
	name, err := syscall.BytePtrFromString(path)
	if err != nil {
		return nil, err
	}
	attrs := struct {
		Count, Reserved                       uint16
		Common, Volume, Directory, File, Fork uint32
	}{Count: 5, Common: 0x00400000} // ATTR_CMN_EXTENDED_SECURITY
	read := func(buf []byte) error {
		_, _, errno := syscall.Syscall6(syscall.SYS_GETATTRLIST,
			uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&attrs)),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 5, 0)
		runtime.KeepAlive(name)
		runtime.KeepAlive(attrs)
		if errno != 0 {
			return errno
		}
		return nil
	}
	buf := make([]byte, 12)
	if err := read(buf); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(buf)
	if size < 12 || size > 65536 {
		return nil, fmt.Errorf("unsupported ACL size %d", size)
	}
	if size > uint32(len(buf)) {
		buf = make([]byte, size)
		if err := read(buf); err != nil {
			return nil, err
		}
	}
	if binary.LittleEndian.Uint32(buf) != uint32(len(buf)) {
		return nil, fmt.Errorf("ACL changed during inspection")
	}
	return buf, nil
}
