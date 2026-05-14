package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/IYanHua/mdt-server/internal/bootstrap"
	"github.com/IYanHua/mdt-server/internal/buildinfo"
	"github.com/IYanHua/mdt-server/internal/buildsvc"
	"github.com/IYanHua/mdt-server/internal/config"
	coreio "github.com/IYanHua/mdt-server/internal/core"
	"github.com/IYanHua/mdt-server/internal/devlog"
	"github.com/IYanHua/mdt-server/internal/logging"
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/persist"
	plugin2 "github.com/IYanHua/mdt-server/internal/plugin"
	"github.com/IYanHua/mdt-server/internal/plugin/builtins/admincmds"
	apiplugin "github.com/IYanHua/mdt-server/internal/plugin/builtins/api"
	"github.com/IYanHua/mdt-server/internal/plugin/builtins/joinpopup"
	"github.com/IYanHua/mdt-server/internal/plugin/builtins/mapvote"
	"github.com/IYanHua/mdt-server/internal/plugin/builtins/scriptrunner"
	"github.com/IYanHua/mdt-server/internal/plugin/builtins/statusbar"
	"github.com/IYanHua/mdt-server/internal/plugin/builtins/unitcommands"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/sim"
	"github.com/IYanHua/mdt-server/internal/storage"
	"github.com/IYanHua/mdt-server/internal/tracepoints"
	"github.com/IYanHua/mdt-server/internal/vanilla"
	"github.com/IYanHua/mdt-server/internal/video"
	"github.com/IYanHua/mdt-server/internal/world"
	"github.com/IYanHua/mdt-server/internal/worldstream"
)

type worldState struct {
	mu      sync.RWMutex
	current string
}

type bindStatusCacheEntry struct {
	bound     bool
	expiresAt time.Time
}

type bindStatusResolver struct {
	mode     string
	apiURL   string
	client   *http.Client
	cacheTTL time.Duration
	identity *persist.PlayerIdentityStore
	mu       sync.Mutex
	cache    map[string]bindStatusCacheEntry
}

const defaultConfigPath = "configs/config.toml"

func normalizeRelativePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(path))
}

func preferredConfigBases(preferExecutable bool) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	addWithBinParent := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		add(clean)
	}

	addExecutable := func() {
		if exe, err := os.Executable(); err == nil {
			addWithBinParent(filepath.Dir(exe))
		}
	}
	addWorkingDir := func() {
		if wd, err := os.Getwd(); err == nil {
			addWithBinParent(wd)
		}
	}

	if preferExecutable {
		addExecutable()
		addWorkingDir()
	} else {
		addWorkingDir()
		addExecutable()
	}
	return out
}

func resolvePathFromBases(path string, bases []string) string {
	path = normalizeRelativePath(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	for _, base := range bases {
		candidate := filepath.Join(base, path)
		if exists(candidate) {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
			return filepath.Clean(candidate)
		}
	}
	if len(bases) > 0 {
		candidate := filepath.Join(bases[0], path)
		if abs, err := filepath.Abs(candidate); err == nil {
			return abs
		}
		return filepath.Clean(candidate)
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func resolveStartupConfigPath(raw string) string {
	raw = normalizeRelativePath(raw)
	if raw == "" {
		raw = normalizeRelativePath(defaultConfigPath)
	}
	preferExecutable := strings.EqualFold(raw, normalizeRelativePath(defaultConfigPath))
	return resolvePathFromBases(raw, preferredConfigBases(preferExecutable))
}

func newBindStatusResolver(mode, apiURL string, timeout, cacheTTL time.Duration, identity *persist.PlayerIdentityStore) *bindStatusResolver {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	if cacheTTL <= 0 {
		cacheTTL = 30 * time.Second
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "api" {
		mode = "internal"
	}
	return &bindStatusResolver{
		mode:     mode,
		apiURL:   strings.TrimSpace(apiURL),
		client:   &http.Client{Timeout: timeout},
		cacheTTL: cacheTTL,
		identity: identity,
		cache:    map[string]bindStatusCacheEntry{},
	}
}

func (r *bindStatusResolver) Bound(connUUID string) bool {
	connUUID = strings.TrimSpace(connUUID)
	if connUUID == "" {
		return false
	}
	if r == nil {
		return false
	}
	if r.mode != "api" {
		if r.identity == nil {
			return false
		}
		rec, ok := r.identity.Lookup(connUUID)
		return ok && rec.Bound
	}
	now := time.Now()
	r.mu.Lock()
	if rec, ok := r.cache[connUUID]; ok && now.Before(rec.expiresAt) {
		r.mu.Unlock()
		return rec.bound
	}
	r.mu.Unlock()

	url := strings.ReplaceAll(r.apiURL, "{id}", connUUID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(string(body)))
	bound := text == "yes"
	r.mu.Lock()
	r.cache[connUUID] = bindStatusCacheEntry{bound: bound, expiresAt: now.Add(r.cacheTTL)}
	r.mu.Unlock()
	return bound
}

var (
	runtimePlayerNameColorEnabled    atomic.Bool
	runtimePublicConnUUIDEnabled     atomic.Bool
	runtimeJoinLeaveChatEnabled      atomic.Bool
	runtimePlayerNamePrefix          atomic.Value
	runtimePlayerNameSuffix          atomic.Value
	runtimePlayerBindPrefixEnabled   atomic.Bool
	runtimePlayerBoundPrefix         atomic.Value
	runtimePlayerUnboundPrefix       atomic.Value
	runtimePlayerTitleEnabled        atomic.Bool
	runtimePlayerConnIDSuffixEnabled atomic.Bool
	runtimePlayerConnIDSuffixFormat  atomic.Value
	runtimePublicConnUUIDStore       *persist.PublicConnUUIDStore
	runtimePlayerIdentityStore       *persist.PlayerIdentityStore
	runtimeBindStatusResolver        *bindStatusResolver
	blockNameTranslationMu           sync.RWMutex
	blockNameTranslations            = defaultBlockNameTranslations()
)

type detailedLogWriter struct {
	mu       sync.Mutex
	dir      string
	prefix   string
	maxSize  int64
	maxFiles int
	file     *os.File
	size     int64
	seq      int64
}

func defaultBlockNameTranslations() map[string]string {
	return map[string]string{
		"conveyor":               "传送带",
		"titanium-conveyor":      "钛传送带",
		"armored-conveyor":       "装甲传送带",
		"junction":               "交叉器",
		"router":                 "分配器",
		"sorter":                 "分类器",
		"inverted-sorter":        "反向分类器",
		"overflow-gate":          "溢流门",
		"underflow-gate":         "反向溢流门",
		"duo":                    "双管炮",
		"scatter":                "散射炮",
		"scorch":                 "火焰炮",
		"core-shard":             "核心:碎片",
		"core-foundation":        "核心:基地",
		"core-nucleus":           "核心:核",
		"core-bastion":           "核心:堡垒",
		"core-citadel":           "核心:城塞",
		"core-acropolis":         "核心:卫城",
		"copper-wall":            "铜墙",
		"copper-wall-large":      "大型铜墙",
		"bridge-conveyor":        "传送带桥",
		"phase-conveyor":         "相位传送桥",
		"mass-driver":            "质量驱动器",
		"unloader":               "卸载器",
		"item-source":            "物品源",
		"item-void":              "物品黑洞",
		"power-node":             "电力节点",
		"power-node-large":       "大型电力节点",
		"rtg-generator":          "RTG发电机",
		"differential-generator": "温差发电机",
		"thorium-reactor":        "钍反应堆",
		"impact-reactor":         "冲击反应堆",
	}
}

func parseBlockNameTranslationJSON(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for rawKey, value := range data {
		switch typed := value.(type) {
		case map[string]any:
			for rawKey, rawVal := range typed {
				key := strings.ToLower(strings.TrimSpace(rawKey))
				val, ok := rawVal.(string)
				if key == "" || !ok || strings.TrimSpace(val) == "" {
					continue
				}
				out[key] = strings.TrimSpace(val)
			}
		case string:
			key := strings.ToLower(strings.TrimSpace(rawKey))
			if key != "" && strings.TrimSpace(typed) != "" {
				out[key] = typed
			}
		}
	}
	return out, nil
}

func blockNameTranslationCandidates(configDir string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	add := func(base string) {
		base = strings.TrimSpace(base)
		if base == "" {
			return
		}
		candidates := []string{
			filepath.Join(base, "json", "block_names.json"),
			filepath.Join(base, "configs", "json", "block_names.json"),
		}
		for _, candidate := range candidates {
			candidate = filepath.Clean(candidate)
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			out = append(out, candidate)
		}
	}
	add(configDir)
	if abs, err := filepath.Abs(configDir); err == nil {
		add(abs)
		add(filepath.Dir(abs))
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		add(exeDir)
		add(filepath.Dir(exeDir))
	}
	if wd, err := os.Getwd(); err == nil {
		add(wd)
		add(filepath.Dir(wd))
	}
	return out
}

func applyBlockNameTranslations(configDir string) {
	merged := defaultBlockNameTranslations()
	for _, path := range blockNameTranslationCandidates(configDir) {
		overrides, err := parseBlockNameTranslationJSON(path)
		if err != nil || len(overrides) == 0 {
			continue
		}
		for k, v := range overrides {
			merged[k] = v
		}
		break
	}
	blockNameTranslationMu.Lock()
	blockNameTranslations = merged
	blockNameTranslationMu.Unlock()
}

func resolveConfigSidecarPath(configDir, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	return filepath.Clean(filepath.Join(configDir, raw))
}

func runtimePathBases() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	add(runtimeBaseDir)
	if abs, err := filepath.Abs(runtimeBaseDir); err == nil {
		add(abs)
	}
	if wd, err := os.Getwd(); err == nil {
		add(wd)
	}
	return out
}

func resolveRuntimePath(raw string) string {
	raw = normalizeRelativePath(raw)
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	for _, base := range runtimePathBases() {
		candidate := filepath.Join(base, raw)
		if _, err := os.Stat(candidate); err == nil {
			if abs, aerr := filepath.Abs(candidate); aerr == nil {
				return abs
			}
			return filepath.Clean(candidate)
		}
	}
	if base := strings.TrimSpace(runtimeBaseDir); base != "" {
		candidate := filepath.Join(base, raw)
		if abs, err := filepath.Abs(candidate); err == nil {
			return abs
		}
		return filepath.Clean(candidate)
	}
	if abs, err := filepath.Abs(raw); err == nil {
		return abs
	}
	return filepath.Clean(raw)
}

func canonicalRuntimePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	clean := filepath.Clean(filepath.FromSlash(raw))
	abs := clean
	if !filepath.IsAbs(abs) {
		abs = resolveRuntimePath(clean)
	}
	base := strings.TrimSpace(runtimeBaseDir)
	if base != "" {
		if baseAbs, err := filepath.Abs(base); err == nil {
			if absClean, aerr := filepath.Abs(abs); aerr == nil {
				if rel, rerr := filepath.Rel(baseAbs, absClean); rerr == nil {
					rel = filepath.Clean(rel)
					if rel == "." {
						return rel
					}
					if rel != "" && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
						return rel
					}
				}
			}
		}
	}
	if filepath.IsAbs(clean) {
		return abs
	}
	return clean
}

func publicConnIDValue(store *persist.PublicConnUUIDStore, uuid string, connID int32) string {
	if !runtimePublicConnUUIDEnabled.Load() {
		return strconv.FormatInt(int64(connID), 10)
	}
	if id := publicConnUUIDValue(store, uuid); id != "" {
		return id
	}
	return strconv.FormatInt(int64(connID), 10)
}

func publicConnUUIDValue(store *persist.PublicConnUUIDStore, uuid string) string {
	uuid = strings.TrimSpace(uuid)
	if uuid != "" && store != nil {
		if id, ok := store.Lookup(uuid); ok && strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
	}
	return ""
}

func ensureConnIdentityRecords(publicStore *persist.PublicConnUUIDStore, identityStore *persist.PlayerIdentityStore, uuid, name, ip string) (string, bool) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" || publicStore == nil || !runtimePublicConnUUIDEnabled.Load() {
		return "", false
	}
	connUUID, err := publicStore.Ensure(uuid, name, ip)
	if err != nil {
		return "", false
	}
	connUUID = strings.TrimSpace(connUUID)
	if connUUID == "" {
		return "", false
	}
	if identityStore == nil {
		return connUUID, false
	}
	_, ok, err := identityStore.Ensure(connUUID)
	if err != nil {
		return connUUID, false
	}
	return connUUID, ok
}

func lookupConnIdentityState(publicStore *persist.PublicConnUUIDStore, identityStore *persist.PlayerIdentityStore, uuid string) (string, bool, bool) {
	if !runtimePublicConnUUIDEnabled.Load() {
		return "", false, false
	}
	connUUID := publicConnUUIDValue(publicStore, uuid)
	if connUUID == "" {
		return "", false, false
	}
	if identityStore == nil {
		return connUUID, true, false
	}
	_, ok := identityStore.Lookup(connUUID)
	return connUUID, true, ok
}

func shouldAnnotateConnectionCheckpoint(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "connect_packet", "world_handshake_sent", "connect_confirm", "connect_aborted_pre_confirm":
		return true
	default:
		return false
	}
}

func appendConnectionCheckpointDetail(detail string, ev netserver.NetEvent, publicStore *persist.PublicConnUUIDStore, identityStore *persist.PlayerIdentityStore) string {
	if !shouldAnnotateConnectionCheckpoint(ev.Kind) {
		return detail
	}
	connUUID, connUUIDReady, identityReady := lookupConnIdentityState(publicStore, identityStore, ev.UUID)
	displayName := strings.TrimSpace(ev.Name)
	displayNameReady := displayName != ""
	checkpoint := fmt.Sprintf("checkpoint conn_uuid=%q conn_uuid_ready=%t identity_ready=%t display_name_ready=%t display_name=%q",
		connUUID, connUUIDReady, identityReady, displayNameReady, displayName)
	if strings.TrimSpace(detail) == "" {
		return checkpoint
	}
	return detail + " | " + checkpoint
}

func buildCoreSnapshotData(wld *world.World) []byte {
	if wld == nil {
		return []byte{0}
	}
	snapshots := wld.TeamCoreItemSnapshots()
	w := protocol.NewWriter()
	if err := w.WriteByte(byte(len(snapshots))); err != nil {
		return []byte{0}
	}
	for _, snapshot := range snapshots {
		if err := w.WriteByte(byte(snapshot.Team)); err != nil {
			return []byte{0}
		}
		if err := w.WriteInt16(int16(len(snapshot.Items))); err != nil {
			return []byte{0}
		}
		for _, stack := range snapshot.Items {
			if err := w.WriteInt16(int16(stack.Item)); err != nil {
				return []byte{0}
			}
			if err := w.WriteInt32(stack.Amount); err != nil {
				return []byte{0}
			}
		}
	}
	return append([]byte(nil), w.Bytes()...)
}

func newDetailedLogWriter(logsDir string, maxMB, maxFiles int) (*detailedLogWriter, error) {
	dir := strings.TrimSpace(logsDir)
	if dir == "" {
		dir = "logs"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if maxMB <= 0 {
		maxMB = 2
	}
	if maxFiles <= 0 {
		maxFiles = 100
	}
	w := &detailedLogWriter{
		dir:      dir,
		prefix:   "net-detailed-en",
		maxSize:  int64(maxMB) * 1024 * 1024,
		maxFiles: maxFiles,
	}
	if err := w.openNewLocked(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *detailedLogWriter) openNewLocked() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
		w.size = 0
	}
	w.seq++
	name := fmt.Sprintf("%s-%s-%03d.log", w.prefix, time.Now().Format("20060102-150405"), w.seq)
	path := filepath.Join(w.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.size = 0
	w.cleanupLocked()
	return nil
}

func (w *detailedLogWriter) cleanupLocked() {
	pattern := filepath.Join(w.dir, w.prefix+"-*.log")
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) <= w.maxFiles {
		return
	}
	type fi struct {
		path string
		mod  time.Time
	}
	infos := make([]fi, 0, len(files))
	for _, p := range files {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		infos = append(infos, fi{path: p, mod: st.ModTime()})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].mod.Before(infos[j].mod) })
	for len(infos) > w.maxFiles {
		_ = os.Remove(infos[0].path)
		infos = infos[1:]
	}
}

func (w *detailedLogWriter) LogLine(line string) {
	if w == nil || strings.TrimSpace(line) == "" {
		return
	}
	data := []byte(line + "\n")
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.openNewLocked(); err != nil {
			return
		}
	}
	if w.maxSize > 0 && w.size+int64(len(data)) > w.maxSize {
		if err := w.openNewLocked(); err != nil {
			return
		}
	}
	n, _ := w.file.Write(data)
	w.size += int64(n)
}

func (w *detailedLogWriter) Write(p []byte) (n int, err error) {
	if w == nil {
		return 0, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.openNewLocked(); err != nil {
			return 0, err
		}
	}
	if w.maxSize > 0 && w.size+int64(len(p)) > w.maxSize {
		if err := w.openNewLocked(); err != nil {
			return 0, err
		}
	}
	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *detailedLogWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.file.Close()
	w.file = nil
	return err
}

var (
	runtimeWorldRoots []string
	runtimeAssetsDir  = "assets"
	runtimeConfigDir  = "configs"
	runtimeBaseDir    = "."
	runtimeWorldPath  atomic.Value
)

