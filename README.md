# punctpolish

平时写笔记有强迫症, 拷贝过来的东西有中文句号, 单词和汉字贴在一块, 看着不舒服, 于是写个 macOS 上的小工具递归监听指定目录, 有文件改动时就触发符号替换机制, 仅修改有变动的文件

## 它能做什么

- 递归监听目录及其子目录
- 只在文件发生变化时处理匹配的文本文件
- 将常见中文标点替换为 ASCII 标点
- 自动处理中英文、数字之间的空格，例如 `ERP 系统`、`JSON 数据`
- 跳过二进制文件，并避免自己写回文件时触发死循环
- 支持 `--dry-run` 和 `--scan-on-start`

示例：

```text
ERP系统已经上线，运行稳定！
这个Agent负责处理JSON数据。
```

处理后会变成：

```text
ERP 系统已经上线, 运行稳定
这个 Agent 负责处理 JSON 数据
```

## 运行要求

- macOS
- Go 1.23.12 或更高版本

## 构建

```bash
go build -o punctpolish ./cmd/punctpolish
```

也可以直接运行：

```bash
go run ./cmd/punctpolish --dir /path/to/docs
```

## 使用方式

最基本的启动方式：

```bash
./punctpolish --dir /path/to/docs
```

常见用法：

```bash
./punctpolish \
  --dir /path/to/docs \
  --foreground \
  --debounce 300ms \
  --log-level debug
```

命令行参数：

- `--dir`：要监听的根目录
- `--config`：显式指定配置文件路径
- `--scan-on-start`：启动时先处理一遍已有文件，再进入监听模式
- `--dry-run`：只输出会发生的变化，不实际写回文件
- `--foreground`：同时把运行日志输出到终端
- `--debounce`：覆盖默认 debounce 时间
- `--log-level`：`debug`、`info`、`warn`、`error`
- `--log-file`：显式指定日志文件路径

## 配置文件

配置文件名为 `.punctpolish.yaml`

程序会按下面顺序查找配置文件：

1. `--config` 指定的路径
2. 当前工作目录
3. 被监听的目录
4. `$HOME`

如果都找不到，就使用内置默认配置

示例：

```yaml
ext:
  - .md
  - .txt

ignore:
  - .git
  - node_modules
  - .idea
  - .vscode

debounce: 500ms
max_file_size: 10485760
```

字段说明：

- `ext`：要处理的文件扩展名
- `ignore`：要忽略的目录名
- `debounce`：文件变化后等待多久再处理
- `max_file_size`：允许处理的最大文件大小，单位为字节

默认值：

- `ext`：`.md`
- `ignore`：`.git`、`node_modules`、`.idea`、`.vscode`、`dist`、`build`
- `debounce`：`500ms`
- `max_file_size`：`10485760`

## 日志

默认日志路径为：

- `$XDG_STATE_HOME/punctpolish/punctpolish.log`
- 如果没有设置 `XDG_STATE_HOME`，则为 `~/.local/state/punctpolish/punctpolish.log`

如果希望同时在终端看到运行日志，可以加上 `--foreground`

## 测试

运行常规测试：

```bash
go test ./...
```

运行 watcher 集成测试：

```bash
go test -tags integration -v ./test
```

运行手工 smoke test：

```bash
bash test/run.sh
```

## 说明

- 默认情况下，程序启动时不会把已有文件全部重写
- 如果你希望启动时先处理一遍已有文件，可以使用 `--scan-on-start`
- 只有扩展名匹配配置的文件才会被处理
- 启动后新创建的子目录也会自动加入监听范围
- 文件里的所有内容都会参与检测和替换，包括 fenced code block 中的内容

