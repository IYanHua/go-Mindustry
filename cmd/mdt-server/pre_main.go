package main

import (
	"fmt"
	"time"
	"github.com/IYanHua/mdt-server/internal/config"
	netserver "github.com/IYanHua/mdt-server/internal/net"
	plugin2 "github.com/IYanHua/mdt-server/internal/plugin"
	"net"
	"runtime"
	"runtime/debug"
)

func startMemoryGuard(cfg config.CoreConfig) {
	mb := func(n int) int64 { return int64(n) * 1024 * 1024 }
	if cfg.MemoryLimitMB > 0 {
		debug.SetMemoryLimit(mb(cfg.MemoryLimitMB))
		fmt.Printf("[memory] set memory limit: %dMB\n", cfg.MemoryLimitMB)
	}

	readHeapAlloc := func() uint64 {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return ms.HeapAlloc
	}

	if cfg.MemoryStartupMaxMB > 0 {
		maxB := uint64(mb(cfg.MemoryStartupMaxMB))
		before := readHeapAlloc()
		if before > maxB {
			runtime.GC()
			if cfg.MemoryFreeOSMemory {
				debug.FreeOSMemory()
			}
			after := readHeapAlloc()
			fmt.Printf("[memory] startup heap_alloc too high: before=%dMB after=%dMB max=%dMB\n",
				before/1024/1024, after/1024/1024, cfg.MemoryStartupMaxMB)
		}
	}

	if cfg.MemoryGCTriggerMB <= 0 {
		return
	}
	interval := time.Duration(cfg.MemoryCheckIntervalSec)
	if interval <= 0 {
		interval = 5
	}
	triggerB := uint64(mb(cfg.MemoryGCTriggerMB))
	go func() {
		t := time.NewTicker(interval * time.Second)
		defer t.Stop()
		for range t.C {
			before := readHeapAlloc()
			if before < triggerB {
				continue
			}
			runtime.GC()
			if cfg.MemoryFreeOSMemory {
				debug.FreeOSMemory()
			}
			after := readHeapAlloc()
			fmt.Printf("[memory] gc triggered: before=%dMB after=%dMB trigger=%dMB\n",
				before/1024/1024, after/1024/1024, cfg.MemoryGCTriggerMB)
		}
	}()
}

func (s *worldState) set(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = canonicalRuntimePath(path)
}

func (s *worldState) get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func connRemoteIP(c *netserver.Conn) string {
	if c == nil || c.RemoteAddr() == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(c.RemoteAddr().String())
	if err != nil {
		return c.RemoteAddr().String()
	}
	return host
}

// wrapChatConn 将 *netserver.Conn 包装为 plugin.ConnInterface



func wrapChatConn(c *netserver.Conn) plugin2.ConnInterface {
	return plugin2.WrapConn(c)
}