type startupStatus int

const (
	startupOK startupStatus = iota
	startupWarn
	startupFail
	startupInfo
)

type startupItem struct {
	status startupStatus
	label  string
	detail string
}

const defaultPlayerRespawnUnitID int16 = 35 // alpha

type startupReport struct {
	items []startupItem
}

func (r *startupReport) add(status startupStatus, label, detail string) {
	r.items = append(r.items, startupItem{status: status, label: label, detail: detail})
}

func (r *startupReport) ok(label, detail string)   { r.add(startupOK, label, detail) }
func (r *startupReport) warn(label, detail string) { r.add(startupWarn, label, detail) }
func (r *startupReport) fail(label, detail string) { r.add(startupFail, label, detail) }
func (r *startupReport) info(label, detail string) { r.add(startupInfo, label, detail) }

func (r *startupReport) print() {
	const (
		green = "\x1b[32m"
		red   = "\x1b[31m"
		yell  = "\x1b[33m"
		reset = "\x1b[0m"
	)
	fmt.Println("========================================")
	fmt.Println(" 启动报告")
	fmt.Println("========================================")
	for _, it := range r.items {
		prefix := "[INFO]"
		color := ""
		switch it.status {
		case startupOK:
			prefix = "[ OK ]"
			color = green
		case startupWarn:
			prefix = "[WARN]"
			color = yell
		case startupFail:
			prefix = "[FAIL]"
			color = red
		}
		if it.detail != "" {
			fmt.Printf("%s%s%s %s: %s\n", color, prefix, reset, it.label, it.detail)
		} else {
			fmt.Printf("%s%s%s %s\n", color, prefix, reset, it.label)
		}
	}
	fmt.Println("========================================")
}

func printUnitIDList(unitNames map[int16]string) {
	if len(unitNames) == 0 {
		return
	}
	type pair struct {
		id   int16
		name string
	}
	items := make([]pair, 0, len(unitNames))
	for id, name := range unitNames {
		items = append(items, pair{id: id, name: name})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].id < items[j].id })

	fmt.Println("========================================")
	fmt.Println(" 单位 ID 列表")
	fmt.Println("========================================")
	const perLine = 6
	for i := 0; i < len(items); i += perLine {
		end := i + perLine
		if end > len(items) {
			end = len(items)
		}
		var b strings.Builder
		for j := i; j < end; j++ {
			if j > i {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%d(%s)", items[j].id, items[j].name)
		}
		fmt.Println(b.String())
	}
	fmt.Println("========================================")
}

type mapLoadStats struct {
	width        int
	height       int
	tiles        int
	blocks       int
	builds       int
	cores        int
	entities     int
	units        int
	blockKinds   int
	floorKinds   int
	overlayKinds int
	tags         int
	msavVersion  int32
	contentBytes int
	patchBytes   int
	rawMapBytes  int
	rawEntBytes  int
	markerBytes  int
	customBytes  int
	hasRulesTag  bool
}

func computeMapLoadStats(model *world.WorldModel) mapLoadStats {
	if model == nil {
		return mapLoadStats{}
	}
	stats := mapLoadStats{
		width:        model.Width,
		height:       model.Height,
		tiles:        len(model.Tiles),
		entities:     len(model.Entities),
		units:        len(model.Units),
		tags:         len(model.Tags),
		msavVersion:  model.MSAVVersion,
		contentBytes: len(model.Content),
		patchBytes:   len(model.Patches),
		rawMapBytes:  len(model.RawMap),
		rawEntBytes:  len(model.RawEntities),
		markerBytes:  len(model.Markers),
		customBytes:  len(model.Custom),
	}
	if model.Tags != nil {
		if v, ok := model.Tags["rules"]; ok && strings.TrimSpace(v) != "" {
			stats.hasRulesTag = true
		}
	}
	blockKinds := map[int16]struct{}{}
	floorKinds := map[int16]struct{}{}
	overlayKinds := map[int16]struct{}{}
	for i := range model.Tiles {
		tile := &model.Tiles[i]
		if tile == nil {
			continue
		}
		if tile.Block > 0 {
			stats.blocks++
			blockKinds[int16(tile.Block)] = struct{}{}
		}
		if tile.Floor > 0 {
			floorKinds[int16(tile.Floor)] = struct{}{}
		}
		if tile.Overlay > 0 {
			overlayKinds[int16(tile.Overlay)] = struct{}{}
		}
		if tile.Build != nil {
			stats.builds++
			name := strings.ToLower(strings.TrimSpace(model.BlockNames[int16(tile.Build.Block)]))
			if strings.Contains(name, "core") || strings.Contains(name, "foundation") || strings.Contains(name, "nucleus") {
				stats.cores++
			}
		}
	}
	stats.blockKinds = len(blockKinds)
	stats.floorKinds = len(floorKinds)
	stats.overlayKinds = len(overlayKinds)
	return stats
}

func printMapDetails(path string, model *world.WorldModel) {
	if model == nil {
		return
	}
	stats := computeMapLoadStats(model)
	fmt.Println("========================================")
	fmt.Println(" 地图加载详情")
	fmt.Println("========================================")
	fmt.Printf("路径: %s\n", path)
	fmt.Printf("尺寸: %dx%d  Tiles=%d\n", stats.width, stats.height, stats.tiles)
	fmt.Printf("建筑: blocks=%d builds=%d cores=%d\n", stats.blocks, stats.builds, stats.cores)
	fmt.Printf("实体: entities=%d units=%d\n", stats.entities, stats.units)
	fmt.Printf("类型: blockKinds=%d floorKinds=%d overlayKinds=%d\n", stats.blockKinds, stats.floorKinds, stats.overlayKinds)
	fmt.Printf("MSAV: version=%d tags=%d rulesTag=%v\n", stats.msavVersion, stats.tags, stats.hasRulesTag)
	fmt.Printf("数据: content=%d patch=%d rawMap=%d rawEntities=%d markers=%d custom=%d\n",
		stats.contentBytes, stats.patchBytes, stats.rawMapBytes, stats.rawEntBytes, stats.markerBytes, stats.customBytes)
	fmt.Println("========================================")
}

func normalizeSnapshotWaveTimeSeconds(wld *world.World, waveTime float32) float32 {
	if waveTime < 0 {
		return 0
	}
	spacing := float32(120)
	if wld != nil {
		if rules := wld.GetRulesManager().Get(); rules != nil {
			if rules.WaveSpacing > 0 {
				spacing = rules.WaveSpacing
			}
			if rules.InitialWaveSpacing > spacing {
				spacing = rules.InitialWaveSpacing
			}
		}
	}
	maxReasonable := spacing * 4
	if maxReasonable < 120 {
		maxReasonable = 120
	}
	if maxReasonable > 3600 {
		maxReasonable = 3600
	}
	if waveTime <= maxReasonable {
		return waveTime
	}
	// Older builds sometimes stored vanilla 60Hz wavetime ticks instead of
	// this server's internal seconds. Convert only when it is clearly tick-scaled.
	if waveTime > maxReasonable*8 {
		seconds := waveTime / 60
		if seconds > 0 && seconds <= maxReasonable {
			return seconds
		}
	}
	return spacing
}

func validateBuildVersion(build int) error {
	if build != 157 {
		return fmt.Errorf("仅支持 Mindustry build 157；请使用 -build 157")
	}
	return nil
}

