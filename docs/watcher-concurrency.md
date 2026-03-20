# Watcher 并发处理说明

这篇文档专门解释 `internal/watcher/watcher.go` 里 `scheduleProcess()` 的并发设计。

它主要回答四个问题:

1. 这里到底要解决什么问题
2. `pending` 这个 map 是干什么的
3. 并发问题具体出现在哪里
4. 当前实现是怎么避免并发错误的

## 先从需求开始: 为什么不能一有事件就立刻处理

文件监听程序通常不会只收到一次事件。

一个常见场景是:

- 编辑器保存文件
- 文件系统连续发出多个事件
- 这些事件可能都指向同一个文件

比如同一个文件可能在很短时间内连续收到:

- `Write`
- 再一次 `Write`
- 或者 `Rename`

如果程序每收到一次事件就立刻处理文件, 会出现两个问题:

1. 同一个文件在极短时间内被重复处理很多次
2. 处理成本被放大, 日志也会变得很吵

所以这里采用了一个常见策略: `debounce`。

它的意思是:

- 先不要立刻处理
- 等一小段时间
- 如果这段时间里同一个文件又来了新事件, 就继续往后等
- 只有等到“安静下来”之后, 才真正处理一次

## `pending` map 是干什么的

在 `Watcher` 里有这样两个字段:

```go
dmu     sync.Mutex
pending map[string]*time.Timer
```

其中:

- `pending` 是一个登记表
- key 是文件路径 `path`
- value 是“这个文件当前对应的延时定时器”

可以把它理解成:

```text
pending[path] = 这个文件现在是否已经安排了一个稍后执行的处理任务
```

举个例子:

```text
pending["/docs/a.md"] = timerA
pending["/docs/b.md"] = timerB
```

这表示:

- `a.md` 已经有一个定时器在等着触发
- `b.md` 也有自己的定时器

如果某个文件不在 `pending` 里, 就表示:

- 它当前没有等待中的延时任务

## 为什么一定要有这个 map

如果没有 `pending`, 程序在每次收到事件时都只能“盲目创建一个新 timer”。

例如同一个文件连续变化三次:

1. 第一次变化, 建一个 timer
2. 第二次变化, 再建一个 timer
3. 第三次变化, 再建一个 timer

结果就是同一个文件最后会被处理三次。

这不符合 debounce 的目标。

所以程序需要一个地方记住:

- 某个文件是不是已经有一个等待中的 timer
- 如果已经有了, 那就不要再新建
- 只需要把旧 timer 的等待时间往后推

`pending` 就是这个“记住状态的地方”。

## `scheduleProcess()` 的基本思路

核心代码如下:

```go
func (w *Watcher) scheduleProcess(path string) {
    w.dmu.Lock()
    defer w.dmu.Unlock()

    if t, ok := w.pending[path]; ok {
        t.Reset(w.cfg.Debounce)
        return
    }

    w.pending[path] = time.AfterFunc(w.cfg.Debounce, func() {
        w.dmu.Lock()
        delete(w.pending, path)
        w.dmu.Unlock()

        slog.Info("file changed", "path", path)
        w.proc.Process(path)
    })
}
```

把它翻成自然语言就是:

1. 收到某个文件的变化事件
2. 先去 `pending` 里看这个文件有没有等待中的 timer
3. 如果已经有了, 不新建任务, 只把这个 timer 往后延
4. 如果还没有, 就新建一个 timer
5. 等 timer 到期后, 真正处理这个文件
6. 处理前先把它从 `pending` 里删除, 表示它现在不再处于“等待中”

## 用时间线看一次完整过程

假设 `debounce = 500ms`, 文件 `/tmp/a.md` 在短时间内连续变了三次。

### 第一次变化

调用:

```go
scheduleProcess("/tmp/a.md")
```

此时 `pending` 里还没有这个路径。

于是程序:

- 创建一个新的 timer
- 计划在 500ms 后执行
- 把它放进 `pending`

这时可以想象成:

```text
pending["/tmp/a.md"] = timer1
```

### 200ms 后第二次变化

又调用:

```go
scheduleProcess("/tmp/a.md")
```

这次程序发现 `pending["/tmp/a.md"]` 已经存在。

于是它不会新建 timer, 而是执行:

```go
t.Reset(500 * time.Millisecond)
```

意思是:

- 原本快到点了
- 但既然又有新变化, 那就重新开始计时

### 再过 300ms 第三次变化

还是同样逻辑:

- 发现已有 timer
- 再次 `Reset`

### 后面终于安静了 500ms

timer 到期, 回调函数执行:

```go
delete(w.pending, "/tmp/a.md")
w.proc.Process("/tmp/a.md")
```

结果就是:

- 从登记表中删掉这条记录
- 真正处理文件一次

虽然前面收到了三次事件, 但最后只处理了一次。

