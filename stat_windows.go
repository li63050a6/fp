//go:build windows

package main

import (
	"os"
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
	return st, nil
}

func applyFileStat(path string, st StatInfo, onChownErr func(error)) error {
	if err := os.Chmod(path, os.FileMode(st.Mode)); err != nil {
		return err
	}
	t := time.Unix(st.MtimeNs/1e9, st.MtimeNs%1e9)
	return os.Chtimes(path, t, t)
}
