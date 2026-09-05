package main

import (
	"errors"
	"flag"
	"strings"
)

// normalizeArgs 将参数重排为「先选项、后位置参数」，
// 使用户可以写成 fp sp 文件 -size 100M，而不强制选项在前。
// valueFlags 是需要独占一个值的选项名（不含前导 -）。
func normalizeArgs(args []string, valueFlags map[string]bool) []string {
	var flags, rest []string
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			name := a
			if eq := strings.IndexByte(a, '='); eq > 0 {
				name = a[:eq]
			}
			if valueFlags[strings.TrimLeft(name, "-")] && !strings.Contains(a, "=") {
				flags = append(flags, a)
				if i+1 < len(args) {
					flags = append(flags, args[i+1])
					i++
				}
			} else {
				flags = append(flags, a)
			}
			i++
			continue
		}
		rest = append(rest, a)
		i++
	}
	return append(flags, rest...)
}

// handleParseError 统一处理选项解析错误：-h/--help 视为正常帮助请求。
func handleParseError(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}