## 并发问题到底出现在哪里

理解并发问题的关键在于:

- `scheduleProcess()` 在 watcher 主流程里被调用
- `time.AfterFunc(..., func() {})` 的回调会在另一个 goroutine 里异步执行

也就是说, 有不止一个执行流会碰 `pending`。

### 执行流 1: 主事件循环

主循环收到文件事件后, 会调用:

```go
w.scheduleProcess(path)
```

这里会:

- 读取 `pending[path]`
- 可能写入 `pending[path]`
- 可能对已有 timer 执行 `Reset`

### 执行流 2: timer 回调

timer 到期后, 回调函数会运行:

```go
delete(w.pending, path)
```

也就是说, 它会删除 map 里的条目。

### 问题本质

于是就形成了这样的情况:

- 一个 goroutine 正在读取或写入 `pending`
- 另一个 goroutine 也可能正在删除 `pending` 中的条目

而 Go 的普通 `map` 不是并发安全的。

如果多个 goroutine 同时读写同一个 map, 可能出现:

- 数据竞争
- 状态混乱
- 直接 panic

典型报错会像这样:

```text
fatal error: concurrent map read and map write
```

## 锁是怎么解决这个问题的

这里用的是 `sync.Mutex`:

```go
dmu sync.Mutex
```

它的作用可以理解成:

- `pending` 是共享数据
- 谁要读它、改它, 都必须先拿到同一把锁
- 没拿到锁的人先等着
- 这样同一时刻只有一个 goroutine 能操作这个 map

### 在 `scheduleProcess()` 里的保护

```go
w.dmu.Lock()
defer w.dmu.Unlock()
```

这一段把下面这些动作包成了一个原子区域:

- 看 `pending[path]` 在不在
- 如果在, 就 `Reset`
- 如果不在, 就新建 timer 并写入 map

这样别的 goroutine 就不能在中间把 map 改掉。

### 在 timer 回调里的保护

```go
w.dmu.Lock()
delete(w.pending, path)
w.dmu.Unlock()
```

这表示:

- timer 回调删除 map 条目时
- 也必须先拿同一把锁

这样主事件循环和 timer 回调不会同时改同一个 `pending` map。

## 为什么删除条目也很重要

当 timer 执行完后, 代码会:

```go
delete(w.pending, path)
```

这一步不仅是为了清理内存, 更是为了表达状态:

- “这个文件当前已经没有等待中的 timer 了”

如果不删, 后续同一个文件再次发生变化时, 程序会误以为:

- 它还在等待中

但实际上旧 timer 已经执行完了。

所以这一删, 本质上是在维护状态机。

## 当前设计等价于一个很小的状态机

对于每个文件路径来说, 其实只有两种状态:

### 状态 1: 没有等待中的 timer

表示:

- 这个文件当前不在 debounce 等待期

收到事件后:

- 创建 timer
- 进入“等待中”状态

### 状态 2: 已经有等待中的 timer

表示:

- 这个文件已经安排了稍后处理

收到新事件后:

- 不创建新的 timer
- 只重置现有 timer

当 timer 到期后:

- 删除 `pending[path]`
- 回到“没有等待中的 timer”状态

而锁的作用, 就是保证这个状态切换不会被多个 goroutine 同时打断。

## 为什么这里的锁只保护 `pending`

你会注意到, 回调里是先删 map, 再调用:

```go
w.proc.Process(path)
```

这里没有把 `Process()` 放在锁里, 这是合理的。

原因是:

- 锁的职责只是保护共享状态 `pending`
- 文件处理可能比较慢
- 如果把整个处理过程都放在锁里, 会让锁持有时间过长

那样会带来两个问题:

1. 其他文件的事件也要等这把锁释放
2. debounce 的状态维护会被慢处理拖住

所以当前写法是:

- 先在锁内完成共享状态更新
- 再在锁外执行真正的文件处理

这是更常见也更健康的并发设计。

## 一句话理解整个方案

`pending` 是“每个文件是否已经安排了延时处理任务”的登记表, timer 用来做 debounce, 而 `Mutex` 用来保证这张登记表在主事件循环和 timer 回调并发访问时不会发生竞争。

## 小结

这个并发问题本质上不是“多个文件同时处理”这么简单, 而是:

- 同一个共享 map 会被多个 goroutine 同时访问
- 一个 goroutine 来自主 watcher 事件流
- 另一个 goroutine 来自 `time.AfterFunc` 的异步回调

当前实现的解决方案也很直接:

- 用 `pending map` 保存每个路径的 timer 状态
- 用 `Reset` 合并短时间内的重复事件
- 用 `sync.Mutex` 串行化对 `pending` 的访问
- 用“删掉条目再处理文件”的方式恢复状态

这样就既实现了 debounce, 又避免了并发读写 map 带来的风险。
