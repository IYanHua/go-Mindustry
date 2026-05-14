package main

import (
	"encoding/json"
	"strings"
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

func buildRuntimeRulesRaw(wld *world.World, mapPath string) string {
	merged := map[string]any{}
	if wld == nil {
		return "{}"
	}
	marshal := func() string {
		// The Go server does not currently synchronize Mindustry's fog discovery
		// bitsets. Leaving map/campaign fog enabled makes official clients render
		// the world and minimap as fully undiscovered black.
		merged["fog"] = false
		merged["staticFog"] = false
		raw, err := json.Marshal(merged)
		if err != nil || len(raw) == 0 {
			return "{}"
		}
		return string(raw)
	}
	if raw := wld.RulesTagRaw(); raw != "" {
		_ = json.Unmarshal([]byte(raw), &merged)
	}
	rulesMgr := wld.GetRulesManager()
	if rulesMgr == nil {
		return marshal()
	}
	rules := rulesMgr.Get()
	if rules == nil {
		return marshal()
	}

	merged["allowEditRules"] = rules.AllowEditRules
	merged["infiniteResources"] = rules.InfiniteResources
	merged["waves"] = rules.Waves
	merged["waveTimer"] = rules.WaveTimer
	merged["airUseSpawns"] = rules.AirUseSpawns
	merged["wavesSpawnAtCores"] = rules.WavesSpawnAtCores
	merged["waveSpacing"] = rules.WaveSpacing * 60
	merged["initialWaveSpacing"] = rules.InitialWaveSpacing * 60
	merged["pvp"] = rules.Pvp
	merged["attackMode"] = rules.AttackMode
	merged["editor"] = rules.Editor
	merged["instantBuild"] = rules.InstantBuild
	merged["buildCostMultiplier"] = rules.BuildCostMultiplier
	merged["buildSpeedMultiplier"] = rules.BuildSpeedMultiplier
	merged["unitBuildSpeedMultiplier"] = rules.UnitBuildSpeedMultiplier
	merged["deconstructRefundMultiplier"] = rules.DeconstructRefundMultiplier
	merged["enemyCoreBuildRadius"] = rules.EnemyCoreBuildRadius
	if rules.Env != 0 {
		merged["env"] = rules.Env
	}
	if modeName := strings.TrimSpace(rules.ModeName); modeName != "" {
		merged["modeName"] = modeName
	}
	return marshal()
}

func syncRulesToConn(conn *netserver.Conn, wld *world.World, mapPath string) {
	if conn == nil || wld == nil {
		return
	}
	raw := buildRuntimeRulesRaw(wld, mapPath)
	_ = conn.SendAsync(&protocol.Remote_NetClient_setRules_23{
		Rules: protocol.Rules{Raw: raw},
	})
}

