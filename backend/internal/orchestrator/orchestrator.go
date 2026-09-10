package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aishow/internal/config"
	"aishow/internal/models"
)

const (
	readyTimeout  = 12 * time.Minute
	stopTimeout   = 20 * time.Second
	vramCooldown  = 5 * time.Second
	probeInterval = 2 * time.Second
)

type spec struct {
	ID       string
	Label    string
	Script   string
	UsesComfy bool
}

type Manager struct {
	cfg    config.Config
	root   string
	http   *http.Client
	mu     sync.Mutex
	procs  map[string]*os.Process
	keep   string
}

func New(cfg config.Config) *Manager {
	root := findRepoRoot(cfg.RepoRoot)
	if root != "" {
		log.Printf("orchestrator repo root: %s", root)
	} else {
		log.Printf("orchestrator: 未找到 scripts/*.bat，自动切换将无法拉起边车")
	}
	return &Manager{
		cfg:   cfg,
		root:  root,
		http:  &http.Client{Timeout: 4 * time.Second},
		procs: map[string]*os.Process{},
	}
}

func (m *Manager) Start(snapFn func() models.SettingsPayload) {
	go func() {
		time.Sleep(2 * time.Second)
		for {
			snap := snapFn()
			if snap.AutoSwitchEngine == nil || *snap.AutoSwitchEngine {
				m.Reconcile(snap)
			}
			time.Sleep(12 * time.Second)
		}
	}()
}

func (m *Manager) Remember(engine string) {
	engine = models.CanonicalEngine(engine)
	if engine == "" {
		return
	}
	m.mu.Lock()
	m.keep = engine
	m.mu.Unlock()
}

func (m *Manager) ActiveEngine(snap models.SettingsPayload) string {
	m.mu.Lock()
	keep := m.keep
	m.mu.Unlock()
	if keep != "" && m.ready(m.endpoint(keep, snap)) {
		return keep
	}
	var found []string
	for _, id := range exclusiveEngines() {
		if m.ready(m.endpoint(id, snap)) {
			found = append(found, id)
		}
	}
	if len(found) == 1 {
		return found[0]
	}
	return keep
}

func (m *Manager) Reconcile(snap models.SettingsPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keep := m.keep
	if keep == "" {
		var readies []string
		for _, id := range exclusiveEngines() {
			if m.ready(m.endpoint(id, snap)) {
				readies = append(readies, id)
			}
		}
		limit := maxLoaded(snap)
		if len(readies) == 0 || len(readies) <= limit {
			return
		}
		keep = readies[len(readies)-1]
		m.keep = keep
		log.Printf("orchestrator: 超过同时加载上限 %d，保留 %s", limit, models.EngineLabel(keep))
	}
	m.evictExtras(keep, snap, nil)
}

