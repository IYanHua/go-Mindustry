package api

import (
	"crypto/rand"
	"fmt"
	"sort"
	"strings"

	apisrv "github.com/IYanHua/mdt-server/internal/api"
	"github.com/IYanHua/mdt-server/internal/config"
	netpkg "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/plugin"
	"github.com/IYanHua/mdt-server/internal/sim"
)

// Plugin wraps the HTTP API server as a plugin.
type Plugin struct {
	server *apisrv.Server
	cfg    *config.Config
}

func (p *Plugin) ID() string { return "builtins/api" }

func (p *Plugin) Init(ctx *plugin.Context) error {
	p.cfg = ctx.Config

	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "api", Description: "管理 API: api status|keys|keygen|keydel",
		Category: "admin", Handler: p.handleAPI,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "apikey", Description: "管理 API 密钥: apikey gen|list|del",
		Category: "admin", Handler: p.handleAPIKey,
	})
	return nil
}

func (p *Plugin) Start() error { return nil }
func (p *Plugin) Stop() error  { return nil }

// InitServer creates the API server. Must be called after the net server is available.
func (p *Plugin) InitServer(srv *netpkg.Server, statsFn func() *sim.TickStats) {
	p.server = apisrv.New(p.cfg.API, srv, statsFn)
}

// Server returns the underlying API server.
func (p *Plugin) Server() *apisrv.Server { return p.server }

func (p *Plugin) handleAPI(args []string) error {
	cfg := p.cfg
	apiSrv := p.server
	if len(args) == 0 || strings.EqualFold(args[0], "status") {
		fmt.Printf("api: enabled=%v bind=%s keys=%d\n", cfg.API.Enabled, cfg.API.Bind, len(cfg.API.Keys))
		fmt.Println("用法: api status | api keys | api keygen | api keydel <key>")
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "keys":
		if len(cfg.API.Keys) == 0 {
			fmt.Println("当前无 APIKEY")
			return nil
		}
		fmt.Printf("APIKEY(%d):\n", len(cfg.API.Keys))
		for _, k := range cfg.API.Keys {
			fmt.Printf("  %s\n", k)
		}
	case "keygen":
		key, err := generateAPIKey()
		if err != nil {
			fmt.Printf("生成 APIKEY 失败: %v\n", err)
			return nil
		}
		cfg.API.Keys = mergeKeys(cfg.API.Keys, key)
		cfg.API.Key = ""
		if apiSrv != nil {
			_ = apiSrv.AddAPIKey(key)
		}
		if cfg.Source != "" {
			if err := config.SaveSidecars(cfg.Source, *cfg); err != nil {
				fmt.Printf("保存配置失败: %v\n", err)
				return nil
			}
		}
		fmt.Printf("已生成 APIKEY: %s\n", key)
	case "keydel":
		if len(args) < 2 {
			fmt.Println("用法: api keydel <key>")
			return nil
		}
		target := strings.TrimSpace(args[1])
		if target == "" {
			fmt.Println("key 不能为空")
			return nil
		}
		cfg.API.Keys = removeKey(cfg.API.Keys, target)
		cfg.API.Key = ""
		if apiSrv != nil {
			_ = apiSrv.DeleteAPIKey(target)
		}
		if cfg.Source != "" {
			if err := config.SaveSidecars(cfg.Source, *cfg); err != nil {
				fmt.Printf("保存配置失败: %v\n", err)
				return nil
			}
		}
		fmt.Println("已删除 APIKEY")
	default:
		fmt.Println("用法: api status | api keys | api keygen | api keydel <key>")
	}
	return nil
}

func (p *Plugin) handleAPIKey(args []string) error { return p.handleAPI(args) }

// ApplyAPIKeySet syncs the desired API keys to the live server.
func (p *Plugin) ApplyAPIKeySet(desired []string) {
	if p.server == nil {
		return
	}
	current := p.server.ListAPIKeys()
	curSet := map[string]struct{}{}
	dstSet := map[string]struct{}{}
	for _, k := range current {
		curSet[k] = struct{}{}
	}
	for _, k := range desired {
		dstSet[k] = struct{}{}
	}
	for k := range curSet {
		if _, ok := dstSet[k]; !ok {
			_ = p.server.DeleteAPIKey(k)
		}
	}
	for k := range dstSet {
		if _, ok := curSet[k]; !ok {
			_ = p.server.AddAPIKey(k)
		}
	}
}

// --- helpers ---
func mergeKeys(keys []string, extra ...string) []string {
	set := map[string]struct{}{}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" {
			set[k] = struct{}{}
		}
	}
	for _, k := range extra {
		k = strings.TrimSpace(k)
		if k != "" {
			set[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func removeKey(keys []string, target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return mergeKeys(keys)
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.TrimSpace(k) == target {
			continue
		}
		out = append(out, k)
	}
	return mergeKeys(out)
}

func generateAPIKey() (string, error) {
	parts := []int{15, 13, 15, 19, 12, 10}
	out := []string{"mdt-server-go"}
	for i, n := range parts {
		if i == 5 {
			out = append(out, "yzf")
		}
		s, err := randomAlphaNum(n)
		if err != nil {
			return "", err
		}
		out = append(out, s)
	}
	return strings.Join(out, "-"), nil
}

func randomAlphaNum(n int) (string, error) {
	const alpha = "abcdefghijklmnopqrstuvwxyz0123456789"
	if n <= 0 {
		return "", nil
	}
	buf := make([]byte, n)
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i, b := range raw {
		buf[i] = alpha[int(b)%len(alpha)]
	}
	return string(buf), nil
}
