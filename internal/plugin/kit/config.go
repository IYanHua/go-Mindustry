package kit

import (
	"sync/atomic"
)

// HotConfig 提供线程安全的配置热重载。
// T 为插件自定义的运行时配置结构体。
type HotConfig[T any] struct {
	val atomic.Value
}

// NewHotConfig 创建配置容器，初始值为 cfg。
func NewHotConfig[T any](cfg *T) *HotConfig[T] {
	h := &HotConfig[T]{}
	if cfg != nil {
		h.val.Store(cfg)
	}
	return h
}

// Load 返回当前配置（线程安全）。
func (h *HotConfig[T]) Load() *T {
	if h == nil {
		return nil
	}
	v := h.val.Load()
	if v == nil {
		return nil
	}
	return v.(*T)
}

// Store 更新配置（线程安全）。
func (h *HotConfig[T]) Store(cfg *T) {
	if h == nil || cfg == nil {
		return
	}
	h.val.Store(cfg)
}