func (m *Manager) EnsureReady(engine string, snap models.SettingsPayload, abort func() bool, report func(string)) error {
	engine = models.CanonicalEngine(engine)
	if abort == nil {
		abort = func() bool { return false }
	}
	if report == nil {
		report = func(string) {}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.keep = engine

	targetURL := m.endpoint(engine, snap)
	if targetURL == "" {
		return fmt.Errorf("未配置 %s 节点地址", models.EngineLabel(engine))
	}

	m.evictExtras(engine, snap, report)

	if m.ready(targetURL) {
		report(models.EngineLabel(engine) + " 已在线，其它模型已按上限下线")
		return nil
	}

	if abort() {
		return fmt.Errorf("任务已取消")
	}

	label := models.EngineLabel(engine)
	report("启动 " + label)
	if err := m.startEngine(engine); err != nil {
		return err
	}
	started := time.Now()
	lastTalk := time.Time{}
	deadline := time.Now().Add(readyTimeout)
	for time.Now().Before(deadline) {
		if abort() {
			return fmt.Errorf("任务已取消")
		}
		if m.ready(targetURL) {
			report(fmt.Sprintf("%s 已就绪（加载 %s）", label, time.Since(started).Truncate(time.Second)))
			return nil
		}
		if exited := m.processExited(engine); exited {
			return fmt.Errorf("启动 %s 失败，边车进程已退出。请看对应启动窗口", label)
		}
		if lastTalk.IsZero() || time.Since(lastTalk) >= 8*time.Second {
			report(waitMessage(label, time.Since(started), m.loadHint(targetURL)))
			lastTalk = time.Now()
		}
		time.Sleep(probeInterval)
	}
	return fmt.Errorf("等待 %s 就绪超时（%s）", label, targetURL)
}

func maxLoaded(snap models.SettingsPayload) int {
	if snap.MaxLoadedEngines != nil && *snap.MaxLoadedEngines > 1 {
		return *snap.MaxLoadedEngines
	}
	return 1
}

func (m *Manager) evictExtras(keep string, snap models.SettingsPayload, report func(string)) {
	if report == nil {
		report = func(string) {}
	}
	keep = models.CanonicalEngine(keep)
	limit := maxLoaded(snap)
	var extras []string
	stoppedComfyPeer := false
	for _, id := range exclusiveEngines() {
		if id == keep {
			continue
		}
		if !m.alive(m.endpoint(id, snap)) {
			continue
		}
		extras = append(extras, id)
	}
	allow := limit - 1
	if allow < 0 {
		allow = 0
	}
	for i, other := range extras {
		if i < allow {
			continue
		}
		if usesComfy(other) {
			stoppedComfyPeer = true
		}
		report(fmt.Sprintf("下线 %s（同时最多加载 %d 个）", models.EngineLabel(other), limit))
		if err := m.stopEngine(other, m.endpoint(other, snap), snap); err != nil {
			log.Printf("orchestrator stop %s: %v", other, err)
		}
	}
	if m.comfyAlive() && (!usesComfy(keep) || stoppedComfyPeer) {
		report("结束 ComfyUI，避免旧图占显存")
		m.stopURL(m.comfyURL())
	}
}

func (m *Manager) Probe(base string) (ready bool, lat int64, detail string) {
	start := time.Now()
	body, ok := m.getJSON(strings.TrimRight(base, "/") + "/health")
	lat = time.Since(start).Milliseconds()
	if !ok {
		return false, lat, "离线"
	}
	if err, _ := body["error"].(string); err != "" {
		return false, lat, err
	}
	if flag, exists := body["ready"]; exists {
		if b, ok := flag.(bool); ok {
			if b {
				return true, lat, "已加载"
			}
			if loading, _ := body["loading"].(bool); loading {
				return false, lat, "加载中"
			}
			return false, lat, "进程在，模型未就绪"
		}
	}
	return true, lat, "HTTP 可达"
}

func (m *Manager) startEngine(engine string) error {
	sp, ok := engineSpec(engine)
	if !ok {
		return fmt.Errorf("未知引擎 %s", engine)
	}
	if m.root == "" {
		return fmt.Errorf("找不到仓库根目录，无法执行 %s", sp.Script)
	}
	script := filepath.Join(m.root, "scripts", sp.Script)
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("找不到启动脚本 %s", script)
	}
	proc, err := startScript(script, m.root)
	if err != nil {
		return fmt.Errorf("启动 %s 失败: %w", sp.Label, err)
	}
	m.procs[engine] = proc
	log.Printf("orchestrator started %s pid=%d script=%s", engine, proc.Pid, script)
	return nil
}

func (m *Manager) stopEngine(engine, endpoint string, snap models.SettingsPayload) error {
	m.postShutdown(endpoint)
	if !waitUntil(stopTimeout, func() bool { return !m.alive(endpoint) }) {
		killPort(portOf(endpoint))
	}
	if p := m.procs[engine]; p != nil {
		_ = killTree(p.Pid)
		delete(m.procs, engine)
	}
	waitUntil(8*time.Second, func() bool { return !m.alive(endpoint) })
	time.Sleep(vramCooldown)
	return nil
}

func (m *Manager) stopURL(base string) {
	if base == "" {
		return
	}
	m.postShutdown(base)
	if !waitUntil(8*time.Second, func() bool { return !m.alive(base) }) {
		killPort(portOf(base))
	}
	waitUntil(8*time.Second, func() bool { return !m.alive(base) })
}

func (m *Manager) processExited(engine string) bool {
	p := m.procs[engine]
	if p == nil {
		return false
	}
	return processGone(p.Pid)
}

