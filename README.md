# punctpolish

`punctpolish` 批量整理中文与中英文混排文本：将常见中文标点替换为 ASCII 标点，并补齐中文与英文、数字之间的空格。Markdown fenced code block 会保持原样。

## 使用

你可以使用默认的可执行文件名 `punctpolish` 或自定义的名字（如 `format`）来运行它：

```bash
format <文件或目录>
```

指定文件时，工具会处理任意扩展名，但仅在内容被识别为文本时才会写回；二进制文件会跳过。

```bash
format ./note.log
```

指定目录时，工具会递归处理目录及其子目录中的 `.md` 和 `.txt` 文件（不区分扩展名大小写）。其他格式不会处理。

```bash
format ./docs
```

## 构建与测试

```bash
go build -o punctpolish ./cmd/punctpolish
go test ./...
```

## 配置到本机的流程

如果你希望在任意目录下都能直接使用该工具，可以按照以下步骤将其配置到本地：

### 1. 编译并指定易记的名称
在项目根目录下编译时，可以直接通过 `-o` 参数将可执行文件命名为你喜欢的简短名字（例如 `format`）：

```bash
go build -o format ./cmd/punctpolish
```

### 2. 赋予可执行权限
确保编译出的文件具有可执行权限（通常 Go 编译出的文件默认已带权限，若无，可手动添加）：

```bash
chmod +x ./format
```

### 3. 复制到家目录（可选）
如果你想先备份到家目录：

```bash
cp ./format ~/
```

### 4. 移动到全局命令路径
将可执行文件移入你的用户全局 `bin` 目录（例如 `~/.local/bin/` 或 `/usr/local/bin/`）。

> **提示**：请确保你的 `PATH` 环境变量中已包含 `~/.local/bin`。

```bash
mv ~/format ~/.local/bin/
# 或者直接从项目目录移入：
# mv ./format ~/.local/bin/
```

### 5. 验证安装
配置完成后，打开任意新终端，即可直接运行简短的命令：

```bash
format <文件或目录>
```

