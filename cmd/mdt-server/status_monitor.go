package main

import (
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/config"
	"github.com/IYanHua/mdt-server/internal/sim"
)

type statusMonitor struct {
	srv     *netserver.Server
	cfg     config.Config
	engine  *sim.Engine
	start   time.Time
	enabled atomic.Bool
}

func newStatusMonitor(srv *netserver.Server, cfg config.Config, engine *sim.Engine) *statusMonitor {
	m := &statusMonitor{srv: srv, cfg: cfg, engine: engine, start: time.Now()}
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for range t.C {
			if m.enabled.Load() {
				fmt.Println(m.FormatOnce())
			}
		}
	}()
	return m
}

func (m *statusMonitor) Enable()  { m.enabled.Store(true) }
func (m *statusMonitor) Disable() { m.enabled.Store(false) }

func (m *statusMonitor) FormatOnce() string {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	uptime := time.Since(m.start).Truncate(time.Second)
	base := fmt.Sprintf("status: pid=%d uptime=%s goroutines=%d mem=%.1fMB sys=%.1fMB sessions=%d",
		os.Getpid(), uptime, runtime.NumGoroutine(), float64(ms.Alloc)/1024/1024, float64(ms.Sys)/1024/1024, len(m.srv.ListSessions()))
	if m.srv != nil {
		snapStats := m.srv.EntitySnapshotCacheStats()
		base = fmt.Sprintf("%s snap_cache(hit=%d miss=%d build=%s filter=%s)",
			base,
			snapStats.Hits,
			snapStats.Misses,
			snapStats.LastBuildDuration.Truncate(time.Millisecond),
			snapStats.LastFilterDuration.Truncate(time.Millisecond),
		)
	}
	if m.engine == nil {
		return base
	}
	stats := m.engine.Stats()
	overrun := "ok"
	if stats.Overrun {
		overrun = "overrun"
	}
	last := "n/a"
	if !stats.LastTickTime.IsZero() {
		last = stats.LastTickTime.Format("15:04:05")
	}
	util := "n/a"
	if stats.LastDispatch.Partitions > 0 && stats.LastDispatch.DispatchDuration > 0 {
		capacity := float64(stats.LastDispatch.DispatchDuration.Nanoseconds() * int64(stats.LastDispatch.Partitions))
		if capacity > 0 {
			util = fmt.Sprintf("%.0f%%", float64(stats.LastDispatch.WorkDuration.Nanoseconds())*100/capacity)
		}
	}
	return fmt.Sprintf("%s tick=%d tps=%d last=%s last_dur=%s part=%d work=%d workers=%d disp=%s busy=%s idle=%s util=%s %s",
		base,
		stats.Tick,
		stats.TPS,
		last,
		stats.LastDuration.Truncate(time.Millisecond),
		stats.Partitions,
		stats.TotalWork,
		stats.WorkerCount,
		stats.LastDispatch.DispatchDuration.Truncate(time.Millisecond),
		stats.LastDispatch.WorkDuration.Truncate(time.Millisecond),
		stats.LastDispatch.IdleDuration.Truncate(time.Millisecond),
		util,
		overrun)
}

