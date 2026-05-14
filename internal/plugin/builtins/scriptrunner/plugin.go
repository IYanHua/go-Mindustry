package scriptrunner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IYanHua/mdt-server/internal/config"
	"github.com/IYanHua/mdt-server/internal/persist"
	"github.com/IYanHua/mdt-server/internal/plugin"
)

type scriptPlugin struct {
	Runtime string
	Name    string
	Path    string
}

type scriptController struct {
	modsCfg config.ModsConfig
	mu      sync.Mutex
	gcStop  chan struct{}
}

func newScriptController(modsCfg config.ModsConfig) *scriptController {
	return &scriptController{modsCfg: modsCfg}
}

func (s *scriptController) RunTask(task config.ScriptTask) (string, error) {
	rt := strings.ToLower(strings.TrimSpace(task.Runtime))
	switch rt {
	case "js":
		return runNodeScriptInDir(s.modsCfg.JSDir, task.Target, task.Args...)
	case "node":
		return runNodeScriptInDir(s.modsCfg.NodeDir, task.Target, task.Args...)
	case "go":
		return runGoInDir(s.modsCfg.GoDir, task.Target, task.Args...)
	default:
		return "", fmt.Errorf("不支持的 runtime: %s", task.Runtime)
	}
}

func (s *scriptController) ScheduleStartupTasks(tasks []config.ScriptTask) {
	for i := range tasks {
		task := tasks[i]
		delay := task.DelaySec
		if delay < 0 {
			delay = 0
		}
		go func(t config.ScriptTask, d int) {
			if d > 0 {
				time.Sleep(time.Duration(d) * time.Second)
			}
			out, err := s.RunTask(t)
			if out != "" {
				fmt.Printf("[script][startup] output runtime=%s target=%s\n%s\n", t.Runtime, t.Target, out)
			}
			if err != nil {
				fmt.Printf("[script][startup] failed runtime=%s target=%s err=%v\n", t.Runtime, t.Target, err)
				return
			}
			fmt.Printf("[script][startup] done runtime=%s target=%s delay=%ds\n", t.Runtime, t.Target, d)
		}(task, delay)
	}
}

func (s *scriptController) RunGCNow() {
	runtime.GC()
	debug.FreeOSMemory()
	fmt.Println("[script] 已执行 GC 与内存回收")
}

func (s *scriptController) SetDailyGC(hhmm string) error {
	hhmm = strings.TrimSpace(hhmm)
	s.mu.Lock()
	if s.gcStop != nil {
		close(s.gcStop)
		s.gcStop = nil
	}
	s.mu.Unlock()
	if hhmm == "" || strings.EqualFold(hhmm, "off") {
		fmt.Println("[script] 每日 GC 已关闭")
		return nil
	}
	if _, err := time.Parse("15:04", hhmm); err != nil {
		return fmt.Errorf("时间格式错误，需 HH:MM: %w", err)
	}
	stop := make(chan struct{})
	s.mu.Lock()
	s.gcStop = stop
	s.mu.Unlock()
	go s.dailyGCLoop(hhmm, stop)
	fmt.Printf("[script] 每日 GC 已设置: %s\n", hhmm)
	return nil
}

func (s *scriptController) dailyGCLoop(hhmm string, stop <-chan struct{}) {
	for {
		now := time.Now()
		today, _ := time.ParseInLocation("15:04", hhmm, now.Location())
		next := time.Date(now.Year(), now.Month(), now.Day(), today.Hour(), today.Minute(), 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-timer.C:
			s.RunGCNow()
		case <-stop:
			timer.Stop()
			return
		}
	}
}

func (s *scriptController) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gcStop != nil {
		close(s.gcStop)
		s.gcStop = nil
	}
}

type ScriptRunnerPlugin struct {
	ctl *scriptController
	cfg *config.Config
}

func (p *ScriptRunnerPlugin) ID() string { return "scriptrunner" }

func (p *ScriptRunnerPlugin) Init(ctx *plugin.Context) error {
	p.cfg = ctx.Config
	p.ctl = newScriptController(ctx.Config.Mods)

	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name:        "js",
		Description: "运行 JS 脚本: js <script.js> [args...]",
		Category:    "script",
		Handler:     p.handleJS,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name:        "node",
		Description: "运行 Node 脚本: node <script.js> [args...]",
		Category:    "script",
		Handler:     p.handleNode,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name:        "go",
		Description: "运行 Go 脚本: go <target.go|.> [args...]",
		Category:    "script",
		Handler:     p.handleGo,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name:        "script",
		Description: "脚本管理: script [help|file|gc|startup]",
		Category:    "script",
		Handler:     p.handleScript,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name:        "mod",
		Description: "列出脚本插件",
		Category:    "script",
		Handler:     p.handleMod,
	})

	ctx.Events.OnConfigReload(func() {
		p.cfg = ctx.Config
	})
	return nil
}

func (p *ScriptRunnerPlugin) Start() error {
	p.ctl.ScheduleStartupTasks(p.cfg.Script.StartupTasks)
	p.ctl.SetDailyGC(p.cfg.Script.DailyGCTime)
	return nil
}

func (p *ScriptRunnerPlugin) Stop() error {
	p.ctl.stop()
	return nil
}

