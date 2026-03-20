## 先说结论

这个项目监听文件变化的本质, 不是靠轮询文件修改时间, 而是依赖操作系统提供的文件系统事件通知能力

整体链路可以概括为

1. 程序把目标目录注册给操作系统监听
2. 操作系统发现某个目录项发生变化
3. 底层文件事件通过 `fsnotify` 暴露给 Go 程序
4. 项目读取事件中的路径和操作类型
5. 再结合扩展名过滤, 忽略目录, debounce 和自写保护, 决定是否处理文件

## 当前项目里的实现方式

项目入口在 `internal/app/app.go`

启动时会做这几件事

1. 创建 `WriteGuard`, 用来避免自己写回文件时触发死循环
2. 创建 `Processor`, 负责真正处理文件内容
3. 创建 `Watcher`, 负责对接 `fsnotify`
4. 把根目录以及所有子目录加入监听
5. 启动事件循环, 持续读取文件变更事件

核心逻辑在 `internal/watcher/watcher.go`

- `fsnotify.NewWatcher()` 创建底层 watcher
- `AddDir()` 用 `filepath.WalkDir` 递归遍历目录
- 对每个目录调用 `w.fw.Add(path)` 注册监听
- `Run()` 持续从 `w.fw.Events` 和 `w.fw.Errors` 读取事件
- `handleEvent()` 根据 `event.Name` 和 `event.Op` 决定如何处理

也就是说, 项目知道"哪个文件发生了变化", 不是自己猜出来的, 而是底层事件直接带上来的

```go
case event, ok := <-w.fw.Events:
    if !ok {
        return
    }
    w.handleEvent(event)
```

然后在事件处理函数里

```go
path := event.Name
relevant := event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename)
```

这里的 `event.Name` 就是发生变化的路径, `event.Op` 则表示变化类型, 比如创建, 写入, 重命名, 删除等

## 它依赖了操作系统的什么能力

这里依赖的是操作系统提供的"文件系统事件通知机制", 不是应用层自己扫描目录

不同系统底层能力不同, 常见对应关系大致如下

- Linux: `inotify`
- macOS / BSD: `kqueue` 或对应的文件事件机制
- Windows: `ReadDirectoryChangesW`

这些机制的共同点是

- 程序先注册要观察的目录或文件
- 之后由内核在变化发生时推送事件
- 程序阻塞等待这些事件到来, 而不是不停轮询磁盘

所以从工程角度看, 它确实属于一种"操作系统提醒你有变化发生了"的模型

## 为什么说它不是简单的轮询

如果用轮询, 代码一般会是这种思路

1. 每隔一段时间遍历目录
2. `stat` 每个文件
3. 比较 `mtime`, 大小或者 inode 是否变化
4. 推断哪个文件可能被改了

这种方式的问题是

- 延迟和轮询间隔绑定
- 文件很多时成本明显升高
- 很难准确区分创建, 写入, 重命名等事件
- 容易漏掉快速发生又恢复的变化

而文件事件通知机制是"变化发生时主动通知", 所以它更适合像 `punctpolish` 这种长期驻留的小工具

## 为什么项目使用 `fsnotify`

### 1. 屏蔽跨平台差异

如果不使用 `fsnotify`, 就需要分别处理 Linux, macOS, Windows 的底层 API 差异

这些差异不只体现在函数名不同, 还包括

- 注册监听的方式不同
- 事件结构不同
- 错误处理方式不同
- 某些事件的语义不同
- 资源释放和句柄管理方式不同

`fsnotify` 的价值就在于, 它把这些差异统一成了一个更稳定的 Go 接口

```go
fw, err := fsnotify.NewWatcher()
err = fw.Add(dir)

for {
    select {
    case event := <-fw.Events:
        // 处理事件
    case err := <-fw.Errors:
        // 处理错误
    }
}
```

对业务代码来说, 重点就不再是"怎么跟不同内核说话", 而是"文件变化后我要做什么"

### 2. 更符合 Go 的并发模型

`fsnotify` 直接暴露 `Events` 和 `Errors` 两个 channel, 和当前项目的事件循环写法很自然地对上了

这也是 `internal/watcher/watcher.go` 代码比较干净的原因之一: 底层事件采集已经被库包装好了

### 3. 降低维护成本

如果直接对接操作系统

- 代码量会明显增加
- 调试难度会上升
- 可读性会下降
- 以后扩展到其他平台会更麻烦

对当前项目来说, 真正重要的业务逻辑其实不是"如何读内核事件", 而是

- 递归监听目录
- 忽略指定目录
- 只处理特定扩展名
- debounce 合并短时间内的重复事件
- 避免自己写文件时再触发自己

这些逻辑在项目里已经自己实现了, `fsnotify` 负责的是更底层的那一层

## 如果不使用 `fsnotify`, 大致会是什么感觉

下面不是完整可运行代码, 只是为了让读者直观看到"直接对接 OS"通常会更底层, 更琐碎

### 以 Linux `inotify` 为例

简化后的思路通常是

1. 调用系统调用创建 `inotify` 实例
2. 对每个目录调用 `inotify_add_watch`
3. 持续读取文件描述符上的原始事件数据
4. 把二进制事件结构解析成"路径 + 操作类型"
5. 再把它们转成业务代码能消费的事件

伪代码大概是这样

```go
fd := inotifyInit()
defer close(fd)

wd := inotifyAddWatch(fd, "/target/dir", IN_CREATE|IN_MODIFY|IN_MOVED_TO|IN_DELETE)
_ = wd

buf := make([]byte, 4096)

for {
    n := read(fd, buf)
    events := parseInotifyEvents(buf[:n])

    for _, ev := range events {
        fullPath := joinWatchedDirAndName(ev.Name)

        if ev.Mask&IN_CREATE != 0 {
            handleCreate(fullPath)
        }
        if ev.Mask&IN_MODIFY != 0 {
            handleWrite(fullPath)
        }
        if ev.Mask&IN_MOVED_TO != 0 {
            handleRename(fullPath)
        }
    }
}
```

这段对比里最值得注意的是两点

- 你要自己面对文件描述符, 事件掩码, 字节缓冲区和事件解析
- 你还要自己补齐递归监听, 新目录注册, 错误处理和资源释放

### 如果还想支持 macOS 和 Windows

事情会进一步复杂, 因为你不能拿 Linux 的这套代码直接复用到其他平台, 需要分别写不同实现, 再在更高一层做统一抽象

这正是 `fsnotify` 帮我们省掉的大头工作

## `punctpolish` 在 `fsnotify` 之上又做了什么

`fsnotify` 只是帮项目拿到"发生了什么变化"

当前项目真正补充的业务层能力主要有四个

### 1. 递归监听

`fsnotify` 通常不是一句话就自动递归监听整个目录树, 所以项目自己在 `AddDir()` 里遍历所有子目录并逐个注册

同时, 如果运行时创建了新目录, `handleEvent()` 发现它是新建目录后, 还会继续把这个目录加入监听范围

### 2. 扩展名过滤

项目不会处理所有变更文件, 只会处理配置里声明的目标扩展名

### 3. debounce

很多编辑器保存文件时可能产生一串连续事件, 比如临时文件写入, rename, write 等

项目用 `scheduleProcess()` 对同一路径做短时间合并, 避免重复处理

### 4. 自写保护

程序自己改写文件时, 底层照样会产生文件变更事件

所以项目通过 `WriteGuard` 在写入前先 `Mark(path)`, 收到事件时再判断是不是自己刚写出来的, 从而避免进入"处理一次就再次触发"的循环
