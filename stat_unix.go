//go:build unix

package main

import (
	"os"
	"syscall"
	"time"
)

func getFileStat(path string) (StatInfo, error) {
	var st StatInfo
	info, err := os.Stat(path)
	if err != nil {
		return st, err
	}
	st.Mode = uint32(info.Mode().Perm())
	st.MtimeNs = info.ModTime().UnixNano()
	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		st.UID = int(sys.Uid)
		st.GID = int(sys.Gid)
	}
	return st, nil
}

func applyFileStat(path string, st StatInfo, onChownErr func(error)) error {
	if err := os.Chmod(path, os.FileMode(st.Mode)); err != nil {
		return err
	}
	t := time.Unix(st.MtimeNs/1e9, st.MtimeNs%1e9)
	if err := os.Chtimes(path, t, t); err != nil {
		return err
	}
	if st.UID == 0 && st.GID == 0 {
		return nil
	}
	if err := os.Chown(path, st.UID, st.GID); err != nil && onChownErr != nil {
		onChownErr(err)
	}
	return nil
}
