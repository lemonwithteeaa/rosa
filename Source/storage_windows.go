//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

var pMoveFileExW = kernel32.NewProc("MoveFileExW")

const (
	MOVEFILE_REPLACE_EXISTING = 0x00000001
	MOVEFILE_WRITE_THROUGH    = 0x00000008
)

func replaceFileAtomic(tmp, path string) error {
	r, _, e := pMoveFileExW.Call(
		uintptr(unsafe.Pointer(p(tmp))),
		uintptr(unsafe.Pointer(p(path))),
		MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH,
	)
	if r == 0 {
		return fmt.Errorf("атомарная замена файла не выполнена: %v", e)
	}
	return nil
}

// atomicWriteFile writes a complete file to a sibling temporary file, flushes it,
// and then replaces the destination with a write-through Windows rename. Readers
// therefore see either the old complete version or the new complete version.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if err = f.Chmod(perm); err != nil {
		_ = f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = replaceFileAtomic(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}
