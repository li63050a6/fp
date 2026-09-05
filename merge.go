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

func runMerge(args []string) error {
	fs := flag.NewFlagSet("m", flag.ContinueOnError)
	outOpt := fs.String("out", "", "输出文件路径（默认：描述文件中记录的原文件名）")
	forceOpt := fs.Bool("force", false, "目标文件已存在时覆盖")
	checkOnlyOpt := fs.Bool("check-only", false, "只校验分片完整性，不还原文件")
	fs.Usage = func() { printMergeUsage(fs) }
	if err := fs.Parse(normalizeArgs(args, map[string]bool{
		"out": true,
	})); err != nil {
		return handleParseError(err)
	}

	manifestPath := fs.Arg(0)
	if manifestPath == "" {
		return errors.New("缺少描述文件路径，用法：fp m <manifest> [选项]")
	}

	m, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	partDir := filepath.Dir(manifestPath)

	var out io.Writer
	if *checkOnlyOpt {
		out = io.Discard
	} else {
		outPath := *outOpt
		if outPath == "" {
			outPath = m.OriginalName
		}
		if _, err := os.Stat(outPath); err == nil {
			if !*forceOpt {
				if !isTerminal(os.Stdin) {
					return fmt.Errorf("目标文件已存在：%s（如要覆盖请加 -force）", outPath)
				}
				fmt.Printf("目标文件已存在：%s，是否覆盖？(y/N)：", outPath)
				var ans string
				if _, err := fmt.Fscanln(os.Stdin, &ans); err != nil {
					return errors.New("无法读取输入，已取消")
				}
				if ans != "y" && ans != "Y" {
					return errors.New("已取消合并")
				}
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		dest, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("无法创建输出文件 %s：%v", outPath, err)
		}
		defer dest.Close()
		out = dest
	}

	if err := mergeParts(m, partDir, out, needChunkHash(m)); err != nil {
		return err
	}

	if *checkOnlyOpt {
		fmt.Println("校验通过：所有分片完整，与原始文件一致。")
		return nil
	}

	outPath := *outOpt
	if outPath == "" {
		outPath = m.OriginalName
	}
	if f, ok := out.(*os.File); ok {
		if err := f.Sync(); err != nil {
			return err
		}
		if err := applyFileStat(outPath, m.Stat, func(err error) {
			fmt.Fprintf(os.Stderr, "警告：还原属主/属组失败：%v\n", err)
		}); err != nil {
			return fmt.Errorf("还原文件属性失败：%v", err)
		}
	}
	fmt.Printf("合并完成：%s（%d 字节）\n", outPath, m.OriginalSize)
	return nil
}

// needChunkHash 判断是否需要对分片做逐片校验。
func needChunkHash(m *Manifest) bool {
	for _, c := range m.Chunks {
		if c.SHA256 != "" {
			return true
		}
	}
	return false
}

// mergeParts 按顺序流式读取分片，逐一校验尺寸与 SHA-256，同时累加整体哈希并写出。
func mergeParts(m *Manifest, partDir string, out io.Writer, verifyChunks bool) error {
	overall := sha256.New()
	var total int64

	for _, c := range m.Chunks {
		path := filepath.Join(partDir, c.Name)
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("无法打开分片 %s：%v", c.Name, err)
		}

		var chunkHasher hash.Hash
		if verifyChunks {
			chunkHasher = sha256.New()
		}

		reader := bufio.NewReaderSize(f, ioBufSize)
		buf := make([]byte, ioBufSize)
		size := int64(0)
		for {
			n, rerr := reader.Read(buf)
			if n > 0 {
				if _, werr := out.Write(buf[:n]); werr != nil {
					f.Close()
					return fmt.Errorf("写入输出失败：%v", werr)
				}
				overall.Write(buf[:n])
				if chunkHasher != nil {
					chunkHasher.Write(buf[:n])
				}
				size += int64(n)
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				f.Close()
				return fmt.Errorf("读取分片 %s 失败：%v", c.Name, rerr)
			}
		}
		f.Close()

		if size != c.Size {
			return fmt.Errorf("分片 %s 尺寸不符：期望 %d 字节，实际 %d 字节", c.Name, c.Size, size)
		}
		if verifyChunks {
			got := hex.EncodeToString(chunkHasher.Sum(nil))
			if got != c.SHA256 {
				return fmt.Errorf("分片 %s 校验失败：内容已损坏", c.Name)
			}
		}
		total += size
	}

	if total != m.OriginalSize {
		return fmt.Errorf("合并大小不符：期望 %d 字节，实际 %d 字节", m.OriginalSize, total)
	}
	if m.OriginalSHA256 != "" && hex.EncodeToString(overall.Sum(nil)) != m.OriginalSHA256 {
		return fmt.Errorf("整体校验失败：合并结果与原始文件不一致")
	}
	return nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func printMergeUsage(fs *flag.FlagSet) {
	fmt.Fprint(fs.Output(), `用法：fp m <manifest> [选项]

选项：
`)
	fs.PrintDefaults()
	fmt.Fprint(fs.Output(), `
示例：
  fp m data.bin.parts/manifest.json -out restored.iso
  fp m data.bin.parts/manifest.json -check-only
`)
}
