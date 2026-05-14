package main

import (
	"fmt"
	"os"
	"strings"
	coreio "github.com/IYanHua/mdt-server/internal/core"
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/runtimeassets"
	"github.com/IYanHua/mdt-server/internal/world"
	"github.com/IYanHua/mdt-server/internal/worldstream"
)

func buildInitialWorldDataPayload(conn *netserver.Conn, wld *world.World, cache *worldCache, path string, ioCore *coreio.Core2) ([]byte, error) {
	if cache == nil {
		return nil, fmt.Errorf("world cache unavailable")
	}
	playerID := int32(1)
	if conn != nil && conn.PlayerID() != 0 {
		playerID = conn.PlayerID()
	}
	strictRemoteRewrite := ioCore != nil && ioCore.HasRemote()
	rewritePayload := func(payload []byte, snap world.Snapshot, rewriteRuntime bool) ([]byte, error) {
		if len(payload) == 0 {
			return nil, fmt.Errorf("empty worldstream payload")
		}
		rules := buildRuntimeRulesRaw(wld, path)
		if ioCore != nil {
			wave := int32(0)
			waveTimeTicks := float32(0)
			tick := float64(0)
			patchPlayerID := int32(0)
			if rewriteRuntime {
				wave = snap.Wave
				waveTimeTicks = snap.WaveTimeTicks()
				tick = float64(snap.Tick)
				patchPlayerID = playerID
			}
			patched, err := ioCore.RewriteWorldStreamPayload(payload, patchPlayerID, rules, wave, waveTimeTicks, tick)
			if err == nil && len(patched) > 0 {
				return patched, nil
			}
			if strictRemoteRewrite {
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("core2 returned empty rewritten worldstream payload")
			}
		}
		if rewriteRuntime {
			if patched, perr := worldstream.RewriteRuntimeStateInWorldStream(payload, snap.Wave, snap.WaveTimeTicks(), float64(snap.Tick), playerID); perr == nil && len(patched) > 0 {
				payload = patched
			} else if playerID != 0 {
				if patched, perr := worldstream.RewritePlayerIDInWorldStream(payload, playerID); perr == nil && len(patched) > 0 {
					payload = patched
				}
			}
		} else if strings.TrimSpace(rules) != "" {
			// keep payload untouched here; rules patch runs just below
		}
		if patched, perr := worldstream.RewriteRulesInWorldStream(payload, rules); perr == nil && len(patched) > 0 {
			payload = patched
		}
		if _, inspectErr := worldstream.InspectWorldStreamPayload(payload); inspectErr != nil {
			return nil, inspectErr
		}
		return payload, nil
	}
	buildSnapshotPayload := func(model *world.WorldModel, snap world.Snapshot) ([]byte, bool, error) {
		if model == nil {
			return nil, false, nil
		}
		payload, err := worldstream.BuildWorldStreamFromModelSnapshot(model, playerID, snap)
		if err != nil || len(payload) == 0 {
			return nil, false, err
		}
		patched, rerr := rewritePayload(payload, snap, false)
		if rerr != nil {
			return nil, false, rerr
		}
		return patched, true, nil
	}
	if conn != nil {
		conn.SetLiveWorldStream(false)
	}
	base, err := cache.get(path)
	if err == nil && len(base) > 0 {
		payload := base
		if wld == nil {
			return payload, nil
		}

		snap := wld.Snapshot()
		if patched, rerr := rewritePayload(payload, snap, true); rerr != nil {
			return nil, rerr
		} else if len(patched) > 0 {
			payload = patched
		}
		return payload, nil
	}

	if wld != nil {
		snap := wld.Snapshot()
		if baseModel := wld.CloneModelForWorldStream(); baseModel != nil {
			if payload, ok, err := buildSnapshotPayload(baseModel, snap); err != nil {
				return nil, err
			} else if ok {
				if conn != nil {
					conn.SetLiveWorldStream(true)
				}
				return payload, nil
			}
		}
	}

	return nil, fmt.Errorf("build initial world payload: cache=%v", err)
}

func loadWorldStream(path string, content *protocol.ContentRegistry) ([]byte, error) {
	actualPath := resolveRuntimePath(path)
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".msav") || strings.HasSuffix(lower, ".msav.msav") {
		if payload, err := worldstream.BuildWorldStreamFromMSAV(actualPath); err == nil && len(payload) > 0 {
			return payload, nil
		}
		if model, err := worldstream.LoadWorldModelFromMSAV(actualPath, content); err == nil && model != nil {
			if payload, berr := worldstream.BuildWorldStreamFromModel(model, 1); berr == nil && len(payload) > 0 {
				return payload, nil
			}
		}
		return worldstream.BuildWorldStreamFromMSAV(actualPath)
	}
	data, err := os.ReadFile(actualPath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func warmWorldCache(cache *worldCache, path string) error {
	if cache == nil {
		return nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	_, err := cache.get(path)
	return err
}

func loadBootstrapWorldFallback() ([]byte, error) {
	data, _, err := runtimeassets.LoadBootstrapWorld(runtimeAssetsDir)
	return data, err
}