func main() {
	cfgPath := flag.String("config", filepath.FromSlash(defaultConfigPath), "path to config file")
	addr := flag.String("addr", "0.0.0.0:6567", "listen address for Mindustry protocol (TCP+UDP)")
	buildVersion := flag.Int("build", 157, "Mindustry build version; only official build 157 is supported")
	worldArg := flag.String("world", "random", "world source: random | <map-name> | <.msav file path>")
	recordVideo := flag.Bool("record-video", false, "record a realtime top-down match video from live server state")
	videoDir := flag.String("video-dir", filepath.FromSlash("data/video"), "base directory for recorded match video sessions")
	videoFPS := flag.Int("video-fps", 30, "capture FPS for realtime match video recording; common values: 5,10,15,20,25,30")
	videoWidth := flag.Int("video-width", 1920, "video output width in pixels")
	videoHeight := flag.Int("video-height", 1080, "video output height in pixels")
	videoTileSize := flag.Int("video-tile-size", 0, "optional max pixels per tile; 0 lets the recorder fit the whole map automatically")
	coreRole := flag.String("core-role", "", "internal use only: child core role (core2|core3|core4)")
	ipcEndpoint := flag.String("ipc-endpoint", "", "internal use only: child core IPC named pipe endpoint")
	parentPID := flag.Int("parent-pid", 0, "internal use only: parent process ID for child cores")
	printVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *printVersion {
		name := strings.TrimSpace(buildinfo.DisplayName)
		if name == "" {
			name = "mdt-server"
		}
		fmt.Printf("%s %s (%s)\n", name, buildinfo.Version, buildinfo.Commit)
		return
	}

	resolvedCfgPath := resolveStartupConfigPath(*cfgPath)
	if err := bootstrap.EnsureStartupConfigTree(resolvedCfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "释放内置配置失败: %v\n", err)
		os.Exit(1)
	}
	cfg := config.Default()
	cfg.Source = resolvedCfgPath
	if loaded, err := config.Load(cfg.Source); err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %v\n", err)
		os.Exit(1)
	} else {
		cfg = loaded
		cfg.Source = resolvedCfgPath
	}
	applyProcessConsoleTitle(cfg, strings.TrimSpace(*coreRole), cfg.Runtime.ServerName)
	applyProcessWindowIcon()
	if strings.TrimSpace(*coreRole) != "" {
		if err := coreio.RunChildCore(*coreRole, *ipcEndpoint, *parentPID, cfg.Persist); err != nil {
			fmt.Fprintf(os.Stderr, "child core %s failed: %v\n", *coreRole, err)
			os.Exit(1)
		}
		return
	}
	if err := validateBuildVersion(*buildVersion); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	configDir := filepath.Dir(cfg.Source)
	if strings.TrimSpace(configDir) == "" {
		configDir = filepath.FromSlash("configs")
	}
	rootDir := filepath.Dir(configDir)
	if strings.TrimSpace(rootDir) == "" || rootDir == configDir {
		rootDir = "."
	}
	runtimeBaseDir = rootDir
	// 非配置类目录/文件全部以 EXE 根目录为基准生成；configs 只存放配置文件。
	config.ApplyBaseDir(&cfg, rootDir)
	detailLog, detailLogErr := newDetailedLogWriter(cfg.Runtime.LogsDir, cfg.Sundries.DetailedLogMaxMB, cfg.Sundries.DetailedLogMaxFiles)
	if detailLogErr != nil {
		fmt.Fprintf(os.Stderr, "初始化 logs 详细日志失败: %v\n", detailLogErr)
		os.Exit(1)
	}
	traceLog, traceLogErr := tracepoints.New(cfg.Tracepoints.File, cfg.Tracepoints.Enabled)
	if traceLogErr != nil {
		fmt.Fprintf(os.Stderr, "初始化 tracepoints 日志失败: %v\n", traceLogErr)
		os.Exit(1)
	}
	defer func() {
		_ = traceLog.Close()
	}()

	// 设置标准log输出到文件和控制台
	logMultiWriter := io.MultiWriter(os.Stdout, detailLog)
	stdlog.SetOutput(logMultiWriter)

	var runtimeTraceCfg atomic.Value
	runtimeTraceCfg.Store(cfg.Tracepoints)
	currentTraceCfg := func() config.TracepointsConfig {
		if v := runtimeTraceCfg.Load(); v != nil {
			if loaded, ok := v.(config.TracepointsConfig); ok {
				return loaded
			}
		}
		return config.Default().Tracepoints
	}
	logTrace := func(category, point string, fields map[string]any) {
		if traceLog == nil || !traceLog.Enabled() {
			return
		}
		traceLog.Log(category, point, fields)
	}

	runtimeConfigDir = configDir
	runtimeAssetsDir = cfg.Runtime.AssetsDir
	runtimeWorldRoots = []string{cfg.Runtime.WorldsDir}
	applyBlockNameTranslations(configDir)

	startMemoryGuard(cfg.Core)

	startup := &startupReport{}
	bootstrapResult, err := bootstrap.EnsureWorkspace(cfg.Source, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化工作目录失败: %v\n", err)
		os.Exit(1)
	}

	log := logging.New(os.Stdout)
	saveConfig := func() error {
		if cfg.Source == "" {
			return nil
		}
		return config.SaveSidecars(cfg.Source, cfg)
	}
	keys, keyErr := mergeValidAPIKeys(cfg.API.Keys, cfg.API.Key)
	if keyErr != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %v\n", keyErr)
		os.Exit(1)
	}
	cfg.API.Keys = keys
	cfg.API.Key = ""
	if st, ok, err := persist.LoadScriptConfig(cfg.Script); err == nil && ok {
		cfg.Script.StartupTasks = st.StartupTasks
		cfg.Script.DailyGCTime = st.DailyGCTime
	}

	nameForBanner := strings.TrimSpace(buildinfo.DisplayName)
	if nameForBanner == "" {
		nameForBanner = "mdt-server"
	}
	fmt.Fprintf(os.Stdout, "%s %s (%s)\n", nameForBanner, buildinfo.Version, buildinfo.Commit)
	fmt.Fprintf(os.Stdout, "config=%s cores=%d tps=%d addr=%s build=%d world=%s vanilla=%s\n", canonicalRuntimePath(cfg.Source), cfg.Runtime.Cores, cfg.Core.TPS, *addr, *buildVersion, *worldArg, canonicalRuntimePath(cfg.Runtime.VanillaProfiles))
	if len(bootstrapResult.CreatedDirs) > 0 || len(bootstrapResult.CreatedFiles) > 0 {
		fmt.Fprintf(os.Stdout, "workspace initialized: dirs=%d files=%d\n", len(bootstrapResult.CreatedDirs), len(bootstrapResult.CreatedFiles))
	}
	gameVersion := strings.TrimSpace(buildinfo.GameVersion)
	if gameVersion == "" {
		gameVersion = fmt.Sprintf("Mindustry %d", *buildVersion)
	}
	startup.ok("游戏版本", gameVersion)
	if strings.TrimSpace(buildinfo.Version) != "" {
		startup.ok("外部版本", buildinfo.Version)
	}

	// 初始化开发者日志
	devLog := devlog.New(os.Stdout)
	devLog.SetLevel(devlog.LogLevelDebug)
	if cfg.Runtime.DevLogEnabled {
		startup.ok("开发者日志", "已启用")
	} else {
		startup.info("开发者日志", "未启用")
	}

	// 插件管理器
	pluginMgr := plugin2.NewManager()
	joinPopupPlugin := &joinpopup.JoinPopupPlugin{}
	mapVotePlugin := mapvote.NewMapVotePlugin()
	unitCmdPlugin := &unitcommands.Plugin{}
	apiPlugin := &apiplugin.Plugin{}
	if err := pluginMgr.RegisterBuiltin(&statusbar.StatusBarPlugin{}); err != nil {
		fmt.Fprintf(os.Stderr, "register statusbar plugin: %v\n", err)
	}
	if err := pluginMgr.RegisterBuiltin(joinPopupPlugin); err != nil {
		fmt.Fprintf(os.Stderr, "register joinpopup plugin: %v\n", err)
	}
	if err := pluginMgr.RegisterBuiltin(&scriptrunner.ScriptRunnerPlugin{}); err != nil {
		fmt.Fprintf(os.Stderr, "register scriptrunner plugin: %v\n", err)
	}
	if err := pluginMgr.RegisterBuiltin(mapVotePlugin); err != nil {
		fmt.Fprintf(os.Stderr, "register mapvote plugin: %v\n", err)
	}
	if err := pluginMgr.RegisterBuiltin(unitCmdPlugin); err != nil {
		fmt.Fprintf(os.Stderr, "register unitcommands plugin: %v\n", err)
	}
	if err := pluginMgr.RegisterBuiltin(&admincmds.Plugin{}); err != nil {
		fmt.Fprintf(os.Stderr, "register admincmds plugin: %v\n", err)
	}
	if err := pluginMgr.RegisterBuiltin(apiPlugin); err != nil {
		fmt.Fprintf(os.Stderr, "register api plugin: %v\n", err)
	}

	// 加载 .so 动态插件
	if loaded, err := pluginMgr.LoadFromDir("plugins"); err != nil {
		fmt.Fprintf(os.Stderr, "load plugins dir: %v\n", err)
	} else if loaded > 0 {
		startup.ok("动态插件", fmt.Sprintf("已加载 %d 个", loaded))
	}

	if cfg.Mods.Enabled {
		// startup.warn("模组系统", "暂未实现")
		files, err := os.ReadDir(cfg.Mods.Directory)
		if err == nil {
			for _, mod := range files {
				if strings.HasSuffix(mod.Name(), ".plugin") && !mod.IsDir() {
					pluginMod := filepath.Join(cfg.Mods.Directory, mod.Name())
					startup.info("模组系统", "开始加载"+pluginMod)
					ModLoader(pluginMod)

				}
			}
		}
	} else {
		startup.info("Mods", "未启用")
	}

	worldChoice := *worldArg
	hotSnapshots := persist.NewHotSnapshotStore()
	var restoredSnapshot persist.State
	var restoredSnapshotOK bool
	if cfg.Persist.Enabled {
		if st, ok, err := persist.Load(cfg.Persist); err != nil {
			log.Warn("cold snapshot load failed", logging.Field{Key: "error", Value: err.Error()})
			startup.warn("冷快照加载", err.Error())
		} else if ok {
			restoredSnapshot = st
			restoredSnapshotOK = true
			if worldChoice == "random" && st.MapPath != "" {
				worldChoice = st.MapPath
				startup.ok("冷快照地图恢复", st.MapPath)
			}
		}
	} else {
		startup.info("快照", "未启用")
	}

	initialWorld, err := resolveWorldSelection(worldChoice)
	if err != nil {
		fmt.Fprintf(os.Stderr, "世界选择无效: %v\n", err)
		os.Exit(1)
	}
	initialWorld = canonicalRuntimePath(initialWorld)
	state := &worldState{current: initialWorld}
	runtimePlayerNameColorEnabled.Store(cfg.Personalization.PlayerNameColorEnabled)
	runtimeJoinLeaveChatEnabled.Store(cfg.Personalization.JoinLeaveChatEnabled)
	runtimePlayerNamePrefix.Store(cfg.Personalization.PlayerNamePrefix)
	runtimePlayerNameSuffix.Store(cfg.Personalization.PlayerNameSuffix)
	runtimePlayerBindPrefixEnabled.Store(cfg.Personalization.PlayerBindPrefixEnabled)
	runtimePlayerBoundPrefix.Store(cfg.Personalization.PlayerBoundPrefix)
	runtimePlayerUnboundPrefix.Store(cfg.Personalization.PlayerUnboundPrefix)
	runtimePlayerTitleEnabled.Store(cfg.Personalization.PlayerTitleEnabled)
	runtimePlayerConnIDSuffixEnabled.Store(cfg.Personalization.PlayerConnIDSuffixEnabled)
	runtimePlayerConnIDSuffixFormat.Store(cfg.Personalization.PlayerConnIDSuffixFormat)
	if cfg.Personalization.StartupCurrentMapLineEnabled {
		fmt.Fprintf(os.Stdout, "当前地图: %s\n", canonicalRuntimePath(initialWorld))
	}

	var publicConnUUIDStore *persist.PublicConnUUIDStore
	var playerIdentityStore *persist.PlayerIdentityStore

	srv := netserver.NewServer(*addr, *buildVersion)

	// Wire map vote handlers for join popup menu integration
	joinPopupPlugin.SetMapVoteHandlers(
		func(connID int32, page int) {
			for _, c := range srv.ListConnectedConns() {
				if c.ConnID() == connID {
					mapVotePlugin.ShowVoteMenuForConn(c, page)
					return
				}
			}
		},
		func(connID int32, menuID, option int32) bool {
			for _, c := range srv.ListConnectedConns() {
				if c.ConnID() == connID {
					return mapVotePlugin.HandleVoteMenuChoiceForConn(c, menuID, option)
				}
			}
			return false
		},
	)
	applyAdmissionPolicy := func(loaded config.Config) error {
		var entries []netserver.AdmissionWhitelistEntry
		if loaded.Admin.WhitelistEnabled {
			loadedEntries, err := netserver.LoadAdmissionWhitelistFile(loaded.Admin.WhitelistFile)
			if err != nil {
				return err
			}
			entries = loadedEntries
		}
		srv.SetAdmissionPolicy(netserver.AdmissionPolicy{
			StrictIdentity:     loaded.Admin.StrictIdentity,
			AllowCustomClients: loaded.Admin.AllowCustomClients,
			PlayerLimit:        loaded.Admin.PlayerLimit,
			WhitelistEnabled:   loaded.Admin.WhitelistEnabled,
			Whitelist:          entries,
			ExpectedMods:       loaded.Mods.ExpectedClientMods,
			BannedNames:        loaded.Admin.BannedNames,
			BannedSubnets:      loaded.Admin.BannedSubnets,
			RecentKickDuration: time.Duration(loaded.Admin.RecentKickSeconds) * time.Second,
		})
		return nil
	}
	if err := applyAdmissionPolicy(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "加载 admission 配置失败: %v\n", err)
		os.Exit(1)
	}
	srv.SetVerboseNetLog(false)
	srv.SetPacketRecvEventsEnabled(cfg.Development.PacketRecvEventsEnabled)
	srv.SetPacketSendEventsEnabled(cfg.Development.PacketSendEventsEnabled)
	srv.SetTerminalPlayerLogsEnabled(cfg.Development.TerminalPlayerLogsEnabled)
	srv.SetTerminalPlayerUUIDEnabled(cfg.Development.TerminalPlayerUUIDEnabled)
	srv.SetRespawnPacketLogsEnabled(cfg.Development.RespawnPacketLogsEnabled)
	srv.SetPlayerNameColorEnabled(cfg.Personalization.PlayerNameColorEnabled)
	srv.SetTranslatedConnLog(cfg.Control.TranslatedConnLogEnabled)
	srv.SetJoinLeaveChatEnabled(cfg.Personalization.JoinLeaveChatEnabled)
	srv.OnTracePacket = func(direction string, c *netserver.Conn, obj any, packetID int, frameworkID int, size int) {
		tc := currentTraceCfg()
		if !tc.Enabled {
			return
		}
		switch direction {
		case "recv":
			if !tc.ClientRequestsEnabled {
				return
			}
			extra := map[string]any{}
			if c != nil {
				extra["conn_id"] = c.ConnID()
				extra["player_id"] = c.PlayerID()
				extra["uuid"] = c.UUID()
			}
			logTrace("client_request", "packet_recv", tracepoints.PacketFields(direction, obj, packetID, frameworkID, size, extra))
		case "send":
			if !tc.ServerSendsEnabled {
				return
			}
			extra := map[string]any{}
			if c != nil {
				extra["conn_id"] = c.ConnID()
				extra["player_id"] = c.PlayerID()
				extra["uuid"] = c.UUID()
			}
			logTrace("server_send", "packet_send", tracepoints.PacketFields(direction, obj, packetID, frameworkID, size, extra))
		}
	}
	srv.SetPlayerDisplayFormatter(func(c *netserver.Conn) string {
		if c == nil {
			return ""
		}
		return formatDisplayPlayerNameRaw(c.BaseName(), c, publicConnUUIDStore, playerIdentityStore)
	})
	srv.RefreshPlayerDisplayNames()
	var (
		effectIDMu      sync.RWMutex
		effectIDsByName = map[string]int16{}
	)
	setEffectIDs := func(ids *vanilla.ContentIDsFile) {
		next := make(map[string]int16)
		if ids != nil {
			for _, entry := range ids.Effects {
				name := strings.ToLower(strings.TrimSpace(entry.Name))
				if name == "" {
					continue
				}
				next[name] = entry.ID
			}
		}
		effectIDMu.Lock()
		effectIDsByName = next
		effectIDMu.Unlock()
	}
	lookupEffectID := func(name string) (int16, bool) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return 0, false
		}
		effectIDMu.RLock()
		id, ok := effectIDsByName[name]
		effectIDMu.RUnlock()
		return id, ok
	}
	contentIDsPath := filepath.Join(filepath.Dir(cfg.Runtime.VanillaProfiles), "content_ids.json")
	if ids, err := vanilla.LoadContentIDs(contentIDsPath); err != nil {
		startup.warn("原版 content IDs", fmt.Sprintf("未加载(%s): %v", canonicalRuntimePath(contentIDsPath), err))
	} else {
		setEffectIDs(ids)
		count := vanilla.ApplyContentIDs(srv.Content, ids)
		startup.ok("原版 content IDs", fmt.Sprintf("entries=%d path=%s", count, canonicalRuntimePath(contentIDsPath)))
	}
	srv.SetServerName(cfg.Runtime.ServerName)
	srv.SetServerDescription(cfg.Runtime.ServerDesc)
	srv.SetVirtualPlayers(int32(cfg.Runtime.VirtualPlayers))
	srv.UdpRetryCount = cfg.Net.UdpRetryCount
	srv.UdpRetryDelay = time.Duration(cfg.Net.UdpRetryDelayMs) * time.Millisecond
	srv.UdpFallbackTCP = cfg.Net.UdpFallbackTCP
	srv.SetSnapshotIntervals(cfg.Net.SyncEntityMs, cfg.Net.SyncStateMs)
	gameTPS := cfg.Core.TPS
	if gameTPS <= 0 {
		gameTPS = sim.DefaultTPS
	}
	wld := world.New(world.Config{
		TPS:                    gameTPS,
		UseMapSyncDataFallback: cfg.Sync.UseMapSyncDataFallback,
		BlockSyncLogsEnabled:   cfg.Sync.BlockSyncLogsEnabled,
	})

		// Initialize plugin context and start all plugins
		pluginCtx := plugin2.NewContext(
			plugin2.WrapServer(srv),
			plugin2.WrapWorld(wld),
			&cfg,
			pluginMgr.ConsoleCommands(),
			pluginMgr.ChatCommands(),
			pluginMgr.EventBus(),
			nil,
		)
		if err := pluginMgr.InitAll(pluginCtx); err != nil {
			fmt.Fprintf(os.Stderr, "plugin init: %v\n", err)
			os.Exit(1)
		}
		if err := pluginMgr.StartAll(); err != nil {
			fmt.Fprintf(os.Stderr, "plugin start: %v\n", err)
			os.Exit(1)
		}
	srv.EntitySnapshotHiddenFn = func(viewer *netserver.Conn, entity protocol.UnitSyncEntity) bool {
		if viewer == nil || wld == nil {
			return false
		}
		unit, ok := entity.(*protocol.UnitEntitySync)
		if !ok || unit == nil {
			return false
		}
		viewerX, viewerY := viewer.SnapshotPos()
		return wld.UnitSyncHiddenForViewer(world.TeamID(viewer.TeamID()), viewerX, viewerY, unit)
	}
	srv.TypeIO.BuildingLookup = func(pos int32) protocol.Building {
		if wld == nil {
			return nil
		}
		if wld.CanControlBuildingPacked(pos) {
			return protocol.ControlBuildingRef{
				PosValue: pos,
				UnitRef: protocol.BlockUnitRef{
					TileRef: protocol.BlockUnitTileRef{PosValue: pos},
				},
			}
		}
		if _, ok := wld.BuildingInfoPacked(pos); ok {
			return protocol.BuildingBox{PosValue: pos}
		}
		return nil
	}
	buildInfoToControlled := func(info world.BuildingInfo) netserver.ControlledBuildInfo {
		return netserver.ControlledBuildInfo{
			Pos:    info.Pos,
			X:      float32(info.X*8 + 4),
			Y:      float32(info.Y*8 + 4),
			TeamID: byte(info.Team),
		}
	}
	srv.ClaimControlledBuildFn = func(playerID int32, buildPos int32) (netserver.ControlledBuildInfo, bool) {
		info, ok := wld.ClaimControlledBuildingPacked(playerID, buildPos)
		if !ok {
			return netserver.ControlledBuildInfo{}, false
		}
		return buildInfoToControlled(info), true
	}
	srv.ControlledBuildInfoFn = func(playerID int32, buildPos int32) (netserver.ControlledBuildInfo, bool) {
		info, ok := wld.ControlledBuildingInfoPacked(playerID, buildPos)
		if !ok {
			return netserver.ControlledBuildInfo{}, false
		}
		return buildInfoToControlled(info), true
	}
	srv.ReleaseControlledBuildFn = func(playerID int32, buildPos int32) bool {
		if buildPos != 0 {
			if wld.ReleaseControlledBuildingPacked(playerID, buildPos) {
				return true
			}
		}
		return wld.ReleaseControlledBuildingByPlayer(playerID)
	}
	srv.SetControlledBuildInputFn = func(playerID int32, buildPos int32, aimX, aimY float32, shooting bool) bool {
		return wld.SetControlledBuildingInputPacked(playerID, buildPos, aimX, aimY, shooting)
	}
	// clientSnapshot writes client motion/position back into the authoritative world model.
	// This mirrors vanilla NetServer.clientSnapshot behavior; without it, entitySnapshot will
	// keep snapping units back to stale positions.
	srv.SetUnitMotionFn = func(unitID int32, vx, vy, rotVel float32) bool {
		_, ok := wld.SetEntityMotion(unitID, vx, vy, rotVel)
		return ok
	}
	srv.SetUnitPositionFn = func(unitID int32, x, y, rotation float32) bool {
		_, ok := wld.SetEntityPosition(unitID, x, y, rotation)
		return ok
	}
	srv.SetUnitRuntimeStateFn = func(unitID int32, state netserver.UnitRuntimeState) bool {
		_, ok := wld.SetEntityRuntimeState(unitID, state.Shooting, state.Boosting, state.UpdateBuilding, state.MineTilePos, state.Plans)
		return ok
	}
	srv.SetUnitStackFn = func(unitID int32, itemID int16, amount int32) bool {
		_, ok := wld.SetEntityStack(unitID, world.ItemID(itemID), amount)
		return ok
	}
	srv.SetUnitPlayerControllerFn = func(unitID int32, playerID int32) bool {
		_, ok := wld.SetEntityPlayerController(unitID, playerID)
		return ok
	}
	srv.OnRequestUnitPayload = func(c *netserver.Conn, targetID int32) {
		if c == nil || wld == nil || c.UnitID() == 0 || targetID == 0 {
			return
		}
		_, _ = wld.RequestUnitPayload(c.UnitID(), targetID)
	}
	srv.OnRequestBuildPayload = func(c *netserver.Conn, buildPos int32) {
		if c == nil || wld == nil || c.UnitID() == 0 || buildPos < 0 {
			return
		}
		_, _ = wld.RequestBuildPayloadPacked(c.UnitID(), buildPos)
	}
	srv.OnRequestDropPayload = func(c *netserver.Conn, x, y float32) {
		if c == nil || wld == nil || c.UnitID() == 0 {
			return
		}
		_, _ = wld.RequestDropPayload(c.UnitID(), x, y)
	}
	srv.OnRequestItem = func(c *netserver.Conn, pos int32, itemID int16, amount int32) {
		if c == nil || wld == nil || c.UnitID() == 0 || amount <= 0 {
			return
		}
		result, ok := wld.RequestItemFromBuildingPacked(c.UnitID(), pos, world.ItemID(itemID), amount)
		if !ok || result.Amount <= 0 {
			return
		}
		broadcastTakeItems(srv, pos, itemID, result.Amount, result.UnitID)
	}
	srv.OnTransferInventory = func(c *netserver.Conn, pos int32) {
		if c == nil || wld == nil || c.UnitID() == 0 {
			return
		}
		result, ok := wld.TransferUnitInventoryToBuildingPacked(c.UnitID(), pos)
		if !ok || result.Amount <= 0 {
			return
		}
		broadcastTransferItemTo(srv, result.UnitID, int16(result.Item), result.Amount, result.UnitX, result.UnitY, pos)
	}
	srv.OnDropItem = func(c *netserver.Conn, angle float32) {
		if c == nil || wld == nil || c.UnitID() == 0 || c.PlayerID() == 0 {
			return
		}
		if _, ok := wld.DropUnitItems(c.UnitID()); !ok {
			return
		}
		broadcastDropItem(srv, c.PlayerID(), angle)
	}
	srv.OnUnitEnteredPayload = func(c *netserver.Conn, unitID, buildPos int32) {
		if wld == nil || unitID == 0 || buildPos < 0 {
			return
		}
		if wld.EnterUnitPayloadPacked(buildPos, unitID) && c != nil && c.UnitID() == unitID {
			srv.ConsumeConnUnit(c, unitID)
		}
	}
	unitCommands := unitCmdPlugin.Svc
	buildService := buildsvc.New(wld, buildsvc.Options{
		MaxQueuedBatches: 256,
		MaxPlansPerBatch: 20,
		MaxOpsPerTick:    64,
	})
	shouldLogBuildSnapshots := func() bool {
		return cfg.Building.Enabled && cfg.Development.BuildSnapshotLogsEnabled
	}
	shouldLogBuildPlace := func() bool {
		return cfg.Building.Enabled && cfg.Development.BuildPlaceLogsEnabled
	}
	shouldLogBuildFinish := func() bool {
		return cfg.Building.Enabled && cfg.Development.BuildFinishLogsEnabled
	}
	shouldLogBuildBreakStart := func() bool {
		return cfg.Building.Enabled && cfg.Development.BuildBreakStartLogsEnabled
	}
	shouldLogBuildBreakDone := func() bool {
		return cfg.Building.Enabled && cfg.Development.BuildBreakDoneLogsEnabled
	}
	shouldLogRespawnCore := func() bool {
		return cfg.Development.RespawnCoreLogsEnabled
	}
	shouldLogRespawnUnit := func() bool {
		return cfg.Development.RespawnUnitLogsEnabled
	}
	shouldFileLogNetEvents := func() bool {
		return cfg.Sundries.NetEventLogsEnabled
	}
	shouldFileLogChat := func() bool {
		return cfg.Sundries.ChatLogsEnabled
	}
	shouldFileLogRespawnCore := func() bool {
		return cfg.Sundries.RespawnCoreLogsEnabled
	}
	shouldFileLogRespawnUnit := func() bool {
		return cfg.Sundries.RespawnUnitLogsEnabled
	}
	shouldFileLogBuildPlace := func() bool {
		return cfg.Sundries.BuildPlaceLogsEnabled
	}
	shouldFileLogBuildFinish := func() bool {
		return cfg.Sundries.BuildFinishLogsEnabled
	}
	shouldFileLogBuildBreakStart := func() bool {
		return cfg.Sundries.BuildBreakStartLogsEnabled
	}
	shouldFileLogBuildBreakDone := func() bool {
		return cfg.Sundries.BuildBreakDoneLogsEnabled
	}
	srv.SpawnUnitFn = func(c *netserver.Conn, unitID int32, tile protocol.Point2, unitType int16) (float32, float32, bool) {
		if c == nil || wld == nil {
			return 0, 0, false
		}
		team := resolveConnTeam(c, wld)
		if corePos, coreName, ok := resolveTeamCoreTileWithName(wld, team, tile); ok && shouldLogRespawnCore() {
			fmt.Printf("[重生] 玩家=%s 队伍=%d 核心=%s 核心坐标=(%d,%d) 出生点=(%d,%d)\n",
				displayPlayerName(c), team, translateBlockNameCN(coreName), corePos.X, corePos.Y, tile.X, tile.Y)
		} else if shouldLogRespawnCore() {
			if model := wld.CloneModel(); model != nil {
				if blockName := worldModelBlockNameAt(model, int(tile.X), int(tile.Y)); blockName != "" {
					fmt.Printf("[重生] 玩家=%s 队伍=%d 未找到核心，回退出生点=(%d,%d) 地块=%s\n",
						displayPlayerName(c), team, tile.X, tile.Y, translateBlockNameCN(blockName))
				}
			}
		}
		if corePos, coreName, ok := resolveTeamCoreTileWithName(wld, team, tile); ok && shouldFileLogRespawnCore() {
			detailLog.LogLine(fmt.Sprintf("%s [RESPAWN] player=%q team=%d core=%s core_x=%d core_y=%d spawn_x=%d spawn_y=%d",
				time.Now().Format(time.RFC3339Nano), c.Name(), team, strings.ToLower(strings.TrimSpace(coreName)), corePos.X, corePos.Y, tile.X, tile.Y))
		} else if shouldFileLogRespawnCore() {
			blockName := ""
			if model := wld.CloneModel(); model != nil {
				blockName = worldModelBlockNameAt(model, int(tile.X), int(tile.Y))
			}
			detailLog.LogLine(fmt.Sprintf("%s [RESPAWN] player=%q team=%d no_core=1 spawn_x=%d spawn_y=%d tile=%s",
				time.Now().Format(time.RFC3339Nano), c.Name(), team, tile.X, tile.Y, blockName))
		}
		spawnUnitType := unitType
		if spawnUnitType <= 0 {
			spawnUnitType = defaultPlayerRespawnUnitID
			if alphaID, ok := wld.ResolveUnitTypeID("alpha"); ok {
				spawnUnitType = alphaID
			}
		}
		spawnUnitType = resolveRespawnUnitTypeByCoreTile(wld, tile, team, spawnUnitType)
		builderSpeed := wld.BuilderSpeedForUnitType(spawnUnitType)
		wld.SetTeamBuilderSpeed(team, builderSpeed)
		if shouldLogRespawnUnit() {
			fmt.Printf("[重生] 玩家=%s 队伍=%d 出生单位=%d 建造速度=%.2f 出生点=(%d,%d)\n",
				displayPlayerName(c), team, spawnUnitType, builderSpeed, tile.X, tile.Y)
		}
		if shouldFileLogRespawnUnit() {
			detailLog.LogLine(fmt.Sprintf("%s [RESPAWN] player=%q team=%d unit=%d build_speed=%.2f spawn_x=%d spawn_y=%d",
				time.Now().Format(time.RFC3339Nano), c.Name(), team, spawnUnitType, builderSpeed, tile.X, tile.Y))
		}
		x := float32(tile.X*8 + 4)
		y := float32(tile.Y*8 + 4)
		ent, err := wld.AddEntityWithID(spawnUnitType, unitID, x, y, team)
		if err != nil {
			return 0, 0, false
		}
		_, _ = wld.SetEntitySpawnedByCore(ent.ID, true)
		_, _ = wld.SetEntityPlayerController(ent.ID, c.PlayerID())
		return x, y, true
	}
	srv.SpawnUnitAtFn = func(c *netserver.Conn, unitID int32, x, y, rotation float32, unitType int16, spawnedByCore bool) (float32, float32, bool) {
		if c == nil || wld == nil || unitType <= 0 {
			return 0, 0, false
		}
		team := resolveConnTeam(c, wld)
		builderSpeed := wld.BuilderSpeedForUnitType(unitType)
		wld.SetTeamBuilderSpeed(team, builderSpeed)
		ent, err := wld.AddEntityWithID(unitType, unitID, x, y, team)
		if err != nil {
			return 0, 0, false
		}
		_, _ = wld.SetEntitySpawnedByCore(ent.ID, spawnedByCore)
		_, _ = wld.SetEntityPosition(ent.ID, x, y, rotation)
		_, _ = wld.SetEntityPlayerController(ent.ID, c.PlayerID())
		return x, y, true
	}
	srv.ResolveRespawnUnitTypeFn = func(c *netserver.Conn, tile protocol.Point2, fallback int16) int16 {
		if wld == nil {
			return fallback
		}
		return resolveRespawnUnitTypeByCoreTile(wld, tile, resolveConnTeam(c, wld), fallback)
	}
	srv.ReserveUnitIDFn = func() int32 {
		if wld == nil {
			return 0
		}
		return wld.ReserveEntityID()
	}
	srv.DropUnitFn = func(unitID int32) {
		if wld == nil {
			return
		}
		wld.RemoveEntity(unitID)
	}
	srv.UnitInfoFn = func(unitID int32) (netserver.UnitInfo, bool) {
		if wld == nil {
			return netserver.UnitInfo{}, false
		}
		ent, ok := wld.GetEntity(unitID)
		if !ok {
			return netserver.UnitInfo{}, false
		}
		return netserver.UnitInfo{
			ID:        ent.ID,
			X:         ent.X,
			Y:         ent.Y,
			Health:    ent.Health,
			MaxHealth: ent.MaxHealth,
			TeamID:    byte(ent.Team),
			TypeID:    ent.TypeID,
		}, true
	}
	srv.UnitSyncFn = func(unitID int32, controller protocol.UnitController) (*protocol.UnitEntitySync, bool) {
		if wld == nil {
			return nil, false
		}
		return wld.UnitSyncSnapshot(srv.Content, unitID, controller)
	}
	var unitNamesByID map[int16]string
	var loadedModel *world.WorldModel
	var loadedMapPath string
	invalidateWorldCache := func() {}

	var playerSpawnTypeID int32 = int32(defaultPlayerRespawnUnitID)
	if err := wld.LoadVanillaProfiles(cfg.Runtime.VanillaProfiles); err != nil {
		log.Warn("vanilla profiles load failed", logging.Field{Key: "path", Value: cfg.Runtime.VanillaProfiles}, logging.Field{Key: "error", Value: err.Error()})
		startup.warn("原版 profiles", fmt.Sprintf("加载失败: %s", err.Error()))
	} else if strings.TrimSpace(cfg.Runtime.VanillaProfiles) != "" {
		startup.ok("原版 profiles", canonicalRuntimePath(cfg.Runtime.VanillaProfiles))
	}
	loadWorldModel := func(path string) {
		path = canonicalRuntimePath(path)
		runtimeWorldPath.Store(path)
		actualPath := resolveRuntimePath(path)
		buildService.Reset()
		lower := strings.ToLower(path)
		if !strings.HasSuffix(lower, ".msav") && !strings.HasSuffix(lower, ".msav.msav") {
			wld.SetModel(nil)
			loadedModel = nil
			loadedMapPath = ""
			return
		}
		model, lerr := worldstream.LoadWorldModelFromMSAV(actualPath, srv.Content)
		if lerr != nil {
			log.Warn("world model load failed", logging.Field{Key: "path", Value: path}, logging.Field{Key: "error", Value: lerr.Error()})
			startup.warn("地图模型", fmt.Sprintf("加载失败: %s", lerr.Error()))
			loadedModel = nil
			loadedMapPath = ""
			return
		}
		wld.SetModel(model)
		if srv.Content != nil && model != nil {
			mapBlockRegs := 0
			for id, name := range model.BlockNames {
				normalized := strings.ToLower(strings.TrimSpace(name))
				if normalized == "" {
					continue
				}
				srv.Content.RegisterBlock(blockRef{id: id, name: normalized})
				mapBlockRegs++
			}
			mapUnitRegs := 0
			for id, name := range model.UnitNames {
				normalized := strings.ToLower(strings.TrimSpace(name))
				if normalized == "" {
					continue
				}
				srv.Content.RegisterUnitType(unitTypeRef{id: id, name: normalized})
				mapUnitRegs++
			}
			if mapBlockRegs > 0 || mapUnitRegs > 0 {
				startup.info("地图内容注册", fmt.Sprintf("blocks=%d units=%d", mapBlockRegs, mapUnitRegs))
			}
		}
		loadedModel = model
		loadedMapPath = path
		startup.ok("地图模型", fmt.Sprintf("%s (%dx%d)", path, model.Width, model.Height))
		if summary := world.DescribeRuleMode(model, wld.GetRulesManager().Get()); summary.Mode != "" {
			modeName := summary.ModeName
			if modeName == "" {
				modeName = "-"
			}
			startup.info("地图模式", fmt.Sprintf("mode=%s modeName=%s waves=%v waveTimer=%v pvp=%v attack=%v editor=%v infiniteResources=%v infiniteAmmo=%v",
				summary.Mode,
				modeName,
				summary.Waves,
				summary.WaveTimer,
				summary.Pvp,
				summary.AttackMode,
				summary.Editor,
				summary.InfiniteResources,
				summary.InfiniteAmmo,
			))
		}
		if model != nil && len(model.UnitNames) > 0 {
			unitNamesByID = make(map[int16]string, len(model.UnitNames))
			for k, v := range model.UnitNames {
				unitNamesByID[k] = strings.ToLower(strings.TrimSpace(v))
			}
			startup.ok("单位 ID 列表", fmt.Sprintf("count=%d", len(unitNamesByID)))
		}
		spawnType := defaultPlayerRespawnUnitID
		if alphaID, ok := wld.ResolveUnitTypeID("alpha"); ok {
			spawnType = alphaID
		}
		atomic.StoreInt32(&playerSpawnTypeID, int32(spawnType))
		for rawTeam := 1; rawTeam <= 255; rawTeam++ {
			team := world.TeamID(rawTeam)
			corePos, ok := resolveTeamCoreTile(wld, team, protocol.Point2{})
			if !ok {
				continue
			}
			unitType := resolveRespawnUnitTypeByCoreTile(wld, corePos, team, spawnType)
			if speed := wld.BuilderSpeedForUnitType(unitType); speed > 0 {
				wld.SetTeamBuilderSpeed(team, speed)
			}
		}
		startup.ok("玩家出生单位", fmt.Sprintf("typeId=%d", spawnType))
	}
	var cache *worldCache
	applyVotedWorld := func(next string) error {
		state.set(next)
		loadWorldModel(next)
		if invalidateWorldCache != nil {
			invalidateWorldCache()
		}
		reloaded, failed := srv.ReloadWorldLiveForAll()
		mapName := worldstream.TrimMapName(filepath.Base(next))
		if reloaded == 0 && failed == 0 {
			srv.BroadcastChat(fmt.Sprintf("[accent]地图已切换[]: [white]%s[]（当前无在线玩家）", mapName))
			return nil
		}
		srv.BroadcastChat(fmt.Sprintf("[accent]地图已切换[]: [white]%s[]（成功=%d 失败=%d）", mapName, reloaded, failed))
		return nil
	}
	mapVotePlugin.SetCallbacks(listWorldMaps, resolveWorldSelection, applyVotedWorld, nil)
	loadWorldModel(initialWorld)
	if restoredSnapshotOK {
		waveTime := normalizeSnapshotWaveTimeSeconds(wld, restoredSnapshot.WaveTime)
		wld.ApplySnapshot(world.Snapshot{
			WaveTime: waveTime,
			Wave:     restoredSnapshot.Wave,
			TimeData: restoredSnapshot.TimeData,
			Tps:      int8(gameTPS),
			Rand0:    restoredSnapshot.Rand0,
			Rand1:    restoredSnapshot.Rand1,
			Tick:     restoredSnapshot.Tick,
		})
	}
	srv.MapNameFn = func() string {
		path := state.get()
		if path == "" {
			return "unknown"
		}
		return worldstream.TrimMapName(filepath.Base(path))
	}
	recorder, rerr := storage.NewRecorder(cfg.Storage)
	if rerr != nil {
		fmt.Fprintf(os.Stderr, "事件存储初始化失败: %v\n", rerr)
		os.Exit(1)
	}
	runtimePublicConnUUIDEnabled.Store(cfg.Control.PublicConnUUIDEnabled)
	publicConnUUIDPath := resolveConfigSidecarPath(runtimeConfigDir, cfg.Control.PublicConnUUIDFile)
	var publicConnUUIDErr error
	publicConnUUIDStore, publicConnUUIDErr = persist.NewPublicConnUUIDStore(publicConnUUIDPath, cfg.Control.ConnUUIDAutoCreateEnabled)
	if publicConnUUIDErr != nil {
		log.Warn("public conn_uuid store init failed", logging.Field{Key: "error", Value: publicConnUUIDErr.Error()})
		startup.warn("公开 conn_uuid", publicConnUUIDErr.Error())
	} else {
		runtimePublicConnUUIDStore = publicConnUUIDStore
	}
	playerIdentityPath := resolveConfigSidecarPath(runtimeConfigDir, cfg.Personalization.PlayerIdentityFile)
	playerIdentityStore, publicIdentityErr := persist.NewPlayerIdentityStore(playerIdentityPath, cfg.Control.PlayerIdentityAutoCreateEnabled)
	if publicIdentityErr != nil {
		log.Warn("player identity store init failed", logging.Field{Key: "error", Value: publicIdentityErr.Error()})
		startup.warn("玩家身份配置", publicIdentityErr.Error())
	} else {
		runtimePlayerIdentityStore = playerIdentityStore
	}
	srv.OnConnectAccepted = func(conn *netserver.Conn, pkt *protocol.ConnectPacket) {
		if conn == nil {
			return
		}
		sourceName := strings.TrimSpace(conn.BaseName())
		if sourceName == "" && pkt != nil {
			sourceName = strings.TrimSpace(pkt.Name)
		}
		_, _ = ensureConnIdentityRecords(publicConnUUIDStore, playerIdentityStore, conn.UUID(), sourceName, connRemoteIP(conn))
	}
	runtimeBindStatusResolver = newBindStatusResolver(
		cfg.Personalization.PlayerBindSource,
		cfg.Personalization.PlayerBindAPIURL,
		time.Duration(cfg.Personalization.PlayerBindAPITimeoutMs)*time.Millisecond,
		time.Duration(cfg.Personalization.PlayerBindAPICacheSec)*time.Second,
		playerIdentityStore,
	)
	srv.OnEvent = func(ev netserver.NetEvent) {
		if publicConnUUIDStore != nil && ev.Kind == "connect_packet" && runtimePublicConnUUIDEnabled.Load() {
			if current := publicConnUUIDValue(publicConnUUIDStore, ev.UUID); current == "" {
				name := strings.TrimSpace(netserver.StripMindustryColorTags(ev.Name))
				_, _ = ensureConnIdentityRecords(publicConnUUIDStore, playerIdentityStore, ev.UUID, name, ev.IP)
			}
		}
		ev.Detail = appendConnectionCheckpointDetail(ev.Detail, ev, publicConnUUIDStore, playerIdentityStore)
		_ = recorder.Record(storage.Event{
			Timestamp: ev.Timestamp,
			Kind:      ev.Kind,
			Packet:    ev.Packet,
			Detail:    ev.Detail,
			ConnID:    ev.ConnID,
			UUID:      ev.UUID,
			IP:        ev.IP,
			Name:      ev.Name,
		})
		line := fmt.Sprintf("%s [NET] kind=%s packet=%s conn_id=%s uuid=%s ip=%s name=%q detail=%s",
			ev.Timestamp.Format(time.RFC3339Nano), ev.Kind, ev.Packet, publicConnIDValue(publicConnUUIDStore, ev.UUID, ev.ConnID), ev.UUID, ev.IP, ev.Name, ev.Detail)
		if shouldFileLogNetEvents() {
			detailLog.LogLine(line)
		}
		tc := currentTraceCfg()
		if tc.Enabled {
			if tc.ClientRequestsEnabled && (ev.Kind == "packet_recv" || ev.Kind == "connect_packet" || strings.HasPrefix(ev.Kind, "connect_confirm") || strings.Contains(ev.Kind, "client_snapshot")) {
				logTrace("client_request", ev.Kind, map[string]any{
					"packet":  ev.Packet,
					"detail":  ev.Detail,
					"conn_id": ev.ConnID,
					"uuid":    ev.UUID,
					"ip":      ev.IP,
					"name":    ev.Name,
				})
			}
			if tc.ServerSendsEnabled && (ev.Kind == "packet_send" || ev.Kind == "world_handshake_sent" || strings.Contains(ev.Kind, "state_snapshot") || strings.Contains(ev.Kind, "entity_snapshot")) {
				logTrace("server_send", ev.Kind, map[string]any{
					"packet":  ev.Packet,
					"detail":  ev.Detail,
					"conn_id": ev.ConnID,
					"uuid":    ev.UUID,
					"ip":      ev.IP,
					"name":    ev.Name,
				})
			}
		}
	}
	srv.SetPublicConnIDFormatter(func(c *netserver.Conn) string {
		if c == nil || !runtimePublicConnUUIDEnabled.Load() {
			return ""
		}
		if publicConnUUIDStore == nil {
			return ""
		}
		id, ok := publicConnUUIDStore.Lookup(c.UUID())
		if !ok {
			return ""
		}
		return id
	})
	if ops, ok, err := persist.LoadOps(cfg.Admin); err != nil {
		log.Warn("ops load failed", logging.Field{Key: "error", Value: err.Error()})
		startup.warn("OP 列表", err.Error())
	} else if ok {
		for _, u := range ops {
			srv.AddOp(u)
		}
		startup.ok("OP 列表", fmt.Sprintf("count=%d", len(ops)))
	}
	saveOps := func() {
		_ = persist.SaveOps(cfg.Admin, srv.ListOps())
	}
	var serverCore *coreio.ServerCore

	cache = &worldCache{content: srv.Content}
	invalidateWorldCache = func() {
		cache.invalidate()
		if err := warmWorldCache(cache, state.get()); err != nil {
			log.Warn("world cache warm failed", logging.Field{Key: "path", Value: state.get()}, logging.Field{Key: "error", Value: err.Error()})
		}
	}
	if err := warmWorldCache(cache, state.get()); err != nil {
		log.Warn("world cache warm failed", logging.Field{Key: "path", Value: state.get()}, logging.Field{Key: "error", Value: err.Error()})
		startup.warn("世界流缓存", err.Error())
	} else {
		startup.ok("世界流缓存", state.get())
	}
	srv.WorldDataFn = func(conn *netserver.Conn, _ *protocol.ConnectPacket) ([]byte, error) {
		var ioCore *coreio.Core2
		if serverCore != nil {
			ioCore = serverCore.Core2
		}
		payload, err := buildInitialWorldDataPayload(conn, wld, cache, state.get(), ioCore)
		if err == nil {
			tc := currentTraceCfg()
			if tc.Enabled && tc.WorldStreamEnabled {
				playerID := int32(0)
				connID := int32(0)
				liveStream := false
				if conn != nil {
					playerID = conn.PlayerID()
					connID = conn.ConnID()
					liveStream = conn.UsesLiveWorldStream()
				}
				snap := world.Snapshot{}
				if wld != nil {
					snap = wld.Snapshot()
				}
				logTrace("world_stream", "build_initial_world_data_payload", map[string]any{
					"conn_id":          connID,
					"player_id":        playerID,
					"map_path":         state.get(),
					"payload_bytes":    len(payload),
					"live_worldstream": liveStream,
					"wave":             snap.Wave,
					"wave_time":        snap.WaveTime,
					"tick":             snap.Tick,
				})
			}
		}
		return payload, err
	}
	srv.OnPostConnect = func(conn *netserver.Conn) {
		if conn == nil {
			return
		}
		mapPath := state.get()
		syncPostConnectWorldStateToConn(srv, conn, wld, cache.model(mapPath), mapPath, cfg.Sync.Strategy)
		pluginMgr.EventBus().DispatchPlayerJoin(plugin2.WrapConn(conn))
		// Keep connect grace long enough for the official client to finish
		// applying the streamed world and rebinding its spawned unit.
		conn.SetWorldReloadGrace(2 * time.Second)
	}
	srv.OnMenuChoose = func(c *netserver.Conn, menuID, option int32) {
		joinPopupPlugin.HandleMenuChoice(plugin2.WrapConn(c), menuID, option)
	}
	srv.OnHotReloadConnFn = func(conn *netserver.Conn) {
		if conn == nil {
			return
		}
		mapPath := state.get()
		syncPostConnectWorldStateToConn(srv, conn, wld, cache.model(mapPath), mapPath, cfg.Sync.Strategy)
		srv.RefreshPlayerDisplayNames()
		conn.SetWorldReloadGrace(2 * time.Second)
	}
	srv.SpawnTileFn = func() (protocol.Point2, bool) {
		if pos, ok := resolveTeamCoreTile(wld, resolveDefaultPlayerTeam(wld), protocol.Point2{}); ok {
			return pos, true
		}
		pos, ok, err := cache.spawnPos(state.get())
		if err == nil && ok {
			return pos, true
		}
		// Fallback for maps where core tile cannot be parsed from msav metadata.
		return fallbackSpawnPosFromModel(wld.CloneModel())
	}
	srv.AssignTeamForConnFn = func(c *netserver.Conn) byte {
		return byte(assignConnTeamVanilla(srv, wld, c))
	}
	spawnRefForConn := func(c *netserver.Conn) protocol.Point2 {
		if c == nil {
			return protocol.Point2{}
		}
		if wld != nil {
			if unitID := c.UnitID(); unitID != 0 {
				if ent, ok := wld.GetEntity(unitID); ok {
					return protocol.Point2{
						X: int32(math.Round((float64(ent.X) - 4) / 8)),
						Y: int32(math.Round((float64(ent.Y) - 4) / 8)),
					}
				}
			}
		}
		x, y := c.SnapshotPos()
		if x == 0 && y == 0 {
			return protocol.Point2{}
		}
		return protocol.Point2{
			X: int32(math.Round((float64(x) - 4) / 8)),
			Y: int32(math.Round((float64(y) - 4) / 8)),
		}
	}
	srv.SpawnTileForConnFn = func(c *netserver.Conn) (protocol.Point2, bool) {
		team := resolveConnTeam(c, wld)
		if pos, ok := resolveTeamCoreTile(wld, team, spawnRefForConn(c)); ok {
			return pos, true
		}
		if wld != nil {
			if pos, ok := fallbackSpawnPosFromModel(wld.CloneModel()); ok {
				return pos, true
			}
		}
		return srv.SpawnTileFn()
	}
	// Official 157 build authority comes from clientSnapshot queue updates.
	// Keep the old shared OnBuildPlans queue disabled so cancelled plans are removed
	// by authoritative snapshot reconciliation instead of lingering in a second queue.
	srv.OnBuildPlans = nil
	type snapshotLogKey struct {
		count    int
		breaking bool
		x        int32
		y        int32
		blockID  int16
	}
	var (
		snapshotLogMu sync.Mutex
		snapshotLogBy = make(map[int32]snapshotLogKey)
		ownerActorMu  sync.RWMutex
		ownerActorBy  = make(map[int32]string)
	)
	buildActor := func(owner int32, team world.TeamID) string {
		if owner != 0 {
			ownerActorMu.RLock()
			actor := strings.TrimSpace(ownerActorBy[owner])
			ownerActorMu.RUnlock()
			if actor != "" {
				return actor
			}
		}
		return fmt.Sprintf("team-%d", team)
	}
	rememberBuildOwner := func(c *netserver.Conn, owner int32) {
		if c == nil || owner == 0 {
			return
		}
		ownerActorMu.Lock()
		ownerActorBy[owner] = displayPlayerName(c)
		ownerActorMu.Unlock()
	}
	handleCoreBuildingControlSelect := func(c *netserver.Conn, info world.BuildingInfo) {
		if c == nil || wld == nil || c.IsDead() || c.UnitID() == 0 {
			return
		}
		if !strings.HasPrefix(info.Name, "core-") || info.Team != resolveConnTeam(c, wld) {
			return
		}
		_ = srv.HandleCoreBuildingControlSelect(c, protocol.UnpackPoint2(info.Pos))
	}
	handlePlayerPayloadBuildingControlSelect := func(c *netserver.Conn, info world.BuildingInfo) {
		if c == nil || wld == nil || c.IsDead() || c.UnitID() == 0 {
			return
		}
		unitID := c.UnitID()
		if !wld.ControlSelectPayloadUnitPacked(info.Pos, unitID) {
			return
		}
		srv.ConsumeConnUnit(c, unitID)
		broadcastRelatedBlockSnapshots(srv, wld, info.Pos)
	}
	handleUnitPayloadBuildingControlSelect := func(c *netserver.Conn, info world.BuildingInfo, unitID int32) {
		if wld == nil || unitID == 0 {
			return
		}
		if !wld.ControlSelectPayloadUnitPacked(info.Pos, unitID) {
			return
		}
		if c != nil && c.UnitID() == unitID {
			srv.ConsumeConnUnit(c, unitID)
		}
		broadcastRelatedBlockSnapshots(srv, wld, info.Pos)
	}
	srv.OnRotateBlock = func(c *netserver.Conn, pos int32, direction bool) {
		res, ok := wld.RotateBuildingPacked(pos, direction)
		if !ok {
			return
		}
		broadcastSetTile(srv, pos, res.BlockID, res.Rotation, byte(res.Team))
		if effectID, ok := lookupEffectID("rotateblock"); ok {
			broadcastEffectReliable(srv, effectID, res.EffectX, res.EffectY, res.EffectRot)
		}
		broadcastRelatedBlockSnapshots(srv, wld, pos)
	}
	srv.OnRequestBlockSnapshot = func(c *netserver.Conn, pos int32) {
		if c == nil || wld == nil {
			return
		}
		if info, ok := wld.BuildingInfoPacked(pos); ok {
			if info.Team != resolveConnTeam(c, wld) {
				return
			}
		}
		sendRequestedBlockSnapshotToConn(c, wld, pos)
	}
	srv.OnBuildingControlSelect = func(c *netserver.Conn, pos int32) {
		if wld == nil || c == nil {
			return
		}
		if !wld.CanControlSelectBuildingPacked(pos) {
			return
		}
		info, ok := wld.BuildingInfoPacked(pos)
		if !ok {
			return
		}
		switch {
		case strings.HasPrefix(info.Name, "core-"):
			handleCoreBuildingControlSelect(c, info)
		default:
			handlePlayerPayloadBuildingControlSelect(c, info)
		}
	}
	srv.OnUnitBuildingControlSelect = func(c *netserver.Conn, unitID, pos int32) {
		if wld == nil || unitID == 0 {
			return
		}
		if !wld.CanControlSelectBuildingPacked(pos) {
			return
		}
		info, ok := wld.BuildingInfoPacked(pos)
		if !ok || strings.HasPrefix(info.Name, "core-") {
			return
		}
		handleUnitPayloadBuildingControlSelect(c, info, unitID)
	}
	srv.OnBuildPlanSnapshot = func(c *netserver.Conn, plans []*protocol.BuildPlan) {
		if c == nil {
			return
		}
		owner := resolveBuildOwner(c)
		team := resolveConnTeam(c, wld)
		syncBuilderStateFromConnSnapshot(wld, c, owner, team, plans, false)
		rememberBuildOwner(c, owner)
		if len(plans) == 0 {
			key := snapshotLogKey{count: 0}
			snapshotLogMu.Lock()
			prev, ok := snapshotLogBy[c.PlayerID()]
			changed := !ok || prev != key
			if changed {
				snapshotLogBy[c.PlayerID()] = key
			}
			snapshotLogMu.Unlock()
			if changed && shouldLogBuildSnapshots() && !cfg.Building.Translated {
				fmt.Printf("[buildtrace] recv snapshot player=%d remote=%s count=0\n", c.PlayerID(), c.RemoteAddr().String())
			}
		} else {
			first := plans[0]
			blockID := int16(0)
			if first != nil && !first.Breaking && first.Block != nil {
				blockID = first.Block.ID()
			}
			if first != nil {
				key := snapshotLogKey{
					count:    len(plans),
					breaking: first.Breaking,
					x:        first.X,
					y:        first.Y,
					blockID:  blockID,
				}
				snapshotLogMu.Lock()
				prev, ok := snapshotLogBy[c.PlayerID()]
				changed := !ok || prev != key
				if changed {
					snapshotLogBy[c.PlayerID()] = key
				}
				snapshotLogMu.Unlock()
				if changed && shouldLogBuildSnapshots() {
					if cfg.Building.Translated {
						action := "建造"
						if first.Breaking {
							action = "拆除"
						}
						fmt.Printf("[建筑] 玩家=%s 快照队列=%d 首项=(x%d-y%d) 动作=%s block=%d(%s) team=%d\n",
							displayPlayerName(c), len(plans), first.X, first.Y, action, blockID, blockDisplayName(wld, blockID), team)
					} else {
						fmt.Printf("[buildtrace] recv snapshot player=%d remote=%s count=%d first_break=%v first_xy=(%d,%d) first_block=%d\n",
							c.PlayerID(), c.RemoteAddr().String(), len(plans), first.Breaking, first.X, first.Y, blockID)
					}
				}
			}
		}
		buildService.SyncPlans(owner, team, plans)
	}
	srv.OnDeletePlans = func(c *netserver.Conn, positions []int32) {
		owner := resolveBuildOwner(c)
		if c != nil && len(positions) > 0 && shouldLogBuildSnapshots() && !cfg.Building.Translated {
			fmt.Printf("[buildtrace] recv deletePlans player=%d remote=%s count=%d\n", c.PlayerID(), c.RemoteAddr().String(), len(positions))
		}
		buildService.CancelPositions(owner, positions)
		wld.CancelBuildPlansPackedForOwner(owner, positions)
	}
	srv.OnRemoveQueueBlock = func(c *netserver.Conn, x, y int32, breaking bool) {
		if c == nil {
			return
		}
		owner := resolveBuildOwner(c)
		if cfg.Building.Translated {
			action := "取消建造"
			if breaking {
				action = "取消拆除"
			}
			fmt.Printf("[建筑] 玩家=%s (x%d-y%d) %s\n", displayPlayerName(c), x, y, action)
		}
		if shouldLogBuildSnapshots() && !cfg.Building.Translated {
			fmt.Printf("[buildtrace] recv removeQueue player=%d remote=%s xy=(%d,%d) breaking=%v\n", c.PlayerID(), c.RemoteAddr().String(), x, y, breaking)
		}
		buildService.CancelPositions(owner, []int32{protocol.PackPoint2(x, y)})
		wld.CancelBuildAtForOwner(owner, x, y, breaking)
	}
	srv.OnCommandUnits = func(c *netserver.Conn, unitIDs []int32, buildTarget any, unitTarget any, posTarget any, queueCommand bool, _ bool) {
		unitCommands.ApplyCommandUnits(c, wld, unitIDs, buildTarget, unitTarget, posTarget, queueCommand)
	}
	srv.OnSetUnitCommand = func(c *netserver.Conn, unitIDs []int32, command *protocol.UnitCommand) {
		unitCommands.ApplySetUnitCommand(c, wld, unitIDs, command)
	}
	srv.OnSetUnitStance = func(c *netserver.Conn, unitIDs []int32, stance protocol.UnitStance, enable bool) {
		unitCommands.ApplySetUnitStance(c, wld, unitIDs, stance, enable)
	}
	srv.OnCommandBuilding = func(c *netserver.Conn, buildings []int32, target protocol.Vec2) {
		wld.CommandBuildingsPacked(buildings, target)
		for _, pos := range buildings {
			broadcastRelatedBlockSnapshots(srv, wld, pos)
		}
	}
	srv.OnTileConfig = func(c *netserver.Conn, pos int32, value any) {
		wld.ConfigureBuildingPacked(pos, value)
		if normalized, ok := wld.BuildingConfigPacked(pos); ok {
			srv.BroadcastTileConfig(pos, normalized, c)
		} else {
			srv.BroadcastTileConfig(pos, value, c)
		}
		broadcastRelatedBlockSnapshots(srv, wld, pos)
	}
	srv.PlayerUnitTypeFn = func() int16 {
		return int16(atomic.LoadInt32(&playerSpawnTypeID))
	}
	srv.StateSnapshotFn = func() *protocol.Remote_NetClient_stateSnapshot_35 {
		snap := wld.Snapshot()
		tc := currentTraceCfg()
		if tc.Enabled && tc.StateBuildEnabled {
			logTrace("state_build", "build_state_snapshot", map[string]any{
				"wave":      snap.Wave,
				"wave_time": snap.WaveTime,
				"tick":      snap.Tick,
				"time_data": snap.TimeData,
				"tps":       snap.Tps,
			})
		}
		return &protocol.Remote_NetClient_stateSnapshot_35{
			// Mindustry state.wavetime is in simulation tick units and should match
			// the advertised TPS, otherwise the client countdown runs at the wrong rate
			// between authoritative snapshot updates.
			WaveTime: snap.WaveTimeTicks(),
			Wave:     snap.Wave,
			Enemies:  snap.Enemies,
			Paused:   snap.Paused,
			GameOver: snap.GameOver,
			TimeData: snap.TimeData,
			Tps:      snap.Tps,
			Rand0:    snap.Rand0,
			Rand1:    snap.Rand1,
			CoreData: buildCoreSnapshotData(wld),
		}
	}
	srv.ExtraEntitySnapshotEntitiesFn = func() ([]protocol.UnitSyncEntity, error) {
		return unitCommands.Overlay(wld.EntitySyncSnapshots(srv.Content, srv.PlayerUnitIDSet())), nil
	}
	traceWorldRuntimeTick := func(stage string) {
		tc := currentTraceCfg()
		if !tc.Enabled || !tc.WorldRuntimeEnabled || wld == nil {
			return
		}
		st := wld.TraceRuntimeState()
		logTrace("world_runtime", stage, map[string]any{
			"tick":           st.Tick,
			"wave":           st.Wave,
			"wave_time":      st.WaveTime,
			"time_data":      st.TimeData,
			"tps":            st.TPS,
			"active_tiles":   st.ActiveTiles,
			"entities":       st.Entities,
			"bullets":        st.Bullets,
			"pending_builds": st.PendingBuilds,
			"pending_breaks": st.PendingBreaks,
		})
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				fmt.Printf("[net] event-loop panic err=%v\n", rec)
			}
		}()
		t := time.NewTicker(time.Second / time.Duration(gameTPS))
		defer t.Stop()
		nextBlockSnapshotSync := time.Now().Add(6 * time.Second)
		nextPlanPreviewSync := time.Now()
		eventBuf := make([]world.EntityEvent, 0, 1024)
		for range t.C {
			now := time.Now()
			if !now.Before(nextPlanPreviewSync) {
				srv.BroadcastStoredClientPlanPreviewsAt(now)
				nextPlanPreviewSync = now.Add(500 * time.Millisecond)
			}
			eventBuf = wld.DrainEntityEventsInto(eventBuf)
			evs := eventBuf
			groupedExplosionBuilds := classifyReactorExplosionBuilds(wld, evs)
			buildHealth := make([]int32, 0, len(evs)*2)
			blockItemSync := make(map[int32]struct{})
			itemTurretAmmoSync := make(map[int32]struct{})
			for i := range evs {
				ev := evs[i]
				switch ev.Kind {
				case world.EntityEventRemoved:
					unitCommands.Remove(ev.Entity.ID)
					broadcastUnitDestroy(srv, ev.Entity.ID)
					if ev.Entity.Health <= 0 {
						if _, ok := srv.PlayerUnitIDSet()[ev.Entity.ID]; ok {
							fmt.Printf("[net] world removed player-unit=%d hp=%.2f pos=(%.1f,%.1f) team=%d\n",
								ev.Entity.ID, ev.Entity.Health, ev.Entity.X, ev.Entity.Y, ev.Entity.Team)
						}
						srv.MarkUnitDead(ev.Entity.ID, "world-removed")
					} else {
						if _, ok := srv.PlayerUnitIDSet()[ev.Entity.ID]; ok {
							fmt.Printf("[net] ignored unit removal conn-unit=%d source=world-removed-positive-health hp=%.2f pos=(%.1f,%.1f)\n",
								ev.Entity.ID, ev.Entity.Health, ev.Entity.X, ev.Entity.Y)
						}
					}
				case world.EntityEventBuildPlaced:
					x, y := unpackTilePos(ev.BuildPos)
					if shouldFileLogBuildPlace() {
						detailLog.LogLine(fmt.Sprintf("%s [BUILD] action=placed x=%d y=%d block_id=%d block=%s team=%d rot=%d",
							time.Now().Format(time.RFC3339Nano), x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam, ev.BuildRot))
					}
					if shouldLogBuildPlace() {
						if cfg.Building.Translated {
							actor := buildActor(ev.BuildOwner, ev.BuildTeam)
							fmt.Printf("[建筑] 玩家=%s (x%d-y%d) 建造了 block=%d(%s) team=%d rot=%d\n", actor, x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam, ev.BuildRot)
						} else {
							fmt.Printf("[buildtrace] placed xy=(%d,%d) block=%d team=%d rot=%d\n", x, y, ev.BuildBlock, ev.BuildTeam, ev.BuildRot)
						}
					}
					broadcastBuildBeginPlace(srv, ev.BuildPos, ev.BuildBlock, ev.BuildRot, byte(ev.BuildTeam), ev.BuildConfig)
				case world.EntityEventBuildConstructed:
					x, y := unpackTilePos(ev.BuildPos)
					if shouldFileLogBuildFinish() {
						detailLog.LogLine(fmt.Sprintf("%s [BUILD] action=constructed x=%d y=%d block_id=%d block=%s team=%d rot=%d",
							time.Now().Format(time.RFC3339Nano), x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam, ev.BuildRot))
					}
					if shouldLogBuildFinish() {
						if cfg.Building.Translated {
							actor := buildActor(ev.BuildOwner, ev.BuildTeam)
							fmt.Printf("[建筑] 玩家=%s (x%d-y%d) 完成建造 block=%d(%s) team=%d rot=%d\n", actor, x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam, ev.BuildRot)
						} else {
							fmt.Printf("[buildtrace] constructed xy=(%d,%d) block=%d team=%d rot=%d\n", x, y, ev.BuildBlock, ev.BuildTeam, ev.BuildRot)
						}
					}
					broadcastBuildConstructedState(srv, wld, ev)
				case world.EntityEventBuildConfig:
					if cfgValue, ok := wld.BuildingConfigPacked(ev.BuildPos); ok {
						srv.BroadcastTileConfig(ev.BuildPos, cfgValue, nil)
					} else if ev.BuildConfig != nil {
						srv.BroadcastTileConfig(ev.BuildPos, ev.BuildConfig, nil)
					}
					broadcastRelatedBlockSnapshots(srv, wld, ev.BuildPos)
				case world.EntityEventBuildDeconstructing:
					x, y := unpackTilePos(ev.BuildPos)
					if shouldFileLogBuildBreakStart() {
						detailLog.LogLine(fmt.Sprintf("%s [BUILD] action=deconstructing x=%d y=%d block_id=%d block=%s team=%d",
							time.Now().Format(time.RFC3339Nano), x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam))
					}
					if shouldLogBuildBreakStart() {
						if cfg.Building.Translated {
							actor := buildActor(ev.BuildOwner, ev.BuildTeam)
							fmt.Printf("[建筑] 玩家=%s (x%d-y%d) 正在拆除 block=%d(%s) team=%d\n", actor, x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam)
						} else {
							fmt.Printf("[buildtrace] deconstructing xy=(%d,%d) block=%d team=%d\n", x, y, ev.BuildBlock, ev.BuildTeam)
						}
					}
					broadcastBuildDeconstructBegin(srv, ev.BuildPos, byte(ev.BuildTeam))
				case world.EntityEventBuildCancelled:
					x, y := unpackTilePos(ev.BuildPos)
					if shouldLogBuildBreakDone() {
						if cfg.Building.Translated {
							actor := buildActor(ev.BuildOwner, ev.BuildTeam)
							fmt.Printf("[建筑] 玩家=%s (x%d-y%d) 取消了建造 block=%d(%s) team=%d\n", actor, x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam)
						} else {
							fmt.Printf("[buildtrace] cancelled xy=(%d,%d) block=%d team=%d\n", x, y, ev.BuildBlock, ev.BuildTeam)
						}
					}
					broadcastBuildDestroyed(srv, ev.BuildPos, ev.BuildBlock)
				case world.EntityEventBuildDestroyed:
					x, y := unpackTilePos(ev.BuildPos)
					if shouldFileLogBuildBreakDone() {
						detailLog.LogLine(fmt.Sprintf("%s [BUILD] action=destroyed x=%d y=%d block_id=%d block=%s team=%d",
							time.Now().Format(time.RFC3339Nano), x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam))
					}
					if shouldLogBuildBreakDone() && groupedExplosionBuilds[i] == nil {
						if cfg.Building.Translated {
							if ev.BuildOwner != 0 {
								actor := buildActor(ev.BuildOwner, ev.BuildTeam)
								fmt.Printf("[建筑] 玩家=%s (x%d-y%d) 拆除了 block=%d(%s) team=%d\n", actor, x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam)
							} else {
								fmt.Printf("[建筑] (x%d-y%d) 被摧毁了 block=%d(%s) team=%d\n", x, y, ev.BuildBlock, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam)
							}
						} else {
							fmt.Printf("[buildtrace] destroyed xy=(%d,%d) block=%d team=%d\n", x, y, ev.BuildBlock, ev.BuildTeam)
						}
					}
					broadcastBuildDestroyedState(srv, ev)
				case world.EntityEventBuildHealth:
					buildHealth = append(buildHealth, ev.BuildPos, int32(math.Float32bits(ev.BuildHP)))
				case world.EntityEventBlockItemSync:
					blockItemSync[ev.BuildPos] = struct{}{}
				case world.EntityEventItemTurretAmmoSync:
					itemTurretAmmoSync[ev.BuildPos] = struct{}{}
				case world.EntityEventTransferItemToUnit:
					amount := ev.ItemAmount
					if amount <= 0 {
						amount = 1
					}
					for n := int32(0); n < amount; n++ {
						broadcastTransferItemToUnit(srv, int16(ev.ItemID), ev.TransferX, ev.TransferY, ev.UnitID)
					}
				case world.EntityEventTransferItemToBuild:
					broadcastTransferItemTo(srv, ev.UnitID, int16(ev.ItemID), ev.ItemAmount, ev.TransferX, ev.TransferY, ev.BuildPos)
				case world.EntityEventBulletFired:
					broadcastBulletCreate(srv, ev.Bullet)
				case world.EntityEventEffect:
					if effectID, ok := lookupEffectID(ev.EffectName); ok {
						broadcastEffectReliable(srv, effectID, ev.EffectX, ev.EffectY, ev.EffectRot)
					}
				}
			}
			if len(buildHealth) > 0 {
				// Send all health deltas in small chunks; do not trim tail,
				// otherwise construct/deconstruct progress appears to "jump".
				const maxInts = 256 // 128 buildings per packet
				for i := 0; i < len(buildHealth); i += maxInts {
					end := i + maxInts
					if end > len(buildHealth) {
						end = len(buildHealth)
					}
					broadcastBuildHealthUpdate(srv, buildHealth[i:end])
				}
			}
			if len(blockItemSync) > 0 {
				positions := make([]int32, 0, len(blockItemSync))
				for packed := range blockItemSync {
					positions = append(positions, packed)
				}
				sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
				broadcastItemBlockSnapshotsForPacked(srv, wld, positions)
			}
			if len(itemTurretAmmoSync) > 0 {
				positions := make([]int32, 0, len(itemTurretAmmoSync))
				for packed := range itemTurretAmmoSync {
					positions = append(positions, packed)
				}
				sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
				broadcastItemTurretAmmoSnapshotsForPacked(srv, wld, positions)
			}
			if !now.Before(nextBlockSnapshotSync) {
				broadcastBlockSnapshots(srv, wld)
				nextBlockSnapshotSync = now.Add(6 * time.Second)
			}
			if shouldLogBuildBreakDone() {
				logGroupedReactorExplosions(wld, evs, groupedExplosionBuilds)
			}
		}
	}()
	saveState := func() {}
	flushColdSnapshot := func() {}
	srv.OnChat = func(c *netserver.Conn, msg string) bool {
		if c != nil && pluginMgr.EventBus().DispatchChat(plugin2.WrapConn(c), msg) {
			return true
		}
		if c != nil && strings.TrimSpace(msg) != "" && shouldFileLogChat() {
			detailLog.LogLine(fmt.Sprintf("%s [CHAT] from=%q player_id=%d uuid=%s ip=%s msg=%q",
				time.Now().Format(time.RFC3339Nano), c.Name(), c.PlayerID(), c.UUID(), c.RemoteAddr().String(), strings.TrimSpace(msg)))
		}
		trimmed := strings.TrimSpace(msg)
		switch trimmed {
		case "/help":
			joinPopupPlugin.ShowHelp(plugin2.WrapConn(c))
			return true
		case "/status":
			srv.SendStatusTo(c)
			return true
		case "/sync":
			if c == nil {
				return true
			}
			syncCurrentRuntimeStateToConn(srv, c, wld, state.get())
			srv.SendChat(c, "[accent]已同步当前运行状态[]")
			return true
		}
		if strings.EqualFold(trimmed, "/stop") {
			if c == nil || !srv.IsOp(c.UUID()) {
				srv.SendChat(c, "[scarlet]没有权限（需要OP）[]")
				return true
			}
			saveState()
			flushColdSnapshot()
			saveOps()
			srv.BroadcastChat("[accent]服务器正在保存并关闭...")
			go func() {
				time.Sleep(200 * time.Millisecond)
				_ = recorder.Close()
				os.Exit(0)
			}()
			return true
		}
		if strings.HasPrefix(trimmed, "/summon ") {
			if c == nil || !srv.IsOp(c.UUID()) {
				srv.SendChat(c, "[scarlet]没有权限（需要OP）[]")
				return true
			}
			args := strings.Fields(strings.TrimSpace(msg))
			if len(args) < 2 {
				srv.SendChat(c, "[scarlet]用法: /summon <typeId|unitName> [x y] [count] [team] []")
				return true
			}
			typeID, typeName, ok := resolveUnitTypeArg(args[1], wld)
			if !ok {
				srv.SendChat(c, "[scarlet]typeId/unitName 无效[]")
				return true
			}
			px, py := c.SnapshotPos()
			x := float64(px)
			y := float64(py)
			team := world.TeamID(1)
			count := 1
			next := 2
			if len(args) >= 4 {
				if xv, err := strconv.ParseFloat(args[2], 32); err == nil {
					if yv, err2 := strconv.ParseFloat(args[3], 32); err2 == nil {
						x = xv
						y = yv
						next = 4
					}
				}
			}
			if len(args) > next {
				if n, err := strconv.ParseInt(args[next], 10, 32); err == nil {
					count = int(n)
					next++
				}
			}
			if len(args) > next {
				if t, err := strconv.ParseInt(args[next], 10, 8); err == nil {
					team = world.TeamID(t)
				}
			}
			if count < 1 {
				count = 1
			}
			if count > 500 {
				count = 500
			}
			success := 0
			var firstID int32
			for i := 0; i < count; i++ {
				sx := float32(x)
				sy := float32(y)
				if i > 0 {
					ring := float32((i-1)/12+1) * 12
					ang := float64(i) * 2 * math.Pi / 12
					sx += float32(math.Cos(ang)) * ring
					sy += float32(math.Sin(ang)) * ring
				}
				ent, err := wld.AddEntity(typeID, sx, sy, team)
				if err != nil {
					continue
				}
				if success == 0 {
					firstID = ent.ID
				}
				success++
			}
			if success == 0 {
				srv.SendChat(c, "[scarlet]召唤失败[]")
				return true
			}
			broadcastSummonVisible(srv, typeID, float32(x), float32(y), byte(team))
			saveState()
			srv.BroadcastChat(fmt.Sprintf("[accent]OP召唤单位[] firstId=%d count=%d type=%d(%s) x=%.1f y=%.1f team=%d", firstID, success, typeID, typeName, x, y, team))
			return true
		}
		if strings.HasPrefix(trimmed, "/despawn ") {
			if c == nil || !srv.IsOp(c.UUID()) {
				srv.SendChat(c, "[scarlet]没有权限（需要OP）[]")
				return true
			}
			args := strings.Fields(strings.TrimSpace(msg))
			if len(args) < 2 {
				srv.SendChat(c, "[scarlet]用法: /despawn <entityId>[]")
				return true
			}
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil || id <= 0 {
				srv.SendChat(c, "[scarlet]entityId 无效[]")
				return true
			}
			if _, ok := wld.RemoveEntity(int32(id)); !ok {
				srv.SendChat(c, "[scarlet]entityId 不存在[]")
				return true
			}
			saveState()
			srv.BroadcastChat(fmt.Sprintf("[accent]OP移除单位[] id=%d", id))
			return true
		}
		if strings.EqualFold(trimmed, "/kill") {
			if c == nil {
				return true
			}
			if !srv.KillSelfUnit(c) {
				srv.SendChat(c, "[scarlet]当前没有可处理的单位[]")
				return true
			}
			srv.SendChat(c, "[accent]已执行 /kill：当前单位已清除[]")
			return true
		}
		if strings.HasPrefix(trimmed, "/umove ") {
			if c == nil || !srv.IsOp(c.UUID()) {
				srv.SendChat(c, "[scarlet]没有权限（需要OP）[]")
				return true
			}
			args := strings.Fields(strings.TrimSpace(msg))
			if len(args) < 4 {
				srv.SendChat(c, "[scarlet]用法: /umove <entityId> <vx> <vy> [rotVel][]")
				return true
			}
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil || id <= 0 {
				srv.SendChat(c, "[scarlet]entityId 无效[]")
				return true
			}
			vx, err := strconv.ParseFloat(args[2], 32)
			if err != nil {
				srv.SendChat(c, "[scarlet]vx 无效[]")
				return true
			}
			vy, err := strconv.ParseFloat(args[3], 32)
			if err != nil {
				srv.SendChat(c, "[scarlet]vy 无效[]")
				return true
			}
			rotVel := float32(0)
			if len(args) >= 5 {
				if rv, rerr := strconv.ParseFloat(args[4], 32); rerr == nil {
					rotVel = float32(rv)
				}
			}
			if _, ok := wld.SetEntityMotion(int32(id), float32(vx), float32(vy), rotVel); !ok {
				srv.SendChat(c, "[scarlet]entityId 不存在[]")
				return true
			}
			saveState()
			srv.SendChat(c, fmt.Sprintf("[accent]单位运动已设置[] id=%d vx=%.2f vy=%.2f rv=%.2f", id, vx, vy, rotVel))
			return true
		}
		if strings.HasPrefix(trimmed, "/uteleport ") {
			if c == nil || !srv.IsOp(c.UUID()) {
				srv.SendChat(c, "[scarlet]没有权限（需要OP）[]")
				return true
			}
			args := strings.Fields(strings.TrimSpace(msg))
			if len(args) < 4 {
				srv.SendChat(c, "[scarlet]用法: /uteleport <entityId> <x> <y> [rotation][]")
				return true
			}
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil || id <= 0 {
				srv.SendChat(c, "[scarlet]entityId 无效[]")
				return true
			}
			x, err := strconv.ParseFloat(args[2], 32)
			if err != nil {
				srv.SendChat(c, "[scarlet]x 无效[]")
				return true
			}
			y, err := strconv.ParseFloat(args[3], 32)
			if err != nil {
				srv.SendChat(c, "[scarlet]y 无效[]")
				return true
			}
			rot := float32(0)
			if len(args) >= 5 {
				if rv, rerr := strconv.ParseFloat(args[4], 32); rerr == nil {
					rot = float32(rv)
				}
			}
			if _, ok := wld.SetEntityPosition(int32(id), float32(x), float32(y), rot); !ok {
				srv.SendChat(c, "[scarlet]entityId 不存在[]")
				return true
			}
			saveState()
			srv.SendChat(c, fmt.Sprintf("[accent]单位传送完成[] id=%d x=%.1f y=%.1f rot=%.1f", id, x, y, rot))
			return true
		}
		if strings.HasPrefix(trimmed, "/ulife ") {
			if c == nil || !srv.IsOp(c.UUID()) {
				srv.SendChat(c, "[scarlet]没有权限（需要OP）[]")
				return true
			}
			args := strings.Fields(strings.TrimSpace(msg))
			if len(args) < 3 {
				srv.SendChat(c, "[scarlet]用法: /ulife <entityId> <seconds(<=0表示无限)>[]")
				return true
			}
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil || id <= 0 {
				srv.SendChat(c, "[scarlet]entityId 无效[]")
				return true
			}
			life, err := strconv.ParseFloat(args[2], 32)
			if err != nil {
				srv.SendChat(c, "[scarlet]seconds 无效[]")
				return true
			}
			if _, ok := wld.SetEntityLife(int32(id), float32(life)); !ok {
				srv.SendChat(c, "[scarlet]entityId 不存在[]")
				return true
			}
			saveState()
			srv.SendChat(c, fmt.Sprintf("[accent]单位寿命已设置[] id=%d life=%.2fs", id, life))
			return true
		}
		if strings.HasPrefix(trimmed, "/ufollow ") {
			if c == nil || !srv.IsOp(c.UUID()) {
				srv.SendChat(c, "[scarlet]没有权限（需要OP）[]")
				return true
			}
			args := strings.Fields(strings.TrimSpace(msg))
			if len(args) < 3 {
				srv.SendChat(c, "[scarlet]用法: /ufollow <id> <targetId> [speed][]")
				return true
			}
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil || id <= 0 {
				srv.SendChat(c, "[scarlet]id 无效[]")
				return true
			}
			targetID, err := strconv.ParseInt(args[2], 10, 32)
			if err != nil || targetID <= 0 {
				srv.SendChat(c, "[scarlet]targetId 无效[]")
				return true
			}
			speed := float32(0)
			if len(args) >= 4 {
				if sp, serr := strconv.ParseFloat(args[3], 32); serr == nil {
					speed = float32(sp)
				}
			}
			if _, ok := wld.SetEntityFollow(int32(id), int32(targetID), speed); !ok {
				srv.SendChat(c, "[scarlet]id 不存在[]")
				return true
			}
			saveState()
			srv.SendChat(c, fmt.Sprintf("[accent]单位跟随已设置[] id=%d -> target=%d speed=%.2f", id, targetID, speed))
			return true
		}
		if strings.HasPrefix(trimmed, "/upatrol ") {
			if c == nil || !srv.IsOp(c.UUID()) {
				srv.SendChat(c, "[scarlet]没有权限（需要OP）[]")
				return true
			}
			args := strings.Fields(strings.TrimSpace(msg))
			if len(args) < 6 {
				srv.SendChat(c, "[scarlet]用法: /upatrol <id> <x1> <y1> <x2> <y2> [speed][]")
				return true
			}
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil || id <= 0 {
				srv.SendChat(c, "[scarlet]id 无效[]")
				return true
			}
			x1, err := strconv.ParseFloat(args[2], 32)
			if err != nil {
				srv.SendChat(c, "[scarlet]x1 无效[]")
				return true
			}
			y1, err := strconv.ParseFloat(args[3], 32)
			if err != nil {
				srv.SendChat(c, "[scarlet]y1 无效[]")
				return true
			}
			x2, err := strconv.ParseFloat(args[4], 32)
			if err != nil {
				srv.SendChat(c, "[scarlet]x2 无效[]")
				return true
			}
			y2, err := strconv.ParseFloat(args[5], 32)
			if err != nil {
				srv.SendChat(c, "[scarlet]y2 无效[]")
				return true
			}
			speed := float32(0)
			if len(args) >= 7 {
				if sp, serr := strconv.ParseFloat(args[6], 32); serr == nil {
					speed = float32(sp)
				}
			}
			if _, ok := wld.SetEntityPatrol(int32(id), float32(x1), float32(y1), float32(x2), float32(y2), speed); !ok {
				srv.SendChat(c, "[scarlet]id 不存在[]")
				return true
			}
			saveState()
			srv.SendChat(c, fmt.Sprintf("[accent]单位巡逻已设置[] id=%d A(%.1f,%.1f) B(%.1f,%.1f) speed=%.2f", id, x1, y1, x2, y2, speed))
			return true
		}
		if strings.HasPrefix(trimmed, "/ubehavior ") {
			if c == nil || !srv.IsOp(c.UUID()) {
				srv.SendChat(c, "[scarlet]没有权限（需要OP）[]")
				return true
			}
			args := strings.Fields(strings.TrimSpace(msg))
			if len(args) < 3 {
				srv.SendChat(c, "[scarlet]用法: /ubehavior clear <id>[]")
				return true
			}
			action := strings.ToLower(args[1])
			if action != "clear" {
				srv.SendChat(c, "[scarlet]仅支持: clear[]")
				return true
			}
			id, err := strconv.ParseInt(args[2], 10, 32)
			if err != nil || id <= 0 {
				srv.SendChat(c, "[scarlet]id 无效[]")
				return true
			}
			if _, ok := wld.ClearEntityBehavior(int32(id)); !ok {
				srv.SendChat(c, "[scarlet]id 不存在[]")
				return true
			}
			saveState()
			srv.SendChat(c, fmt.Sprintf("[accent]单位行为已清除[] id=%d", id))
			return true
		}
		if strings.HasPrefix(trimmed, "/") {
			// 先尝试插件注册的聊天命令
			if pluginMgr != nil {
				cmdName := strings.TrimPrefix(trimmed, "/")
				if spaceIdx := strings.Index(cmdName, " "); spaceIdx > 0 {
					cmdName = cmdName[:spaceIdx]
				}
				args := strings.Fields(trimmed)
				if len(args) > 1 {
					args = args[1:]
				} else {
					args = nil
				}
				if pluginMgr.ChatCommands().Handle(cmdName, wrapChatConn(c), args) {
					return true
				}
			}
			srv.SendChat(c, fmt.Sprintf("[scarlet]无效命令: %s[]", trimmed))
			return true
		}
		return false
	}

	var engine *sim.Engine
	resolveRuntimeBudgets := func() (totalCores, ioWorkers, simWorkers int) {
		totalCores = cfg.Runtime.Cores
		if totalCores <= 0 {
			totalCores = runtime.NumCPU()
		}
		if cfg.Core.DualCoreEnabled {
			ioWorkers = 1
			if totalCores >= 8 {
				ioWorkers = 2
			}
		}
		simWorkers = totalCores - 1 - ioWorkers
		if simWorkers < 1 {
			simWorkers = 1
		}
		if simWorkers > totalCores {
			simWorkers = totalCores
		}
		return totalCores, ioWorkers, simWorkers
	}
	totalRuntimeCores, ioWorkers, schedulerWorkers := resolveRuntimeBudgets()
	if cfg.Runtime.SchedulerEnabled {
		engine = sim.NewEngine(sim.Config{
			TPS:        gameTPS,
			Cores:      totalRuntimeCores,
			Partitions: schedulerWorkers,
		})
		wld.SetScheduler(engine)
		startup.ok("世界调度器", fmt.Sprintf("enabled total_cores=%d io_workers=%d sim_workers=%d", totalRuntimeCores, ioWorkers, schedulerWorkers))
	} else {
		wld.SetScheduler(nil)
		startup.info("世界调度器", "未启用")
	}
	runWorldTick := func(delta time.Duration) {
		step := func() {
			wld.Step(delta)
			unitCommands.Step(wld)
			pluginMgr.EventBus().DispatchTick()
			traceWorldRuntimeTick("game_tick")
		}
		if engine != nil {
			engine.RunTick(delta, step)
			return
		}
		step()
	}
	captureRuntimeSnapshot := func(kind string) persist.State {
		snap := wld.Snapshot()
		return persist.State{
			Version:  persist.SnapshotVersion,
			Kind:     kind,
			MapPath:  state.get(),
			WaveTime: snap.WaveTime,
			Wave:     snap.Wave,
			Enemies:  snap.Enemies,
			Paused:   snap.Paused,
			GameOver: snap.GameOver,
			Tick:     snap.Tick,
			TimeData: snap.TimeData,
			Tps:      snap.Tps,
			Rand0:    snap.Rand0,
			Rand1:    snap.Rand1,
		}
	}
	if cfg.Core.DualCoreEnabled {
		serverCore = coreio.NewServerCore(
			time.Second/time.Duration(gameTPS),
			coreio.Config{
				Name:          "io-core",
				MessageBuf:    30000,
				WorkerCount:   ioWorkers,
				VerboseNetLog: false,
			},
			cfg.Persist,
		)
		if exePath, err := os.Executable(); err == nil {
			if err := serverCore.EnableChildRoles(exePath, []string{"--config=" + cfg.Source}, "core2", "core3", "core4"); err != nil {
				startup.warn("子核心进程", fmt.Sprintf("启动失败，回退到进程内核心: %v", err))
			} else {
				startup.ok("子核心进程", "core2/core3/core4 IPC 已连接")
			}
		} else {
			startup.warn("子核心进程", fmt.Sprintf("无法定位可执行文件，回退到进程内核心: %v", err))
		}
		cache.backend = serverCore.Core3
		serverCore.Core2.SetVerboseNetLog(false)
		serverCore.Core2.SetRecorder(recorder)
		serverCore.SetPersistStateProvider(func() persist.State {
			return captureRuntimeSnapshot("hot")
		})
		serverCore.SetGameTickFn(func(_ uint64, delta time.Duration) {
			runWorldTick(delta)
		})

		netCore := netserver.NewNetworkCoreWithCore(srv, serverCore.Core2)
		netCore.SetServerCore(serverCore)
		netCore.SetRecorder(recorder)
		baseConnOpen := func(c *netserver.Conn) {
			netCore.ConnectionOpen(c)
		}
		baseConnClose := func(c *netserver.Conn) {
			if c != nil {
				pluginMgr.EventBus().DispatchPlayerLeave(plugin2.WrapConn(c))
				if publicConnUUIDStore != nil && runtimePublicConnUUIDEnabled.Load() {
					host := ""
					if c.RemoteAddr() != nil {
						if h, _, err := net.SplitHostPort(c.RemoteAddr().String()); err == nil {
							host = h
						} else {
							host = c.RemoteAddr().String()
						}
					}
					_ = publicConnUUIDStore.ObserveDisconnect(c.UUID(), c.Name(), host)
				}
				buildService.ClearOwner(resolveBuildOwner(c))
				wld.ClearBuilderState(resolveBuildOwner(c))
				if unitID := c.UnitID(); unitID != 0 {
					wld.RemoveEntity(unitID)
				}
				wld.CancelBuildPlansByOwner(resolveBuildOwner(c))
			}
			netCore.ConnectionClose(c)
		}
		srv.OnConnOpen = baseConnOpen
		srv.OnConnClose = baseConnClose
		basePacketDecoded := func(c *netserver.Conn, obj any, err error) bool {
			if err != nil {
				// Let server.handleConn run its normal close/error path to avoid duplicate close events.
				return false
			}
			netCore.ProcessPacket(c, obj, nil)
			return true
		}
		srv.OnPacketDecoded = basePacketDecoded

		if serverCore.Core4 != nil {
			srv.OnConnOpen = func(c *netserver.Conn) {
				baseConnOpen(c)
				if c != nil {
					serverCore.Core4.RecordConnectionOpen(c.ConnID(), connRemoteIP(c), c.UUID())
				}
			}
			srv.OnConnClose = func(c *netserver.Conn) {
				if c != nil {
					serverCore.Core4.RecordConnectionClose(c.ConnID())
				}
				baseConnClose(c)
			}
			srv.OnPacketDecoded = func(c *netserver.Conn, obj any, err error) bool {
				if err != nil {
					return basePacketDecoded(c, obj, err)
				}
				if serverCore.Core4 != nil && c != nil {
					if pkt, ok := obj.(*protocol.ConnectPacket); ok {
						res, perr := serverCore.Core4.AllowConnection(connRemoteIP(c), pkt.UUID)
						if perr != nil {
							fmt.Printf("[core4] allow_connection failed conn=%d ip=%s uuid=%s err=%v\n", c.ConnID(), connRemoteIP(c), pkt.UUID, perr)
							_ = c.Close()
							return true
						}
						if !res.Allowed {
							_ = c.Close()
							return true
						}
						if connUUID := pkt.UUID; strings.TrimSpace(connUUID) != "" {
							_, _ = serverCore.Core4.PlayerShard(connUUID, connRemoteIP(c))
						}
					}
					res, perr := serverCore.Core4.AllowPacket(connRemoteIP(c), c.ConnID(), c.UUID(), fmt.Sprintf("%T", obj))
					if perr != nil {
						fmt.Printf("[core4] allow_packet failed conn=%d ip=%s uuid=%s packet=%T err=%v\n", c.ConnID(), connRemoteIP(c), c.UUID(), obj, perr)
						_ = c.Close()
						return true
					}
					if !res.Allowed {
						_ = c.Close()
						return true
					}
				}
				return basePacketDecoded(c, obj, err)
			}
		}
		serverCore.StartAll()
	} else {
		// Single-core mode: keep server's own packet loop; only add disconnect cleanup and a simple tick loop.
		srv.OnConnClose = func(c *netserver.Conn) {
			if c == nil {
				return
			}
			pluginMgr.EventBus().DispatchPlayerLeave(plugin2.WrapConn(c))
			if publicConnUUIDStore != nil && runtimePublicConnUUIDEnabled.Load() {
				host := ""
				if c.RemoteAddr() != nil {
					if h, _, err := net.SplitHostPort(c.RemoteAddr().String()); err == nil {
						host = h
					} else {
						host = c.RemoteAddr().String()
					}
				}
				_ = publicConnUUIDStore.ObserveDisconnect(c.UUID(), c.Name(), host)
			}
			buildService.ClearOwner(resolveBuildOwner(c))
			wld.ClearBuilderState(resolveBuildOwner(c))
			if unitID := c.UnitID(); unitID != 0 {
				wld.RemoveEntity(unitID)
			}
			wld.CancelBuildPlansByOwner(resolveBuildOwner(c))
		}
		go func() {
			interval := time.Second / time.Duration(gameTPS)
			next := time.Now().Add(interval)
			const maxCatchUp = 4
			for {
				now := time.Now()
				if now.Before(next) {
					time.Sleep(next.Sub(now))
					continue
				}
				steps := 0
				for !now.Before(next) && steps < maxCatchUp {
					runWorldTick(interval)
					steps++
					next = next.Add(interval)
					now = time.Now()
				}
				if steps == maxCatchUp && !now.Before(next) {
					next = now.Add(interval)
				}
			}
		}()
	}
	monitor := newStatusMonitor(srv, cfg, engine)
	saveState = func() {}
	flushColdSnapshot = func() {}
	if cfg.Persist.Enabled {
		saveState = func() {
			hotSnapshots.Update(captureRuntimeSnapshot("hot"))
		}
		flushColdSnapshot = func() {
			stateData := hotSnapshots.Update(captureRuntimeSnapshot("hot"))
			stateData.Kind = "cold"
			if err := persist.Save(cfg.Persist, stateData); err != nil {
				fmt.Printf("[snapshot] cold save failed: %v\n", err)
			}
		}
		saveState()
		hotInterval := time.Duration(cfg.Persist.HotIntervalSec) * time.Second
		if hotInterval <= 0 {
			hotInterval = time.Second
		}
		go func() {
			t := time.NewTicker(hotInterval)
			defer t.Stop()
			for range t.C {
				saveState()
			}
		}()
		coldInterval := time.Duration(cfg.Persist.IntervalSec) * time.Second
		if coldInterval <= 0 {
			coldInterval = 30 * time.Second
		}
		go func() {
			t := time.NewTicker(coldInterval)
			defer t.Stop()
			for range t.C {
				flushColdSnapshot()
			}
		}()
	}
	var videoRecorder *video.Recorder
	if *recordVideo {
		rec, err := video.Start(video.Config{
			Enabled:   true,
			OutputDir: *videoDir,
			FPS:       *videoFPS,
			Width:     *videoWidth,
			Height:    *videoHeight,
			TileSize:  *videoTileSize,
		}, wld.CloneModel, wld.Snapshot, state.get, func() []video.PlayerState {
			snaps := srv.ListPlayerSnapshots()
			out := make([]video.PlayerState, 0, len(snaps))
			for _, snap := range snaps {
				out = append(out, video.PlayerState{
					Name:      snap.Name,
					UUID:      snap.UUID,
					UnitID:    snap.UnitID,
					TeamID:    snap.TeamID,
					X:         snap.X,
					Y:         snap.Y,
					Connected: snap.Connected,
					Dead:      snap.Dead,
				})
			}
			return out
		})
		if err != nil {
			startup.warn("视频录制", err.Error())
		} else {
			videoRecorder = rec
			startup.ok("视频录制", fmt.Sprintf("实时编码 dir=%s fps=%d size=%dx%d", rec.SessionDir(), *videoFPS, *videoWidth, *videoHeight))
		}
	} else {
		startup.info("视频录制", "未启用")
	}
	var (
		apiListener net.Listener
	)
	if cfg.API.Enabled {
		ln, err := net.Listen("tcp", cfg.API.Bind)
		if err != nil {
			startup.fail("API", err.Error())
			if cfg.Personalization.StartupReportEnabled {
				startup.print()
			}
			fmt.Fprintf(os.Stderr, "API 启动前预检失败: %v\n", err)
			os.Exit(1)
		}
		apiListener = ln
		var statsFn func() *sim.TickStats
		if engine != nil {
			statsFn = func() *sim.TickStats {
				st := engine.Stats()
				return &st
			}
		}
		apiPlugin.InitServer(srv, statsFn)
		go func() {
			if err := apiPlugin.Server().ServeListener(apiListener); err != nil {
				log.Error("api serve failed", logging.Field{Key: "error", Value: err.Error()})
			}
		}()
		startup.ok("API", fmt.Sprintf("bind=%s auth=%v", cfg.API.Bind, len(cfg.API.Keys) > 0))
	} else {
		startup.info("API", "未启用")
	}
	var stopOnce sync.Once
	stopServer := func(reason string) {
		stopOnce.Do(func() {
			pluginMgr.EventBus().DispatchShutdown()
			const shutdownForceTimeout = 6 * time.Second
			forceExit := time.AfterFunc(shutdownForceTimeout, func() {
				fmt.Println("关闭超时，强制退出服务器")
				os.Exit(0)
			})
			defer forceExit.Stop()

			if reason != "" {
				fmt.Println(reason)
			}
			if serverCore != nil && serverCore.Core1 != nil {
				serverCore.Core1.Stop()
			}
			const shutdownKickReason = "服务器正在关闭"
			const shutdownKickDelay = 400 * time.Millisecond
			notifyDone := make(chan int, 1)
			go func() {
				notifyDone <- srv.NotifyShutdown(shutdownKickReason, shutdownKickDelay)
			}()
			select {
			case notified := <-notifyDone:
				if notified > 0 {
					time.Sleep(shutdownKickDelay + 100*time.Millisecond)
				}
			case <-time.After(1200 * time.Millisecond):
				fmt.Println("玩家关闭通知超时，继续强制关闭监听")
			}
			srv.Shutdown()
			if videoRecorder != nil {
				if err := videoRecorder.Close(); err != nil {
					fmt.Printf("[video] finalize failed: %v\n", err)
				}
				videoPath := videoRecorder.VideoPath()
				if _, err := os.Stat(videoPath); err == nil {
					fmt.Printf("[video] saved match video: %s (frames=%d dropped=%d)\n", videoPath, videoRecorder.FrameCount(), videoRecorder.DroppedCount())
				} else {
					fmt.Printf("[video] recording session kept at: %s (frames=%d dropped=%d)\n", videoRecorder.SessionDir(), videoRecorder.FrameCount(), videoRecorder.DroppedCount())
				}
			}
			saveState()
			saveOps()
			if engine != nil {
				engine.Stop()
			}
			if serverCore != nil {
				serverCore.StopAll()
			}
			_ = traceLog.Close()
			_ = detailLog.Close()
			_ = recorder.Close()
			os.Exit(0)
		})
	}
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalCh)
	go func() {
		sig, ok := <-signalCh
		if !ok {
			return
		}
		stopServer(fmt.Sprintf("收到信号 %s，正在保存并关闭服务器", sig))
	}()
	if serverCore != nil {
		serverCore.SetChildExitHandler(func(role string, err error) {
			if err != nil {
				fmt.Printf("[core] %s 子进程异常退出: %v\n", role, err)
			} else {
				fmt.Printf("[core] %s 子进程已退出\n", role)
			}
			stopServer(fmt.Sprintf("检测到 %s 子进程退出，正在保存并关闭服务器", role))
		})
	}
	if apiPlugin.Server() != nil {
		apiPlugin.Server().SetSummonFunc(func(typeID int16, x, y float32, team byte) error {
			ent, err := wld.AddEntity(typeID, x, y, world.TeamID(team))
			if err != nil {
				return err
			}
			broadcastSummonVisible(srv, typeID, x, y, team)
			saveState()
			srv.BroadcastChat(fmt.Sprintf("[accent]API召唤单位[] id=%d type=%d x=%.1f y=%.1f team=%d", ent.ID, typeID, x, y, team))
			return nil
		})
		apiPlugin.Server().SetStopFunc(func() {
			stopServer("API 请求关闭服务器")
		})
		apiPlugin.Server().SetUnitMoveFunc(func(id int32, vx, vy, rotVel float32) error {
			if _, ok := wld.SetEntityMotion(id, vx, vy, rotVel); !ok {
				return errors.New("entity not found")
			}
			saveState()
			return nil
		})
		apiPlugin.Server().SetUnitTeleportFunc(func(id int32, x, y, rotation float32) error {
			if _, ok := wld.SetEntityPosition(id, x, y, rotation); !ok {
				return errors.New("entity not found")
			}
			saveState()
			return nil
		})
		apiPlugin.Server().SetUnitLifeFunc(func(id int32, lifeSec float32) error {
			if _, ok := wld.SetEntityLife(id, lifeSec); !ok {
				return errors.New("entity not found")
			}
			saveState()
			return nil
		})
		apiPlugin.Server().SetUnitFollowFunc(func(id int32, targetID int32, speed float32) error {
			if _, ok := wld.SetEntityFollow(id, targetID, speed); !ok {
				return errors.New("entity not found")
			}
			saveState()
			return nil
		})
		apiPlugin.Server().SetUnitPatrolFunc(func(id int32, x1, y1, x2, y2, speed float32) error {
			if _, ok := wld.SetEntityPatrol(id, x1, y1, x2, y2, speed); !ok {
				return errors.New("entity not found")
			}
			saveState()
			return nil
		})
		apiPlugin.Server().SetUnitBehaviorFunc(func(id int32, mode string) error {
			switch strings.ToLower(strings.TrimSpace(mode)) {
			case "", "clear", "none", "stop":
				if _, ok := wld.ClearEntityBehavior(id); !ok {
					return errors.New("entity not found")
				}
				saveState()
				return nil
			default:
				return errors.New("unsupported behavior mode")
			}
		})
		apiPlugin.Server().SetOpsChangedFunc(func() {
			saveOps()
		})
	}
	go func() {
		interval := time.Duration(cfg.Control.ReloadIntervalSec) * time.Second
		if interval <= 0 {
			interval = 5 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			loaded, loadErr := config.Load(cfg.Source)
			if loadErr != nil {
				if cfg.Control.ReloadLogEnabled {
					fmt.Printf("[config] reload failed: %v\n", loadErr)
				}
				continue
			}
			loaded.Source = cfg.Source
			config.ApplyBaseDir(&loaded, rootDir)
			keys, keyErr := mergeValidAPIKeys(loaded.API.Keys, loaded.API.Key)
			if keyErr != nil {
				if cfg.Control.ReloadLogEnabled {
					fmt.Printf("[config] reload failed: %v\n", keyErr)
				}
				continue
			}
			loaded.API.Keys = keys
			loaded.API.Key = ""

			srv.SetServerName(loaded.Runtime.ServerName)
			srv.SetServerDescription(loaded.Runtime.ServerDesc)
			srv.SetVirtualPlayers(int32(loaded.Runtime.VirtualPlayers))
			applyProcessConsoleTitle(loaded, "", loaded.Runtime.ServerName)
			srv.UdpRetryCount = loaded.Net.UdpRetryCount
			srv.UdpRetryDelay = time.Duration(loaded.Net.UdpRetryDelayMs) * time.Millisecond
			srv.UdpFallbackTCP = loaded.Net.UdpFallbackTCP
			srv.SetSnapshotIntervals(loaded.Net.SyncEntityMs, loaded.Net.SyncStateMs)
			runtimePlayerNameColorEnabled.Store(loaded.Personalization.PlayerNameColorEnabled)
			srv.SetPacketRecvEventsEnabled(loaded.Development.PacketRecvEventsEnabled)
			srv.SetPacketSendEventsEnabled(loaded.Development.PacketSendEventsEnabled)
			srv.SetTerminalPlayerLogsEnabled(loaded.Development.TerminalPlayerLogsEnabled)
			srv.SetTerminalPlayerUUIDEnabled(loaded.Development.TerminalPlayerUUIDEnabled)
			srv.SetRespawnPacketLogsEnabled(loaded.Development.RespawnPacketLogsEnabled)
			srv.SetPlayerNameColorEnabled(loaded.Personalization.PlayerNameColorEnabled)
			srv.SetTranslatedConnLog(loaded.Control.TranslatedConnLogEnabled)
			srv.SetJoinLeaveChatEnabled(loaded.Personalization.JoinLeaveChatEnabled)
			runtimeJoinLeaveChatEnabled.Store(loaded.Personalization.JoinLeaveChatEnabled)
			runtimeTraceCfg.Store(loaded.Tracepoints)
			runtimePlayerNamePrefix.Store(loaded.Personalization.PlayerNamePrefix)
			runtimePlayerNameSuffix.Store(loaded.Personalization.PlayerNameSuffix)
			runtimePlayerBindPrefixEnabled.Store(loaded.Personalization.PlayerBindPrefixEnabled)
			runtimePlayerBoundPrefix.Store(loaded.Personalization.PlayerBoundPrefix)
			runtimePlayerUnboundPrefix.Store(loaded.Personalization.PlayerUnboundPrefix)
			runtimePlayerTitleEnabled.Store(loaded.Personalization.PlayerTitleEnabled)
			runtimePlayerConnIDSuffixEnabled.Store(loaded.Personalization.PlayerConnIDSuffixEnabled)
			runtimePlayerConnIDSuffixFormat.Store(loaded.Personalization.PlayerConnIDSuffixFormat)
			mapVotePlugin.ReloadConfig(&loaded)
			runtimeBindStatusResolver = newBindStatusResolver(
				loaded.Personalization.PlayerBindSource,
				loaded.Personalization.PlayerBindAPIURL,
				time.Duration(loaded.Personalization.PlayerBindAPITimeoutMs)*time.Millisecond,
				time.Duration(loaded.Personalization.PlayerBindAPICacheSec)*time.Second,
				runtimePlayerIdentityStore,
			)
			srv.RefreshPlayerDisplayNames()
			runtimePublicConnUUIDEnabled.Store(loaded.Control.PublicConnUUIDEnabled)
			applyBlockNameTranslations(configDir)
			if serverCore != nil && serverCore.Core2 != nil {
				serverCore.Core2.SetVerboseNetLog(false)
			}
			if loaded.Control.PublicConnUUIDFile != cfg.Control.PublicConnUUIDFile && cfg.Control.ReloadLogEnabled {
				fmt.Printf("[config] public_conn_uuid_file changed to %s, restart required to reopen mapping file\n", loaded.Control.PublicConnUUIDFile)
			}
			if loaded.Control.ConnUUIDAutoCreateEnabled != cfg.Control.ConnUUIDAutoCreateEnabled && cfg.Control.ReloadLogEnabled {
				fmt.Printf("[config] conn_uuid_auto_create changed to %t, restart required to reopen mapping policy\n", loaded.Control.ConnUUIDAutoCreateEnabled)
			}
			if loaded.Control.PlayerIdentityAutoCreateEnabled != cfg.Control.PlayerIdentityAutoCreateEnabled && cfg.Control.ReloadLogEnabled {
				fmt.Printf("[config] player_identity_auto_create changed to %t, restart required to reopen identity file policy\n", loaded.Control.PlayerIdentityAutoCreateEnabled)
			}
			if loaded.Personalization.PlayerIdentityFile != cfg.Personalization.PlayerIdentityFile && cfg.Control.ReloadLogEnabled {
				fmt.Printf("[config] player_identity_file changed to %s, restart required to reopen identity file\n", loaded.Personalization.PlayerIdentityFile)
			}
			if loaded.Tracepoints.File != cfg.Tracepoints.File && cfg.Control.ReloadLogEnabled {
				fmt.Printf("[config] tracepoints file changed to %s, restart required to reopen trace log file\n", loaded.Tracepoints.File)
			}

			if apiPlugin.Server() != nil {
				apiPlugin.ApplyAPIKeySet( loaded.API.Keys)
			}
			if loaded.API.Enabled != cfg.API.Enabled || loaded.API.Bind != cfg.API.Bind {
				if cfg.Control.ReloadLogEnabled {
					fmt.Printf("[config] api enabled/bind changed (enabled=%v bind=%s), restart required to apply\n", loaded.API.Enabled, loaded.API.Bind)
				}
			}
			if err := applyAdmissionPolicy(loaded); err != nil {
				if cfg.Control.ReloadLogEnabled {
					fmt.Printf("[config] reload failed: %v\n", err)
				}
				continue
			}

			cfg = loaded
			next := time.Duration(cfg.Control.ReloadIntervalSec) * time.Second
			if next <= 0 {
				next = 5 * time.Second
			}
			ticker.Reset(next)
			if cfg.Control.ReloadLogEnabled {
				fmt.Printf("[config] reloaded: server=%q entity_ms=%d state_ms=%d keys=%d interval=%ds\n",
					cfg.Runtime.ServerName, cfg.Net.SyncEntityMs, cfg.Net.SyncStateMs, len(cfg.API.Keys), cfg.Control.ReloadIntervalSec)
			}
		}
	}()
	removeEntityByID := func(id int32) bool {
		_, ok := wld.RemoveEntity(id)
		return ok
	}
	setEntityMotion := func(id int32, vx, vy, rotVel float32) bool {
		_, ok := wld.SetEntityMotion(id, vx, vy, rotVel)
		return ok
	}
	setEntityPos := func(id int32, x, y, rot float32) bool {
		_, ok := wld.SetEntityPosition(id, x, y, rot)
		return ok
	}
	setEntityLife := func(id int32, life float32) bool {
		_, ok := wld.SetEntityLife(id, life)
		return ok
	}
	setEntityFollow := func(id, targetID int32, speed float32) bool {
		_, ok := wld.SetEntityFollow(id, targetID, speed)
		return ok
	}
	setEntityPatrol := func(id int32, x1, y1, x2, y2, speed float32) bool {
		_, ok := wld.SetEntityPatrol(id, x1, y1, x2, y2, speed)
		return ok
	}
	clearEntityBehavior := func(id int32) bool {
		_, ok := wld.ClearEntityBehavior(id)
		return ok
	}
	reloadVanillaProfiles := func(path string) error {
		return wld.LoadVanillaProfiles(path)
	}
	reloadVanillaContentIDs := func(path string) error {
		ids, err := vanilla.LoadContentIDs(path)
		if err != nil {
			return err
		}
		setEffectIDs(ids)
		_ = vanilla.ApplyContentIDs(srv.Content, ids)
		return nil
	}
	startup.ok("服务端启动", "初始化完成")
	if cfg.Personalization.StartupReportEnabled {
		startup.print()
	}
	if cfg.Personalization.MapLoadDetailsEnabled && loadedModel != nil {
		printMapDetails(loadedMapPath, loadedModel)
	}
	if cfg.Personalization.UnitIDListEnabled {
		printUnitIDList(unitNamesByID)
	}
	closeImmediate := func() {
		_ = traceLog.Close()
		_ = detailLog.Close()
		_ = recorder.Close()
	}
	go runConsole(srv, state, pluginMgr, *addr, *buildVersion, &cfg, saveConfig, recorder, monitor, saveOps, loadWorldModel, invalidateWorldCache, reloadVanillaProfiles, reloadVanillaContentIDs, removeEntityByID, setEntityMotion, setEntityPos, setEntityLife, setEntityFollow, setEntityPatrol, clearEntityBehavior, stopServer, closeImmediate, wld)
	if serverCore != nil {
		go func() {
			if err := srv.Serve(); err != nil {
				fmt.Fprintf(os.Stderr, "服务器启动失败: %v\n", err)
				os.Exit(1)
			}
		}()
		serverCore.Core1.Run(time.Second / time.Duration(gameTPS))
	} else {
		if err := srv.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "服务器启动失败: %v\n", err)
			os.Exit(1)
		}
	}
}

func colorState(enabled bool) string {
	if enabled {
		return "\x1b[32m✓ 开启\x1b[0m"
	}
	return "\x1b[31m✗ 关闭\x1b[0m"
}