func (m *Manager) endpoint(engine string, snap models.SettingsPayload) string {
	switch models.CanonicalEngine(engine) {
	case models.EngineH3Ref2VAInt8:
		return strings.TrimRight(snap.SGLANGRef2VAURL, "/")
	case models.EngineFastH3:
		return strings.TrimRight(snap.FastH3URL, "/")
	case models.EngineLLadaImage:
		return strings.TrimRight(snap.LLaDAImageURL, "/")
	default:
		return strings.TrimRight(snap.SGLANGFL2VAURL, "/")
	}
}

func (m *Manager) comfyURL() string {
	u := strings.TrimRight(m.cfg.ComfyURL, "/")
	if u == "" {
		return "http://127.0.0.1:8188"
	}
	return u
}

func (m *Manager) comfyAlive() bool {
	return m.alive(m.comfyURL())
}

func waitMessage(label string, elapsed time.Duration, hint string) string {
	msg := fmt.Sprintf("等待 %s 加载 %s", label, elapsed.Truncate(time.Second))
	if hint != "" {
		return msg + " · " + hint
	}
	return msg + " · 权重从磁盘搬到显存，不是卡住"
}

func (m *Manager) loadHint(base string) string {
	body, ok := m.getJSON(base + "/health")
	if !ok {
		return "进程已拉起，正在导入依赖 / 读盘"
	}
	if err, _ := body["error"].(string); err != "" {
		return err
	}
	if model, _ := body["model"].(string); strings.TrimSpace(model) != "" {
		return "正在把本地权重装进 GPU"
	}
	return ""
}

func (m *Manager) ready(base string) bool {
	if base == "" {
		return false
	}
	body, ok := m.getJSON(base + "/health")
	if !ok {
		return false
	}
	if ready, exists := body["ready"]; exists {
		if b, ok := ready.(bool); ok {
			return b
		}
	}
	if okFlag, exists := body["ok"]; exists {
		if b, ok := okFlag.(bool); ok {
			return b
		}
	}
	return true
}

func (m *Manager) alive(base string) bool {
	if base == "" {
		return false
	}
	_, ok := m.getJSON(base + "/health")
	if ok {
		return true
	}
	_, ok = m.getJSON(base + "/")
	return ok
}

func (m *Manager) getJSON(raw string) (map[string]any, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, false
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, false
	}
	var payload map[string]any
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if json.Unmarshal(b, &payload) != nil {
		return map[string]any{}, resp.StatusCode < 500
	}
	return payload, true
}

func (m *Manager) postShutdown(base string) {
	if base == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/shutdown", nil)
	if err != nil {
		return
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func exclusiveEngines() []string {
	return []string{
		models.EngineH3,
		models.EngineH3Ref2VAInt8,
		models.EngineFastH3,
		models.EngineLLadaImage,
	}
}

func engineSpec(engine string) (spec, bool) {
	switch models.CanonicalEngine(engine) {
	case models.EngineH3Ref2VAInt8:
		return spec{ID: models.EngineH3Ref2VAInt8, Label: "H3 Ref2VA INT8", Script: "start_h3_ref2va_int8.bat", UsesComfy: true}, true
	case models.EngineFastH3:
		return spec{ID: models.EngineFastH3, Label: "FastH3", Script: "start_fasth3_gguf.bat", UsesComfy: true}, true
	case models.EngineLLadaImage:
		return spec{ID: models.EngineLLadaImage, Label: "LLaDA-Image", Script: "start_llada_image.bat"}, true
	case models.EngineH3:
		return spec{ID: models.EngineH3, Label: "H3-Base", Script: "start_h3_nf4.bat"}, true
	default:
		return spec{}, false
	}
}

func usesComfy(engine string) bool {
	sp, ok := engineSpec(engine)
	return ok && sp.UsesComfy
}

func findRepoRoot(explicit string) string {
	var cands []string
	if strings.TrimSpace(explicit) != "" {
		cands = append(cands, explicit)
	}
	if wd, err := os.Getwd(); err == nil {
		cands = append(cands, wd)
	}
	if exe, err := os.Executable(); err == nil {
		cands = append(cands, filepath.Dir(exe))
	}
	for _, start := range cands {
		dir := start
		for i := 0; i < 8; i++ {
			if scriptExists(dir) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

func scriptExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "scripts", "start_h3_nf4.bat"))
	return err == nil
}

func portOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

func waitUntil(d time.Duration, ok func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ok() {
			return true
		}
		time.Sleep(400 * time.Millisecond)
	}
	return ok()
}
