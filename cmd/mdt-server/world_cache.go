package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	coreio "github.com/IYanHua/mdt-server/internal/core"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
	"github.com/IYanHua/mdt-server/internal/worldstream"
)

func getSpawnPos(model *world.WorldModel, cache *worldCache, path string) (protocol.Point2, bool) {
	// 1. 优先使用核心位置缓存
	if pos, ok, err := spawnPosFromCache(cache, path); err == nil && ok {
		return pos, true
	}
	// 2. 回退到模型中的重生点
	return fallbackSpawnPosFromModel(model)
}

// spawnPosFromCache 从缓存获取重生点
func spawnPosFromCache(cache *worldCache, path string) (protocol.Point2, bool, error) {
	if cache == nil {
		return protocol.Point2{}, false, nil
	}
	return cache.spawnPos(path)
}

type worldCache struct {
	mu        sync.Mutex
	path      string
	modTime   time.Time
	data      []byte
	baseModel *world.WorldModel
	corePos   protocol.Point2
	corePosOK bool
	backend   *coreio.Core3
	content   *protocol.ContentRegistry
}

func (c *worldCache) invalidate() {
	if c != nil && c.backend != nil {
		_ = c.backend.InvalidateWorldCache("")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.path = ""
	c.modTime = time.Time{}
	c.data = nil
	c.baseModel = nil
	c.corePos = protocol.Point2{}
	c.corePosOK = false
}

func isMSAVPath(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lower, ".msav") || strings.HasSuffix(lower, ".msav.msav")
}

func (c *worldCache) fetchRemote(path string) (coreio.SnapshotResult, error) {
	if c == nil || c.backend == nil {
		return coreio.SnapshotResult{}, fmt.Errorf("remote world cache unavailable")
	}
	res, err := c.backend.GetWorldCache(path)
	if err != nil {
		return coreio.SnapshotResult{}, err
	}
	if _, inspectErr := worldstream.InspectWorldStreamPayload(res.Data); inspectErr != nil {
		return coreio.SnapshotResult{}, fmt.Errorf("inspect remote worldstream payload: %w", inspectErr)
	}
	c.mu.Lock()
	c.path = canonicalRuntimePath(path)
	if info, statErr := os.Stat(resolveRuntimePath(path)); statErr == nil {
		c.modTime = info.ModTime()
	}
	c.data = nil
	c.baseModel = nil
	c.corePos = res.CorePos
	c.corePosOK = res.CorePosOK
	c.mu.Unlock()
	return res, nil
}

func (c *worldCache) get(path string) ([]byte, error) {
	if c != nil && c.backend != nil {
		res, err := c.fetchRemote(path)
		if err != nil {
			return nil, err
		}
		return res.Data, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	canonicalPath := canonicalRuntimePath(path)
	actualPath := resolveRuntimePath(canonicalPath)

	info, err := os.Stat(actualPath)
	if err != nil {
		return nil, err
	}
	if c.path == canonicalPath && c.modTime.Equal(info.ModTime()) && len(c.data) > 0 {
		return c.data, nil
	}

	data, err := loadWorldStream(canonicalPath, c.content)
	if err != nil {
		return nil, err
	}
	if _, inspectErr := worldstream.InspectWorldStreamPayload(data); inspectErr != nil {
		return nil, fmt.Errorf("inspect local worldstream payload: %w", inspectErr)
	}
	c.path = canonicalPath
	c.modTime = info.ModTime()
	c.data = data
	c.baseModel = nil
	c.corePosOK = false
	if model, merr := worldstream.LoadWorldModelFromWorldStreamPayload(data, c.content); merr == nil {
		c.baseModel = model
	} else if isMSAVPath(canonicalPath) {
		if model, merr := worldstream.LoadWorldModelFromMSAV(actualPath, c.content); merr == nil {
			c.baseModel = model
		}
	}
	if isMSAVPath(canonicalPath) {
		if pos, ok, err := worldstream.FindCoreTileFromMSAV(actualPath); err == nil {
			c.corePos = pos
			c.corePosOK = ok
		}
	}
	return data, nil
}

func (c *worldCache) spawnPos(path string) (protocol.Point2, bool, error) {
	if c != nil && c.backend != nil {
		res, err := c.fetchRemote(path)
		if err != nil {
			return protocol.Point2{}, false, err
		}
		return res.CorePos, res.CorePosOK, nil
	}
	if _, err := c.get(path); err != nil {
		return protocol.Point2{}, false, err
	}
	return c.corePos, c.corePosOK, nil
}

func (c *worldCache) model(path string) *world.WorldModel {
	if c == nil {
		return nil
	}
	if c.backend != nil {
		res, err := c.fetchRemote(path)
		if err != nil || len(res.Data) == 0 {
			return nil
		}
		if model, merr := worldstream.LoadWorldModelFromWorldStreamPayload(res.Data, c.content); merr == nil {
			return model
		}
		actualPath := resolveRuntimePath(canonicalRuntimePath(path))
		if isMSAVPath(path) {
			if model, merr := worldstream.LoadWorldModelFromMSAV(actualPath, c.content); merr == nil {
				return model
			}
		}
		return nil
	}
	if _, err := c.get(path); err != nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.baseModel
}

