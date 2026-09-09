package provenanceapp

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"
)

func checkpointSecurity(path string, info os.FileInfo) ([]byte, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("file ownership unavailable")
	}
	size, err := syscall.Listxattr(path, nil)
	if err != nil {
		return nil, err
	}
	if size < 0 || size > 65536 {
		return nil, fmt.Errorf("unsupported security attribute list size %d", size)
	}
	names := make([]byte, size)
	n, err := syscall.Listxattr(path, names)
	if err != nil {
		return nil, fmt.Errorf("read security attribute list: %w", err)
	}
	if n != size {
		return nil, fmt.Errorf("security attribute list changed during inspection")
	}
	keys := strings.Split(string(names), "\x00")
	sort.Strings(keys)
	metadata := make([]byte, 8)
	binary.LittleEndian.PutUint32(metadata, stat.Uid)
	binary.LittleEndian.PutUint32(metadata[4:], stat.Gid)
	for _, key := range keys {
		// Covers POSIX/NFS ACLs and security labels; user data is not permissions.
		if key == "" || strings.HasPrefix(key, "user.") {
			continue
		}
		value, err := checkpointSecurityAttribute(path, key)
		if err != nil {
			return nil, err
		}
		metadata = binary.LittleEndian.AppendUint32(metadata, uint32(len(key)))
		metadata = append(metadata, key...)
		metadata = binary.LittleEndian.AppendUint32(metadata, uint32(len(value)))
		metadata = append(metadata, value...)
	}
	return metadata, nil
}

func checkpointSecurityAttribute(path, key string) ([]byte, error) {
	size, err := syscall.Getxattr(path, key, nil)
	if err != nil {
		return nil, err
	}
	if size < 0 || size > 65536 {
		return nil, fmt.Errorf("unsupported security attribute size %d", size)
	}
	value := make([]byte, size)
	n, err := syscall.Getxattr(path, key, value)
	if err != nil {
		return nil, fmt.Errorf("read security attribute: %w", err)
	}
	if n != size {
		return nil, fmt.Errorf("security attribute changed during inspection")
	}
	return value, nil
}
