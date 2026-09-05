package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const manifestVersion = 1
const defaultManifestName = "manifest.json"

// ChunkEntry 描述单个分片。
type ChunkEntry struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

// StatInfo 记录原文件的 stat 元数据，用于合并时还原。
type StatInfo struct {
	Mode    uint32 `json:"mode"`
	MtimeNs int64  `json:"mtime_ns"`
	UID     int    `json:"uid,omitempty"`
	GID     int    `json:"gid,omitempty"`
}

// Manifest 描述文件结构。
type Manifest struct {
	Version        int          `json:"version"`
	OriginalName   string       `json:"original_name"`
	OriginalSize   int64        `json:"original_size"`
	OriginalSHA256 string       `json:"original_sha256"`
	Stat           StatInfo     `json:"stat"`
	SplitMode      string       `json:"split_mode"`
	ChunkSize      int64        `json:"chunk_size"`
	ChunkCount     int          `json:"chunk_count"`
	Chunks         []ChunkEntry `json:"chunks"`
}

func (m *Manifest) validate() error {
	if m.Version != manifestVersion {
		return fmt.Errorf("不支持的 manifest 版本：%d", m.Version)
	}
	if m.OriginalName == "" {
		return errors.New("manifest 缺少原文件名")
	}
	if m.ChunkCount != len(m.Chunks) {
		return errors.New("manifest 的分片数与实际不符")
	}
	if len(m.Chunks) == 0 {
		return errors.New("manifest 中没有分片记录")
	}
	return nil
}

func saveManifest(dir, name string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, name), data, 0o644)
}

func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败：%v", err)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}
