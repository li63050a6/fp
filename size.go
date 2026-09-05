package main

import (
	"errors"
	"strconv"
	"strings"
)

var sizeUnits = map[string]int64{
	"":    1,
	"b":   1,
	"k":   1 << 10,
	"kb":  1 << 10,
	"kib": 1 << 10,
	"m":   1 << 20,
	"mb":  1 << 20,
	"mib": 1 << 20,
	"g":   1 << 30,
	"gb":  1 << 30,
	"gib": 1 << 30,
}

// parseSize 解析分片大小字符串，支持 100M、1.5G、512K 等格式，单位不区分大小写。
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("分片大小不能为空")
	}

	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0, errors.New("分片大小格式无效：" + s)
	}

	mult := int64(1)
	if i < len(s) {
		u, ok := sizeUnits[strings.ToLower(s[i:])]
		if !ok {
			return 0, errors.New("分片大小单位无效，仅支持 B/K/M/G（如 100M）")
		}
		mult = u
	}

	if strings.Contains(s[:i], ".") {
		f, err := strconv.ParseFloat(s[:i], 64)
		if err != nil {
			return 0, errors.New("分片大小格式无效：" + s)
		}
		return int64(f * float64(mult)), nil
	}

	n, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return 0, errors.New("分片大小格式无效：" + s)
	}
	return n * mult, nil
}

// ceilDiv 返回 a/b 向上取整的结果。
func ceilDiv(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	return (a + b - 1) / b
}
