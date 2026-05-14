package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"github.com/IYanHua/mdt-server/internal/config"
)

type scriptPlugin struct {
	Runtime string
	Name    string
	Path    string
}

func listScriptPlugins(cfg config.ModsConfig) ([]scriptPlugin, error) {
	var out []scriptPlugin
	lists := []struct {
		runtime string
		dir     string
		exts    map[string]struct{}
	}{
		{runtime: "js", dir: cfg.JSDir, exts: map[string]struct{}{".js": {}, ".mjs": {}, ".cjs": {}}},
		{runtime: "node", dir: cfg.NodeDir, exts: map[string]struct{}{".js": {}, ".mjs": {}, ".cjs": {}}},
		{runtime: "go", dir: cfg.GoDir, exts: map[string]struct{}{".go": {}}},
	}
	for _, item := range lists {
		dir := strings.TrimSpace(item.dir)
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if _, ok := item.exts[ext]; !ok {
				continue
			}
			out = append(out, scriptPlugin{
				Runtime: item.runtime,
				Name:    strings.TrimSuffix(e.Name(), ext),
				Path:    filepath.Join(dir, e.Name()),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Runtime == out[j].Runtime {
			return out[i].Path < out[j].Path
		}
		return out[i].Runtime < out[j].Runtime
	})
	return out, nil
}

func runNodeScriptInDir(baseDir, script string, args ...string) (string, error) {
	absScript, absBase, err := securePathInDir(baseDir, script)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(absScript))
	switch ext {
	case ".js", ".mjs", ".cjs":
	default:
		return "", fmt.Errorf("仅支持 .js/.mjs/.cjs: %s", absScript)
	}
	cmd := exec.Command("node", append([]string{absScript}, args...)...)
	cmd.Dir = absBase
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runGoInDir(baseDir, target string, args ...string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absBase, 0o755); err != nil {
		return "", err
	}
	clean := strings.TrimSpace(target)
	if clean == "" {
		return "", errors.New("target 不能为空")
	}
	if filepath.IsAbs(clean) {
		return "", errors.New("target 必须是相对路径")
	}
	if clean != "." {
		absTarget, _, serr := securePathInDir(absBase, clean)
		if serr != nil {
			return "", serr
		}
		relTarget, rerr := filepath.Rel(absBase, absTarget)
		if rerr != nil {
			return "", rerr
		}
		clean = relTarget
	}
	cmdArgs := append([]string{"run", clean}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = absBase
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func securePathInDir(baseDir, relative string) (string, string, error) {
	base := strings.TrimSpace(baseDir)
	if base == "" {
		return "", "", errors.New("目录未配置")
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(absBase, 0o755); err != nil {
		return "", "", err
	}
	rel := filepath.Clean(strings.TrimSpace(relative))
	if rel == "." || rel == "" {
		return "", "", errors.New("目标不能为空")
	}
	if filepath.IsAbs(rel) {
		return "", "", errors.New("目标必须是相对路径")
	}
	absTarget, err := filepath.Abs(filepath.Join(absBase, rel))
	if err != nil {
		return "", "", err
	}
	prefix := absBase + string(os.PathSeparator)
	if absTarget != absBase && !strings.HasPrefix(absTarget, prefix) {
		return "", "", errors.New("目标超出允许目录")
	}
	st, err := os.Stat(absTarget)
	if err != nil {
		return "", "", err
	}
	if st.IsDir() {
		return "", "", errors.New("目标不能是目录")
	}
	return absTarget, absBase, nil
}


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

func mergeValidAPIKeys(keys []string, extra ...string) ([]string, error) {
	merged := mergeKeys(keys, extra...)
	for _, key := range merged {
		if !config.IsValidAPIKey(key) {
			return nil, fmt.Errorf("API密钥不合格: %s", key)
		}
	}
	return merged, nil
}



