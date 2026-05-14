package main

import (
	"errors"
	"fmt"
	"io/fs"
	mathrand "math/rand"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"github.com/IYanHua/mdt-server/internal/config"
	"github.com/IYanHua/mdt-server/internal/worldstream"
)

func resolveWorldSelection(arg string) (string, error) {
	if strings.EqualFold(arg, "random") {
		return pickRandomWorld()
	}

	trimmed := strings.TrimSpace(arg)
	if trimmed == "" {
		return "", errors.New("地图参数为空")
	}

	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, ".bin") {
		return "", fmt.Errorf("已禁用 .bin 地图，请使用 .msav")
	}
	if strings.HasSuffix(lower, ".msav") || strings.HasSuffix(lower, ".msav.msav") {
		if exists(trimmed) {
			return canonicalRuntimePath(trimmed), nil
		}
		if strings.HasSuffix(lower, ".msav") || strings.HasSuffix(lower, ".msav.msav") {
			base := worldstream.TrimMapName(filepath.Base(trimmed))
			if p, ok, err := findWorldByBaseName(base); err != nil {
				return "", err
			} else if ok {
				return canonicalRuntimePath(p), nil
			}
			for _, candidate := range []string{
				filepath.Join("..", "core", "assets", "maps", "default", base+".msav"),
				filepath.Join("..", "..", "core", "assets", "maps", "default", base+".msav"),
			} {
				if exists(candidate) {
					return canonicalRuntimePath(candidate), nil
				}
			}
		}
		return "", fmt.Errorf("地图文件不存在: %s", trimmed)
	}

	if p, ok, err := findWorldByBaseName(trimmed); err != nil {
		return "", err
	} else if ok {
		return canonicalRuntimePath(p), nil
	}

	for _, candidate := range []string{
		filepath.Join("..", "core", "assets", "maps", "default", trimmed+".msav"),
		filepath.Join("..", "..", "core", "assets", "maps", "default", trimmed+".msav"),
	} {
		if exists(candidate) {
			return canonicalRuntimePath(candidate), nil
		}
	}

	if exists(trimmed) {
		return trimmed, nil
	}
	return "", fmt.Errorf("地图不存在: %s", trimmed)
}

func pickRandomWorld() (string, error) {
	localFiles, err := listWorldFilesRecursive(localWorldRoots())
	if err == nil && len(localFiles) > 0 {
		return canonicalRuntimePath(localFiles[mathrand.New(mathrand.NewSource(time.Now().UnixNano())).Intn(len(localFiles))]), nil
	}
	coreCandidates := []string{
		filepath.Join("..", "core", "assets", "maps", "default", "*.msav"),
		filepath.Join("..", "..", "core", "assets", "maps", "default", "*.msav"),
	}
	for _, g := range coreCandidates {
		files, err := filepath.Glob(g)
		if err == nil && len(files) > 0 {
			return canonicalRuntimePath(files[mathrand.New(mathrand.NewSource(time.Now().UnixNano())).Intn(len(files))]), nil
		}
	}
	return "", errors.New("未找到地图文件（需要 assets/worlds/*.msav 或 core/assets/maps/default/*.msav）")
}

func listWorldMaps() ([]string, error) {
	outSet := map[string]struct{}{}

	for _, g := range []string{
		filepath.Join("..", "core", "assets", "maps", "default", "*.msav"),
		filepath.Join("..", "..", "core", "assets", "maps", "default", "*.msav"),
	} {
		msavFiles, err := filepath.Glob(g)
		if err != nil {
			return nil, err
		}
		for _, f := range msavFiles {
			outSet[worldstream.TrimMapName(filepath.Base(f))] = struct{}{}
		}
	}
	localFiles, err := listWorldFilesRecursive(localWorldRoots())
	if err != nil {
		return nil, err
	}
	for _, f := range localFiles {
		outSet[worldstream.TrimMapName(filepath.Base(f))] = struct{}{}
	}

	out := make([]string, 0, len(outSet))
	for name := range outSet {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func localWorldRoots() []string {
	if len(runtimeWorldRoots) > 0 {
		return append([]string(nil), runtimeWorldRoots...)
	}
	return []string{
		filepath.Join("assets", "worlds"),
		filepath.Join("go-server", "assets", "worlds"),
		filepath.Join("..", "assets", "worlds"),
	}
}

func listWorldFilesRecursive(roots []string) ([]string, error) {
	outSet := make(map[string]struct{})
	out := make([]string, 0, 64)
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		st, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !st.IsDir() {
			continue
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".msav") {
				return nil
			}
			clean := filepath.Clean(path)
			if _, ok := outSet[clean]; ok {
				return nil
			}
			outSet[clean] = struct{}{}
			out = append(out, clean)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

func findWorldByBaseName(name string) (string, bool, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "", false, nil
	}
	files, err := listWorldFilesRecursive(localWorldRoots())
	if err != nil {
		return "", false, err
	}
	for _, f := range files {
		base := strings.ToLower(worldstream.TrimMapName(filepath.Base(f)))
		if base == name {
			return f, true, nil
		}
	}
	return "", false, nil
}

func exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func printIPs(listenAddr string) {
	fmt.Printf("监听地址: %s\n", listenAddr)
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("获取网卡失败: %v\n", err)
		return
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip := ipNet.IP
			if ip.IsLoopback() {
				continue
			}
			fmt.Printf("IP: %s (%s)\n", ip.String(), iface.Name)
		}
	}
}

func printSelfCheck(listenAddr string, build int, worldPath string, cfg config.Config) {
	fmt.Println("自检（无网络探测模式）:")
	fmt.Printf("  监听地址: %s\n", listenAddr)
	fmt.Printf("  目标版本: %d\n", build)
	fmt.Printf("  当前地图: %s\n", canonicalRuntimePath(worldPath))
	fmt.Printf("  API: enabled=%v bind=%s auth=%v keys=%d\n", cfg.API.Enabled, cfg.API.Bind, len(cfg.API.Keys) > 0, len(cfg.API.Keys))
	fmt.Printf("  Storage: mode=%s db=%v dir=%s\n", cfg.Storage.Mode, cfg.Storage.DatabaseEnabled, canonicalRuntimePath(cfg.Storage.Directory))
	fmt.Printf("  Mods: enabled=%v dir=%s\n", cfg.Mods.Enabled, cfg.Mods.Directory)
	fmt.Printf("  Snapshot: enabled=%v cold_dir=%s file=%s cold_interval=%ds hot_interval=%ds retention=%dd\n",
		cfg.Persist.Enabled, canonicalRuntimePath(cfg.Persist.Directory), cfg.Persist.File, cfg.Persist.IntervalSec, cfg.Persist.HotIntervalSec, cfg.Persist.RetentionDays)
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		fmt.Printf("  端口解析失败: %v\n", err)
		return
	}
	fmt.Printf("  地址解析: host=%s port=%s (仅检查格式)\n", host, port)
	if exists(worldPath) {
		fmt.Println("  地图文件: 正常")
	} else {
		fmt.Println("  地图文件: 不存在或不可读")
	}
	fmt.Println("  网络探测: 已禁用（避免触发连接中断日志）")
}

func printAPIKey(cfg config.Config) {
	fmt.Printf("API 绑定: %s\n", cfg.API.Bind)
	fmt.Printf("API 启用: %v\n", cfg.API.Enabled)
	if len(cfg.API.Keys) == 0 {
		fmt.Println("API Key: 未设置")
		return
	}
	fmt.Printf("API Keys(%d):\n", len(cfg.API.Keys))
	for _, k := range cfg.API.Keys {
		fmt.Printf("  %s\n", k)
	}
}