func (p *ScriptRunnerPlugin) saveScript() error {
	return persist.SaveScriptConfig(p.cfg.Script, persist.ScriptState{
		Version:      1,
		StartupTasks: p.cfg.Script.StartupTasks,
		DailyGCTime:  p.cfg.Script.DailyGCTime,
	})
}

func (p *ScriptRunnerPlugin) handleJS(args []string) error {
	if len(args) < 1 {
		fmt.Println("用法: js <script.js> [args...]")
		return nil
	}
	out, err := runNodeScriptInDir(p.cfg.Mods.JSDir, args[0], args[1:]...)
	if out != "" {
		fmt.Print(out)
	}
	if err != nil {
		fmt.Printf("js 执行失败: %v\n", err)
	}
	return nil
}

func (p *ScriptRunnerPlugin) handleNode(args []string) error {
	if len(args) < 1 {
		fmt.Println("用法: node <script.js> [args...]")
		return nil
	}
	out, err := runNodeScriptInDir(p.cfg.Mods.NodeDir, args[0], args[1:]...)
	if out != "" {
		fmt.Print(out)
	}
	if err != nil {
		fmt.Printf("node 执行失败: %v\n", err)
	}
	return nil
}

func (p *ScriptRunnerPlugin) handleGo(args []string) error {
	if len(args) < 1 {
		fmt.Println("用法: go <target.go|.> [args...]")
		return nil
	}
	out, err := runGoInDir(p.cfg.Mods.GoDir, args[0], args[1:]...)
	if out != "" {
		fmt.Print(out)
	}
	if err != nil {
		fmt.Printf("go 执行失败: %v\n", err)
	}
	return nil
}

func (p *ScriptRunnerPlugin) handleScript(args []string) error {
	if len(args) == 0 || strings.EqualFold(args[0], "help") {
		fmt.Println("script 用法:")
		fmt.Println("  script help")
		fmt.Println("  script file")
		fmt.Println("  script gc now")
		fmt.Println("  script gc daily <HH:MM|off>")
		fmt.Println("  script startup list")
		fmt.Println("  script startup add <delaySec> <js|node|go> <target> [args...]")
		fmt.Println("  script startup del <index>")
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "file":
		fmt.Printf("script 配置文件: %s\n", p.cfg.Script.File)
	case "gc":
		if len(args) < 2 {
			fmt.Println("用法: script gc now | script gc daily <HH:MM|off>")
			return nil
		}
		switch strings.ToLower(args[1]) {
		case "now":
			p.ctl.RunGCNow()
		case "daily":
			if len(args) < 3 {
				fmt.Println("用法: script gc daily <HH:MM|off>")
				return nil
			}
			val := strings.TrimSpace(args[2])
			if err := p.ctl.SetDailyGC(val); err != nil {
				fmt.Printf("设置每日 GC 失败: %v\n", err)
				return nil
			}
			p.cfg.Script.DailyGCTime = val
			_ = p.saveScript()
		default:
			fmt.Println("用法: script gc now | script gc daily <HH:MM|off>")
		}
	case "startup":
		if len(args) < 2 {
			fmt.Println("用法: script startup list|add|del ...")
			return nil
		}
		switch strings.ToLower(args[1]) {
		case "list":
			if len(p.cfg.Script.StartupTasks) == 0 {
				fmt.Println("当前无开机脚本任务")
				return nil
			}
			for i, t := range p.cfg.Script.StartupTasks {
				fmt.Printf("[%d] delay=%ds runtime=%s target=%s args=%v\n", i, t.DelaySec, t.Runtime, t.Target, t.Args)
			}
		case "add":
			if len(args) < 5 {
				fmt.Println("用法: script startup add <delaySec> <js|node|go> <target> [args...]")
				return nil
			}
			delay, err := strconv.Atoi(args[2])
			if err != nil || delay < 0 {
				fmt.Println("delaySec 必须是 >=0 的整数")
				return nil
			}
			task := config.ScriptTask{
				DelaySec: delay,
				Runtime:  strings.ToLower(args[3]),
				Target:   args[4],
				Args:     append([]string(nil), args[5:]...),
			}
			p.cfg.Script.StartupTasks = append(p.cfg.Script.StartupTasks, task)
			_ = p.saveScript()
			fmt.Println("已添加开机脚本任务（下次启动自动执行）")
		case "del":
			if len(args) < 3 {
				fmt.Println("用法: script startup del <index>")
				return nil
			}
			idx, err := strconv.Atoi(args[2])
			if err != nil || idx < 0 || idx >= len(p.cfg.Script.StartupTasks) {
				fmt.Println("index 无效")
				return nil
			}
			p.cfg.Script.StartupTasks = append(p.cfg.Script.StartupTasks[:idx], p.cfg.Script.StartupTasks[idx+1:]...)
			_ = p.saveScript()
			fmt.Println("已删除开机脚本任务")
		default:
			fmt.Println("用法: script startup list|add|del ...")
		}
	default:
		fmt.Println("用法: script help")
	}
	return nil
}

func (p *ScriptRunnerPlugin) handleMod(args []string) error {
	plugins, err := listScriptPlugins(p.cfg.Mods)
	if err != nil {
		fmt.Printf("插件列表错误: %v\n", err)
		return nil
	}
	if len(plugins) == 0 {
		fmt.Println("当前无脚本插件")
		return nil
	}
	for _, sp := range plugins {
		fmt.Printf("plugin type=%s name=%s path=%s\n", sp.Runtime, sp.Name, sp.Path)
	}
	return nil
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
