package engineadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CoscaAI/cosca/internal/worldspec"
)

// ──────────────────────────────────────────────────────────────
// roblox-adapter — materializa um WorldSpec num projeto Rojo + Luau.
// Padrões do BIBLE ([FACT]): AuthorityMode=Server, DataStore UpdateAsync=CAS
// + session-lock, validação de RemoteEvent (never-trust-client).
// ──────────────────────────────────────────────────────────────

type RobloxAdapter struct {
	OutDir string
}

// NewRobloxAdapter cria um adapter Roblox que grava em OutDir.
func NewRobloxAdapter(outDir string) *RobloxAdapter {
	return &RobloxAdapter{OutDir: outDir}
}

// Target implementa EngineAdapter.
func (a *RobloxAdapter) Target() string { return "roblox" }

// Materialize implementa EngineAdapter: gera o projeto Rojo (filesystem) do WorldSpec.
func (a *RobloxAdapter) Materialize(_ context.Context, spec worldspec.WorldSpec) (ExecutionHandle, error) {
	out := a.OutDir
	if out == "" {
		out = "rojo-out"
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return ExecutionHandle{}, err
	}

	// 1) Rojo project (filesystem -> Instances).
	proj := map[string]any{
		"name": spec.Name,
		"tree": map[string]any{
			"$className": "DataModel",
			"ReplicatedStorage": map[string]any{
				"$className": "ReplicatedStorage",
				"Packages":   map[string]any{"$path": "src/ReplicatedStorage/Packages"},
			},
			"ServerScriptService": map[string]any{
				"$className": "ServerScriptService",
				"Server":     map[string]any{"$path": "src/ServerScriptService"},
			},
		},
	}
	projJSON, _ := json.MarshalIndent(proj, "", "  ")
	if err := os.WriteFile(filepath.Join(out, "default.project.json"), projJSON, 0o644); err != nil {
		return ExecutionHandle{}, err
	}

	// 2) Luau runtime + configs do toolchain.
	files := map[string]string{
		"src/ReplicatedStorage/Packages/WorldSpec.luau": worldSpecLuau(spec),
		"src/ServerScriptService/Authority.server.luau": authorityLuau(spec),
		"src/ServerScriptService/DataStore.luau":        dataStoreLuau(),
		"src/ServerScriptService/Remotes.luau":          remotesLuau(),
		"selene.toml":                                   seleneToml(),
		"stylua.toml":                                   styluaToml(),
		".luaurc":                                       luaurc(),
	}
	names := make([]string, 0, len(files))
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)
	written := []string{"default.project.json"}
	for _, rel := range names {
		full := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return ExecutionHandle{}, err
		}
		if err := os.WriteFile(full, []byte(files[rel]), 0o644); err != nil {
			return ExecutionHandle{}, err
		}
		written = append(written, rel)
	}

	return ExecutionHandle{
		Target:    a.Target(),
		Summary:   fmt.Sprintf("Materialized WorldSpec '%s' (type=%s, %d entities, %d terrain) -> Rojo project", spec.Name, spec.WorldType, len(spec.Entities), len(spec.Terrain)),
		Files:     written,
		WorldHash: spec.Hash(),
	}, nil
}

// ──────────────────────────────────────────────────────────────
// Templates Luau (princípios do BIBLE — refletem o WorldSpec real).
// ──────────────────────────────────────────────────────────────

func worldSpecLuau(spec worldspec.WorldSpec) string {
	return "-- WorldSpec (ADR-024) espelhado no runtime. Server = autoridade.\n" +
		"local WorldSpec = {}\n" +
		"WorldSpec.WorldType = " + str(spec.WorldType) + "\n" +
		"WorldSpec.SchemaVersion = " + str(spec.SchemaVersion) + "\n" +
		"WorldSpec.Seed = " + fmt.Sprint(spec.Seed) + "\n" +
		"WorldSpec.EntityCount = " + fmt.Sprint(len(spec.Entities)) + "\n\n" +
		"-- O client NAO valida estado critico; so consome visao.\n" +
		"return WorldSpec\n"
}

func authorityLuau(spec worldspec.WorldSpec) string {
	return "-- AuthorityMode = Server (ADR-023): o server e a autoridade.\n" +
		"local Players = game:GetService(\"Players\")\n" +
		"local RunService = game:GetService(\"RunService\")\n\n" +
		"local Authority = {}\n" +
		"Authority.IsServerAuthed = RunService:IsServer()\n\n" +
		"-- Estado critico so existe no servidor.\n" +
		"Authority.State = { seed = " + fmt.Sprint(spec.Seed) + ", worldType = " + str(spec.WorldType) + " }\n\n" +
		"function Authority.OnPlayerJoined(_player)\n" +
		"-- server e dono de todo estado critico.\n" +
		"end\n\n" +
		"Players.PlayerAdded:Connect(Authority.OnPlayerJoined)\n\n" +
		"return Authority\n"
}

func dataStoreLuau() string {
	return "-- Persistencia (BIBLE [FACT]): UpdateAsync = CAS atomico; SetAsync = wipe.\n" +
		"local DataStoreService = game:GetService(\"DataStoreService\")\n\n" +
		"local DataStore = {}\nDataStore.Name = \"Cosca_WorldSaved\"\n\n" +
		"function DataStore.UpdateAsync(key, transform)\n" +
		"\tlocal store = DataStoreService:GetDataStore(DataStore.Name)\n" +
		"\tlocal ok, value = pcall(function()\n" +
		"\t\treturn store:UpdateAsync(key, function(current)\n" +
		"\t\t\tlocal cur = (type(current) == \"table\") and current or { version = 0 }\n" +
		"\t\t\treturn transform(cur)\n" +
		"\t\tend)\n" +
		"\tend)\n" +
		"\tif not ok then return nil, value end -- retry/backoff; nunca re-escrever cegamente\n" +
		"\treturn value, nil\n" +
		"end\n\n" +
		"return DataStore\n"
}

func remotesLuau() string {
	return "-- Validacao de RemoteEvent (BIBLE [FACT]): client nunca e autoridade.\n" +
		"local Remotes = {}\n" +
		"Remotes.MAX_LATENCY = 0.8\n" +
		"Remotes.MAX_SPAWN_DIST = 200\n\n" +
		"local calls = {}\n" +
		"local function throttled(player)\n" +
		"\tlocal now = os.clock()\n" +
		"\tif (calls[player] or 0) > now then return true end\n" +
		"\tcalls[player] = now + 0.1\n" +
		"\treturn false\n" +
		"end\n\n" +
		"function Remotes.OnFireRequest(player, params)\n" +
		"\tif throttled(player) then return false, \"rate_limited\" end\n" +
		"\tlocal latency = math.abs(params.latency or 0)\n" +
		"\tif latency < 0 or latency > Remotes.MAX_LATENCY then return false, \"invalid_latency\" end\n" +
		"\tif math.abs((params.origin and params.origin.X) or 0) > Remotes.MAX_SPAWN_DIST then return false, \"invalid_origin\" end\n" +
		"\treturn true, \"ok\"\n" +
		"end\n\n" +
		"return Remotes\n"
}

func seleneToml() string {
	return `std = "roblox"

[lints]
correctness = "warn"
unused = "warn"
`
}

func styluaToml() string {
	return `column_width = 120
line_endings = "Unix"
indent_type = "Tabs"
`
}

func luaurc() string {
	return "{\n\t\"languageMode\": \"strict\"\n}\n"
}

// str emite uma string Luau (com aspas) segura.
func str(s string) string {
	return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
}
