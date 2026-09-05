# fp —— 文件分片 / 合并工具

一个用 Go 编写、可编译为单个二进制文件的命令行工具，用于把大文件拆分为若干个小分片，并能通过描述文件（manifest）完整还原原文件。

## 功能概述

- **支持两种分片方式**：按固定分片大小、按分片数量。
- **配套合并功能**：`fp merge` 根据 manifest 自动还原原文件。
- **生成描述文件（manifest）**：记录原文件名、总大小、分片数、分片大小及每片校验值。
- **完整性校验**：拆分时计算每片哈希（SHA-256），合并时逐片校验，发现破损即报错。
- **内容与属性全保留**：合并结果与原始文件逐字节一致（SHA-256 + 大小双校验），并还原文件权限、修改时间、属主/属组（stat 元数据）。
- **中文友好**：界面、错误提示使用中文，帮助文本（`-h` / `--help`）为简体中文。
- **名称可配置**：输出文件夹名、manifest 文件名、分片文件名都可自定义。

## 技术选型

- 语言：Go（>= 1.21）
- 标准库为主，无第三方运行时依赖
- 构建产物：单个二进制文件，支持交叉编译（`GOOS` / `GOARCH`）

## 命令行设计

```
fp sp <input> [选项]       # split，拆分
fp m <manifest> [选项]    # merge，合并
fp --version
fp --help        # 同 -h，简体中文帮助文本
```

### sp 子命令（拆分）

```
fp sp <input>
  -size <size>    按分片大小拆分，支持单位 K/M/G（如 100M）
  -n <count>      按分片数量拆分
  -out <dir>      输出文件夹名（默认：<input> 同目录下的 <文件名>.parts）
  -manifest <名>  manifest 文件名（默认 manifest.json）
  -prefix <前缀>  分片文件名前缀（默认：原文件名）
  -pad <n>        分片序号位数（默认 4，如 part0001）
  -hash           计算每片 SHA-256（默认开启，可用 -no-hash 关闭）
```

> `-size` 与 `-n` 二选一，同时指定则报错。

### m 子命令（合并）

```
fp m <manifest>
  -out <path>     输出文件路径（默认：manifest 中记录的原文件名）
  -force          目标文件已存在时覆盖（默认提示确认）
  -check-only     只做校验，不写出文件
```

## 分片输出结构

以 `data.bin`、按 100M 拆分、输出文件夹 `data.bin.parts`、前缀 `data.bin`、序号 4 位为例：

```
data.bin.parts/
  manifest.json        # 描述文件（可用 -manifest 改名）
  data.bin.part0001    # 第 1 片（可用 -prefix / -pad 改前缀和位数）
  data.bin.part0002
  ...
```

分片命名规则：`<前缀>.part<序号>`，如 `data.bin.part0001`。可通过 `-prefix` 自定义前缀、`-pad` 设置序号位数、`-manifest` 修改描述文件名。

### manifest.json 格式

```json
{
  "version": 1,
  "original_name": "data.bin",
  "original_size": 1234567890,
  "original_sha256": "整个原文件的 SHA-256，合并后复核",
  "stat": {
    "mode": "0644",
    "mtime_ns": 1720000000123456789,
    "uid": 1000,
    "gid": 1000
  },
  "split_mode": "size",
  "chunk_size": 104857600,
  "chunk_count": 12,
  "chunks": [
    {"name": "data.bin.part0001", "size": 104857600, "sha256": "..."},
    {"name": "data.bin.part0002", "size": 104857600, "sha256": "..."}
  ]
}
```

### sp 流程（拆分）

1. 解析并校验参数（`-size` 与 `-n` 互斥）。
2. 打开输入文件，获取大小、权限、修改时间、属主/属组并存入 manifest。
3. 计算每片大小与片数。
4. 创建输出文件夹（`-out`），按命名规则生成分片文件，逐个写入固定大小数据，同时计算 SHA-256；全程同步计算原文件整体 SHA-256。
5. 写入描述文件（`-manifest` 指定，默认 `manifest.json`）。
6. 汇总输出（片数、总大小、各片大小、耗时）。

### m 流程（合并）

1. 读取并校验 `manifest.json`。
2. 逐一打开分片，计算 SHA-256 与 manifest 记录比对，不一致立即报错退出。
3. 校验全部通过后，按顺序拼接写出原文件。
4. 核验输出文件大小与 `original_size` 一致，并对整个输出文件复核 `original_sha256`，不一致报错。
5. 依 `stat` 还原权限、修改时间；属主/属组在具备权限时恢复（`chown` 不成功仅告警，不视为失败）。

## 一致性保证

合并结果与原始文件的一致性由三层校验保证：

1. **每片 SHA-256**：分片内容与拆分时一致。
2. **整体 SHA-256 + 大小**：合并后的完整文件与 `original_sha256` / `original_size` 一致。
3. **stat 元数据还原**：权限、修改时间、属主/属组按 manifest 还原。

满足以上条件即可认为合并结果与原始文件**逐字节完全相同**（属主/属组在无权限时可能无法恢复，会给出告警）。

## 校验与测试

- `go vet ./...` 静态检查
- `go test ./...` 单元测试（覆盖：大小分片、数量分片、边界大小、合并还原、哈希校验失败、元数据还原场景）

## 编译与使用

```sh
go build -o fp .            # 本地构建
go build -ldflags "-s -w" -o fp .   # 精简体积

# 跨平台交叉编译示例
GOOS=linux  GOARCH=amd64 go build -o fp-linux-amd64 .
GOOS=windows GOARCH=amd64 go build -o fp-windows-amd64.exe .
GOOS=darwin  GOARCH=amd64 go build -o fp-darwin-amd64 .
```

## 使用示例

```sh
# 按 100M 拆分，自定义输出文件夹、前缀、描述文件名
fp sp bigfile.iso -size 100M -out 分片文件 -prefix bigfile -manifest 清单.json

# 按 10 片拆分
fp sp bigfile.iso -n 10

# 合并还原到指定路径
fp m data.bin.parts/manifest.json -out bigfile-restored.iso

# 只校验分片，不还原
fp m data.bin.parts/manifest.json -check-only
```

## 后续可扩展方向

- 分片并行处理提升性能
- 支持其他哈希算法（MD5、SHA-1）
- 断点续传 / 增量拆分
- 加密分片