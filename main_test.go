package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"100", 100, true},
		{"100B", 100, true},
		{"1K", 1024, true},
		{"512k", 512 * 1024, true},
		{"100M", 100 * 1024 * 1024, true},
		{"2G", 2 << 30, true},
		{"1.5K", 1536, true},
		{"", 0, false},
		{"abc", 0, false},
		{"10X", 0, false},
		{"-5", 0, false},
	}
	for _, c := range cases {
		got, err := parseSize(c.in)
		if c.ok && err != nil {
			t.Errorf("parseSize(%q) 意外错误：%v", c.in, err)
		}
		if !c.ok && err == nil {
			t.Errorf("parseSize(%q) 应报错，实际返回 %d", c.in, got)
		}
		if c.ok && got != c.want {
			t.Errorf("parseSize(%q) = %d，期望 %d", c.in, got, c.want)
		}
	}
}

func TestSplitBySize(t *testing.T) {
	runRoundTrip(t, "size", []string{"-size", "10K"})
}

func TestSplitByCount(t *testing.T) {
	runRoundTrip(t, "count", []string{"-n", "7"})
}

func TestSplitSingleChunk(t *testing.T) {
	runRoundTrip(t, "size-single", []string{"-size", "1G"})
}

func TestRuntimeAllocations(t *testing.T) {
	ioBufSize = 1024
	defer func() { ioBufSize = 4 << 20 }()
	runRoundTrip(t, "small-buf", []string{"-size", "3K"})
}

func TestMergeDetectCorruptChunk(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(src, randBytes(5000), 0o644); err != nil {
		t.Fatal(err)
	}

	partDir, err := runSplitToDir(t, src, []string{"-size", "1K"})
	if err != nil {
		t.Fatal(err)
	}

	parts, err := os.ReadDir(partDir)
	if err != nil {
		t.Fatal(err)
	}
	var first string
	for _, p := range parts {
		if filepath.Ext(p.Name()) == ".json" {
			continue
		}
		first = p.Name()
		break
	}
	if first == "" {
		t.Fatal("未找到分片文件")
	}

	corrupt := filepath.Join(partDir, first)
	data, _ := os.ReadFile(corrupt)
	data[0] ^= 0xFF
	if err := os.WriteFile(corrupt, data, 0o644); err != nil {
		t.Fatal(err)
	}

	err = runMergeTo(t, filepath.Join(partDir, defaultManifestName), "")
	if err == nil {
		t.Fatal("合并损坏分片应报错")
	}
}

func TestCheckOnly(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(src, randBytes(4096), 0o644); err != nil {
		t.Fatal(err)
	}
	partDir, err := runSplitToDir(t, src, []string{"-size", "1K"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runMergeTo(t, filepath.Join(partDir, defaultManifestName), "-check-only"); err != nil {
		t.Fatalf("check-only 应通过：%v", err)
	}
}

func TestMetadataRestore(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(src, randBytes(3000), 0o755); err != nil {
		t.Fatal(err)
	}
	partDir, err := runSplitToDir(t, src, []string{"-size", "1K"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runMergeTo(t, filepath.Join(partDir, defaultManifestName), "-out", filepath.Join(dir, "restored.bin")); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(dir, "restored.bin")); err != nil {
		t.Fatal(err)
	} else if st.Mode().Perm() != 0o755 {
		t.Errorf("权限未还原：%v", st.Mode().Perm())
	}
}

// 辅助：执行一次 拆分→合并 往返，校验内容、大小与 SHA-256 一致。
func runRoundTrip(t *testing.T, name string, splitArgs []string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "data.bin")
	content := randBytes(12345)
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	partDir, err := runSplitToDir(t, src, splitArgs)
	if err != nil {
		t.Fatalf("%s：拆分失败：%v", name, err)
	}

	out := filepath.Join(dir, "restored.bin")
	if err := runMergeTo(t, filepath.Join(partDir, defaultManifestName), "-out", out); err != nil {
		t.Fatalf("%s：合并失败：%v", name, err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("%s：合并结果与原始内容不一致", name)
	}

	wantHash := sha256.Sum256(content)
	if gotHash := sha256.Sum256(got); gotHash != wantHash {
		t.Fatalf("%s：SHA-256 不一致", name)
	}

	if gotSize := int64(len(got)); gotSize != int64(len(content)) {
		t.Fatalf("%s：大小不一致：%d vs %d", name, gotSize, len(content))
	}
}

func runSplitToDir(t *testing.T, src string, args []string) (string, error) {
	t.Helper()
	if err := runSplit(append(args, src)); err != nil {
		return "", err
	}
	return src + ".parts", nil
}

func runMergeTo(t *testing.T, manifestPath string, extra ...string) error {
	t.Helper()
	return runMerge(append(extra, manifestPath))
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}
