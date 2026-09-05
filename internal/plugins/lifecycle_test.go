package plugins

import (
	"testing"
)

func pluginInfo(id string, deps ...string) PluginInfo {
	var depList []PluginDependency
	for _, d := range deps {
		depList = append(depList, PluginDependency{PluginID: d})
	}
	return PluginInfo{
		Manifest: PluginManifest{ID: id, Version: "1.0.0", Dependencies: depList},
	}
}

func TestOrderByDependencies(t *testing.T) {
	lm := NewLifecycleManager(nil, nil, nil)

	// Sem dependências: preserva a ordem.
	plugins := []PluginInfo{pluginInfo("a"), pluginInfo("b")}
	got, err := lm.orderByDependencies(plugins)
	if err != nil || len(got) != 2 {
		t.Fatalf("simple: %v, %v", got, err)
	}

	// Dependentes depois das dependências.
	plugins = []PluginInfo{
		pluginInfo("search", "core"),
		pluginInfo("core"),
		pluginInfo("ui", "search"),
	}
	got, err = lm.orderByDependencies(plugins)
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	pos := map[string]int{}
	for i, p := range got {
		pos[p.Manifest.ID] = i
	}
	if !(pos["core"] < pos["search"] && pos["search"] < pos["ui"]) {
		t.Fatalf("ordem violada: %v", got)
	}
}

func TestOrderByDependenciesCycle(t *testing.T) {
	lm := NewLifecycleManager(nil, nil, nil)
	plugins := []PluginInfo{
		pluginInfo("a", "b"),
		pluginInfo("b", "a"),
	}
	if _, err := lm.orderByDependencies(plugins); err == nil {
		t.Fatal("cycle must produce an error")
	}
}

func TestLifecycleManagerEmptyIsNoop(t *testing.T) {
	// Manager vazio (nenhum plugin) → Init/Start/Stop são no-ops sem erro.
	mgr := NewManager(ManagerConfig{Dir: t.TempDir()}, nil)
	lm := NewLifecycleManager(mgr, nil, nil)
	if err := lm.InitPlugins(); err != nil {
		t.Fatalf("InitPlugins: %v", err)
	}
	if err := lm.StartPlugins(); err != nil {
		t.Fatalf("StartPlugins: %v", err)
	}
	if err := lm.StopPlugins(); err != nil {
		t.Fatalf("StopPlugins: %v", err)
	}
	if s := lm.Status(); s.TotalPlugins != 0 {
		t.Fatalf("status: %+v", s)
	}
	// Health de plugin inexistente → não pode panicar.
	_, _ = lm.GetPluginHealth("ghost")
}

func TestPluginLogger(t *testing.T) {
	// O pluginLogger não deve panicar em nenhum nível.
	l := &pluginLogger{pluginID: "test-plugin"}
	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")
}

func TestPluginRuntimeAPINoManager(t *testing.T) {
	// API de runtime com registries REAIS (como em produção): chamadas
	// funcionam sem panic.
	api := &pluginRuntimeAPI{pluginID: "test", hookRegistry: DefaultHookRegistry(), eventBus: DefaultEventBus()}
	_, _ = api.GetConfig("k")
	_ = api.SetConfig("k", "v")
	_ = api.EmitEvent("type", "data")
	if _, err := api.RegisterHook("point", nil); err != nil {
		t.Logf("RegisterHook: %v", err)
	}
	_ = api.UnregisterHook("id")
}
