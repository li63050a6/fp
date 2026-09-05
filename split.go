package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
)

var ioBufSize = 4 << 20 // 4MB 读写缓冲区（测试可覆盖）

func runSplit(args []string) error {
	fs := flag.NewFlagSet("sp", flag.ContinueOnError)
	sizeOpt := fs.String("size", "", "按分片大小拆分，支持单位 K/M/G（如 100M）")
	nOpt := fs.Int("n", 0, "按分片数量拆分")
	outOpt := fs.String("out", "", "输出文件夹名（默认：<文件名>.parts）")
	manifestOpt := fs.String("manifest", defaultManifestName, "描述文件名（默认：manifest.json）")
	prefixOpt := fs.String("prefix", "", "分片文件名前缀（默认：原文件名）")
	padOpt := fs.Int("pad", 4, "分片序号位数（默认：4）")
	hashOpt := fs.Bool("hash", true, "计算每片 SHA-256")
	noHashOpt := fs.Bool("no-hash", false, "不计算每片 SHA-256")
	fs.Usage = func() { printSplitUsage(fs) }
	if err := fs.Parse(normalizeArgs(args, map[string]bool{
		"size": true, "n": true, "out": true, "manifest": true, "prefix": true, "pad": true,
	})); err != nil {
		return handleParseError(err)
	}

	input := fs.Arg(0)
	if input == "" {
		return errors.New("缺少输入文件，用法：fp sp <文件> [选项]")
	}
	if *sizeOpt != "" && *nOpt > 0 {
		return errors.New("-size 与 -n 不能同时使用")
	}
	if *sizeOpt == "" && *nOpt <= 0 {
		return errors.New("必须用 -size 或 -n 指定分片方式")
	}
	if *padOpt < 1 {
		return errors.New("-pad 必须大于 0")
	}

	in, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("无法打开输入文件：%v", err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	total := info.Size()

	if *nOpt > 0 {
		chunkCount := int64(*nOpt)
		chunkSize := ceilDiv(total, chunkCount)
		if err := doSplit(input, total, chunkCount, chunkSize, in,
			*outOpt, *manifestOpt, *prefixOpt, *padOpt, *hashOpt && !*noHashOpt, "count"); err != nil {
			return err
		}
		return nil
	}

	chunkSize, err := parseSize(*sizeOpt)
	if err != nil {
		return err
	}
	if chunkSize <= 0 {
		return errors.New("分片大小必须大于 0")
	}
	chunkCount := ceilDiv(total, chunkSize)
	if chunkCount == 0 {
		chunkCount = 1
	}
	return doSplit(input, total, chunkCount, chunkSize, in,
		*outOpt, *manifestOpt, *prefixOpt, *padOpt, *hashOpt && !*noHashOpt, "size")
}

func doSplit(input string, total, chunkCount, chunkSize int64, in io.Reader, outOpt, manifestName, prefixOpt string, pad int, needChunkHash bool, mode string) error {

	sizes := make([]int64, chunkCount)
	rem := total
	for i := range sizes {
		if chunkSize > 0 && rem > chunkSize {
			sizes[i] = chunkSize
			rem -= chunkSize
		} else {
			sizes[i] = rem
			rem = 0
		}
	}

	outDir := outOpt
	if outDir == "" {
		outDir = input + ".parts"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("无法创建输出文件夹 %s：%v", outDir, err)
	}

	prefix := prefixOpt
	if prefix == "" {
		prefix = filepath.Base(input)
	}

	stat, err := getFileStat(input)
	if err != nil {
		return err
	}

	overall := sha256.New()
	chunks := make([]ChunkEntry, 0, chunkCount)
	reader := bufio.NewReaderSize(in, ioBufSize)

	for i, sz := range sizes {
		name := fmt.Sprintf("%s.part%0*d", prefix, pad, i+1)
		path := filepath.Join(outDir, name)
		dest, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("无法创建分片 %s：%v", path, err)
		}

		var chunkHasher hash.Hash
		if needChunkHash {
			chunkHasher = sha256.New()
		}

		written := int64(0)
		buf := make([]byte, ioBufSize)
		for written < sz {
			want := int64(len(buf))
			if sz-written < want {
				want = sz - written
			}
			n, rerr := reader.Read(buf[:want])
			if n > 0 {
				if _, werr := dest.Write(buf[:n]); werr != nil {
					dest.Close()
					return fmt.Errorf("写入分片 %s 失败：%v", name, werr)
				}
				overall.Write(buf[:n])
				if chunkHasher != nil {
					chunkHasher.Write(buf[:n])
				}
				written += int64(n)
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				dest.Close()
				return fmt.Errorf("读取输入文件失败：%v", rerr)
			}
		}
		if cerr := dest.Close(); cerr != nil {
			return cerr
		}
		if written != sz {
			return fmt.Errorf("输入文件在分片 %s 处意外提前结束（期望 %d 字节，实读 %d）", name, sz, written)
		}

		entry := ChunkEntry{Name: name, Size: sz}
		if chunkHasher != nil {
			entry.SHA256 = hex.EncodeToString(chunkHasher.Sum(nil))
		}
		chunks = append(chunks, entry)
	}

	m := &Manifest{
		Version:        manifestVersion,
		OriginalName:   filepath.Base(input),
		OriginalSize:   total,
		OriginalSHA256: hex.EncodeToString(overall.Sum(nil)),
		Stat:           stat,
		SplitMode:      mode,
		ChunkSize:      chunkSize,
		ChunkCount:     len(chunks),
		Chunks:         chunks,
	}
	if err := saveManifest(outDir, manifestName, m); err != nil {
		return fmt.Errorf("写入描述文件失败：%v", err)
	}

	fmt.Printf("拆分完成：\n")
	fmt.Printf("  原文件：%s（%d 字节）\n", m.OriginalName, m.OriginalSize)
	fmt.Printf("  分片数：%d 片\n", len(chunks))
	fmt.Printf("  分片大小：%d 字节\n", chunkSize)
	fmt.Printf("  输出文件夹：%s\n", outDir)
	fmt.Printf("  描述文件：%s\n", filepath.Join(outDir, manifestName))
	return nil
}

func printSplitUsage(fs *flag.FlagSet) {
	fmt.Fprint(fs.Output(), `用法：fp sp <文件> [选项]

选项：
`)
	fs.PrintDefaults()
	fmt.Fprint(fs.Output(), `
示例：
  fp sp bigfile.iso -size 100M
  fp sp bigfile.iso -n 10
`)
}
