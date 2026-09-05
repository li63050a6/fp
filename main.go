package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printHelp()
		return
	}

	switch args[0] {
	case "sp":
		if err := runSplit(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "错误：%v\n", err)
			os.Exit(1)
		}
	case "m":
		if err := runMerge(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "错误：%v\n", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		printHelp()
	case "-v", "--version", "version":
		fmt.Printf("fp 版本 %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "错误：未知命令 %q\n请使用 fp --help 查看帮助。\n", args[0])
		os.Exit(2)
	}
}

func printHelp() {
	fmt.Print(`fp —— 文件分片 / 合并工具

用法：
  fp sp <文件> [选项]       拆分大文件为多个分片
  fp m <manifest> [选项]    根据描述文件合并还原
  fp --version              显示版本
  fp -h | --help            显示本帮助

命令 sp（拆分）选项：
  -size <大小>    按分片大小拆分，支持单位 K/M/G（如 100M）
  -n <数量>       按分片数量拆分
  -out <目录>     输出文件夹名（默认：<文件名>.parts）
  -manifest <名>  描述文件名（默认：manifest.json）
  -prefix <前缀>  分片文件名前缀（默认：原文件名）
  -pad <n>        分片序号位数（默认：4）
  -no-hash        不计算每片 SHA-256（默认计算）
  -size 与 -n 二选一。

命令 m（合并）选项：
  -out <路径>     输出文件路径（默认：描述文件中记录的原文件名）
  -force          目标文件已存在时覆盖（默认询问）
  -check-only     只校验分片完整性，不还原文件

示例：
  fp sp bigfile.iso -size 100M
  fp sp bigfile.iso -n 10
  fp m data.bin.parts/manifest.json -out restored.iso
`)
}
