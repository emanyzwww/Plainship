// Package dev 实现本地开发模式: 监听源码变更, 自动重新构建并通知浏览器热更新.
// 设计原则:
//   - 零第三方依赖: 文件监听使用轮询, 跨平台且无需 cgo.
//   - 复用构建引擎: 变更后调用 builder.Build, 与 plainship build 完全一致的产物.
//   - 不触碰 Git: dev 模式只构建不提交、不打编号, 保持工作区干净.
package dev

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Watcher 监听 Space 源码目录的变更.
// 监听范围: docs/ (递归), themes/ (递归), 根目录 plainship.yaml.
// 变更检测基于文件修改时间快照, 轮询间隔默认 300ms.
type Watcher struct {
	// Roots 是需要监听的文件或目录列表 (绝对路径).
	Roots []string
	// Interval 是轮询间隔.
	Interval time.Duration
	// OnChange 在检测到变更时调用 (在独立 goroutine 中, 串行执行).
	OnChange func()

	mu       sync.Mutex
	snapshot map[string]int64 // path -> mtime unix nano
	stop     chan struct{}
	done     chan struct{}
}

// NewWatcher 创建监听器.
func NewWatcher(roots []string, onChange func()) *Watcher {
	return &Watcher{
		Roots:    roots,
		Interval: 300 * time.Millisecond,
		OnChange: onChange,
		snapshot: map[string]int64{},
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start 启动监听循环 (阻塞直到 Stop).
func (w *Watcher) Start() {
	defer close(w.done)
	w.takeSnapshot()
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			if w.changed() {
				w.takeSnapshot()
				if w.OnChange != nil {
					w.OnChange()
				}
			}
		}
	}
}

// Stop 停止监听.
func (w *Watcher) Stop() {
	close(w.stop)
	<-w.done
}

// changed 比较当前快照与上次快照, 检测是否有变化.
func (w *Watcher) changed() bool {
	cur := w.scan()
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(cur) != len(w.snapshot) {
		return true
	}
	for p, m := range cur {
		if old, ok := w.snapshot[p]; !ok || old != m {
			return true
		}
	}
	return false
}

// takeSnapshot 更新快照.
func (w *Watcher) takeSnapshot() {
	cur := w.scan()
	w.mu.Lock()
	w.snapshot = cur
	w.mu.Unlock()
}

// scan 收集所有监听路径的 mtime (unix nano).
func (w *Watcher) scan() map[string]int64 {
	out := map[string]int64{}
	for _, root := range w.Roots {
		w.scanPath(root, out)
	}
	return out
}

func (w *Watcher) scanPath(path string, out map[string]int64) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if !info.IsDir() {
		out[path] = info.ModTime().UnixNano()
		return
	}
	_ = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		// 忽略隐藏文件与编辑器临时文件.
		base := fi.Name()
		if strings.HasPrefix(base, ".") || strings.HasSuffix(base, "~") || strings.HasSuffix(base, ".swp") {
			return nil
		}
		out[p] = fi.ModTime().UnixNano()
		return nil
	})
}
