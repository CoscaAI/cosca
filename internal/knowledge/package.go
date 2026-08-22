// Knowledge Package — manifesto genérico de biblioteca.
//
// Um Knowledge Package descreve uma biblioteca/framework/tool/runtime que o
// Cosca CONHECE — não que ele instalou. A distinção é fundamental:
//
//	KNOWLEDGE ≠ DEPENDENCY ≠ EXECUTABLE CODE.
//
// O Cosca pode conhecer 100 bibliotecas sem instalar nenhuma. O manifesto é
// genérico ({id, kind, ecosystem, versions, sources, ...}) — sem "modo
// Next.js", "modo Prisma": a biblioteca é apenas outro Knowledge Package.
// O pipeline real de aquisição (fontes, versões, coleta de doc/código/
// releases, hash + procedência, quarantine) vive em internal/acquisition —
// aqui fica a descoberta heurística LOCAL (DetectPackageInfo), sem rede.
//
// Persistência: .cosca/knowledge/packages/<id>.json (runtime, gitignored).
package knowledge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ── Constantes do manifesto ─────────────────────────────────────────────────

// Kinds de Knowledge Package.
const (
	PackageKindLibrary   = "library"
	PackageKindFramework = "framework"
	PackageKindTool      = "tool"
	PackageKindRuntime   = "runtime"
)

// Níveis de conhecimento do pacote (epistemic-ish — o sistema sabe quando
// NÃO sabe): nenhum conhecimento ainda, parcial, ou validado.
const (
	PackageKnowledgeNone      = "none"
	PackageKnowledgePartial   = "partial"
	PackageKnowledgeValidated = "validated"
)

// Ciclo de vida do status do manifesto.
const (
	PackageStatusManifest    = "manifest"    // registrado, nada adquirido ainda
	PackageStatusAcquiring   = "acquiring"   // coleta de fontes em andamento
	PackageStatusQuarantined = "quarantined" // artefatos A-XXXX em quarentena
	PackageStatusValidated   = "validated"   // conhecimento extraído e validado
)

// EcosystemUnknown é o default best-effort quando a heurística não reconhece
// a biblioteca. Honesto: o Cosca sabe que NÃO sabe o ecosystem.
const EcosystemUnknown = "unknown"

// PackagesDir é o diretório dos manifestos (relativo a .cosca/).
const PackagesDir = "knowledge/packages"

// ── KnowledgePackage ────────────────────────────────────────────────────────

// KnowledgePackage é o manifesto genérico de uma biblioteca conhecida.
type KnowledgePackage struct {
	ID             string    `json:"id"`                   // "prisma"
	Kind           string    `json:"kind"`                 // "library" | "framework" | "tool" | "runtime"
	Ecosystem      string    `json:"ecosystem"`            // "typescript" | "go" | "rust" | ...
	Versions       []string  `json:"versions"`             // ["6.x"] — families tracked
	Sources        []string  `json:"sources"`              // ["official-docs", "official-repository"]
	KnowledgeLevel string    `json:"knowledge_level"`      // "none"|"partial"|"validated" (epistemic-ish)
	DependsOn      []string  `json:"depends_on,omitempty"` // knowledge dependencies (Next→React)
	License        string    `json:"license,omitempty"`
	Repository     string    `json:"repository,omitempty"` // "prisma/prisma"
	AcquiredAt     time.Time `json:"acquired_at,omitempty"`
	ArtifactIDs    []string  `json:"artifact_ids,omitempty"` // A-XXXX refs
	SHA256         string    `json:"sha256,omitempty"`       // hash of acquired content
	Status         string    `json:"status"`                 // "manifest"|"acquiring"|"quarantined"|"validated"|"acquired"

	// VersionDiffs é a fonte determinística do Knowledge Diff: chave
	// "from->to" → entradas do que mudou entre as duas versões. Registrado
	// por um humano ou pelo pipeline de aquisição — o Cosca NUNCA inventa um
	// diff. Chaves reversas são derivadas por Diff (kinds negados), então
	// guardar apenas uma direção é suficiente.
	VersionDiffs map[string][]DiffEntry `json:"version_diffs,omitempty"`
}

// Validate valida o manifesto com erros claros em pt-BR: ID obrigatório,
// kind no conjunto conhecido e ecosystem obrigatório.
func (p *KnowledgePackage) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("knowledge: id é obrigatório (ex.: \"prisma\")")
	}
	switch p.Kind {
	case PackageKindLibrary, PackageKindFramework, PackageKindTool, PackageKindRuntime:
	default:
		return fmt.Errorf("knowledge: kind %q inválido — use library, framework, tool ou runtime", p.Kind)
	}
	if strings.TrimSpace(p.Ecosystem) == "" {
		return errors.New("knowledge: ecosystem é obrigatório (ex.: \"typescript\", \"go\", \"rust\")")
	}
	return nil
}

// ── Descoberta heurística (SEM rede) ────────────────────────────────────────

// ecosystemGuesses é a tabela estática de chute de ecosystem, documentada
// como BEST-EFFORT: mapeamento manual de bibliotecas conhecidas, sem nenhuma
// consulta de rede. A aquisição real (fontes/versões/releases) fica para o
// wire com internal/acquisition (uma etapa posterior).
var ecosystemGuesses = map[string]string{
	// typescript — a maioria do ecossistema JS/TS é agnóstica a runtime.
	"prisma": "typescript", "zod": "typescript", "next": "typescript",
	"nextjs": "typescript", "react": "typescript", "vue": "typescript",
	"express": "typescript", "fastify": "typescript", "hono": "typescript", "nestjs": "typescript",
	"ts-node": "typescript", "eslint": "typescript", "tailwindcss": "typescript",
	// go
	"pgx": "go", "gin": "go", "echo": "go", "chi": "go", "fiber": "go",
	"viper": "go", "cobra": "go", "zap": "go", "gorm": "go",
	"sqlc": "go", "ent": "go", "testify": "go", "grpc-go": "go",
	// rust — async/runtime
	"tokio": "rust", "async-std": "rust", "smol": "rust",
	// rust — web
	"axum": "rust", "actix": "rust", "actix-web": "rust", "hyper": "rust",
	"warp": "rust", "rocket": "rust", "tide": "rust", "poem": "rust",
	// rust — serde/validation
	"serde": "rust", "serde-json": "rust", "serde-yaml": "rust", "validator": "rust",
	// rust — CLI
	"clap": "rust", "structopt": "rust", "argh": "rust", "pico-args": "rust",
	// rust — db
	"tokio-postgres": "rust", "tokio-postgresql": "rust", "postgres": "rust",
	"sqlx": "rust", "diesel": "rust", "sea-orm": "rust", "redis": "rust", "mongodb": "rust",
	// rust — logging/observability
	"tracing": "rust", "log": "rust", "env_logger": "rust", "pretty-env-logger": "rust",
	// rust — error/other
	"anyhow": "rust", "thiserror": "rust", "tokio-util": "rust", "futures": "rust",
	"rayon": "rust", "rand": "rust", "chrono": "rust", "uuid": "rust",
	// python — web
	"fastapi": "python", "django": "python", "flask": "python",
	"starlette": "python", "aiohttp": "python", "tornado": "python",
	"bottle": "python", "sanic": "python",
	// python — data/ORM
	"sqlalchemy": "python", "pydantic": "python", "sqlmodel": "python",
	"peewee": "python", "tortoise-orm": "python", "psycopg2": "python",
	// python — test
	"pytest": "python", "unittest2": "python", "behave": "python", "hypothesis": "python",
	// python — HTTP/cloud
	"requests": "python", "httpx": "python", "boto3": "python", "google-cloud-storage": "python",
	// python — sci/other
	"numpy": "python", "pandas": "python", "scipy": "python", "scikit-learn": "python",
	"matplotlib": "python", "celery": "python", "redis-py": "python", "dotenv": "python",
	// python — IA/ML: node-based UIs (ComfyUI e afins)
	"comfyui": "python", "comfy-ui": "python", "comfy": "python",
	"stable-diffusion-webui": "python",
	// python — IA/ML: LLM/inference
	"openai": "python", "anthropic": "python", "ollama": "python",
	"langchain": "python", "langgraph": "python", "llama-index": "python",
	"transformers": "python", "torch": "python", "pytorch": "python",
	"tensorflow": "python", "jax": "python", "onnxruntime": "python",
	"sentence-transformers": "python", "vllm": "python", "huggingface-hub": "python",
	// python — IA/ML: ML/data
	"torchvision": "python", "torchaudio": "python", "diffusers": "python",
	"accelerate": "python", "peft": "python", "datasets": "python", "tokenizers": "python",
	"xgboost": "python", "lightgbm": "python", "statsmodels": "python",
	"seaborn": "python", "plotly": "python",
	// python — IA/ML: áudio/visão/voz
	"whisper": "python", "faster-whisper": "python", "kokoro": "python", "tts": "python",
	"stable-diffusion": "python", "controlnet": "python",
	"opencv-python": "python", "pillow": "python", "soundfile": "python", "librosa": "python",
	// python — IA/ML: agents
	"smolagents": "python", "crewai": "python", "autogen": "python",
	"pydantic-ai": "python", "instructor": "python",
	// java — frameworks/http
	"spring": "java", "spring-boot": "java", "spring-mvc": "java",
	"spring-security": "java", "spring-data": "java", "spring-cloud": "java",
	"springboot": "java", "quarkus": "java", "micronaut": "java",
	"vertx": "java", "vaadin": "java", "grails": "java",
	// java — persistence
	"hibernate": "java", "jpa": "java", "mybatis": "java",
	"flyway": "java", "liquibase": "java", "jooq": "java", "jdbc": "java",
	// java — testing
	"junit": "java", "junit5": "java", "testng": "java", "mockito": "java",
	"assertj": "java", "cucumber": "java",
	// java — build/tooling
	"maven": "java", "gradle": "java", "lombok": "java", "mapstruct": "java",
	"jackson": "java", "gson": "java",
	// java — observability/logging
	"slf4j": "java", "log4j": "java", "logback": "java",
	// java — cloud/outros
	"kafka-clients": "java", "apache-kafka": "java", "redis-client": "java",
	"jedis": "java", "lettuce": "java", "guava": "java",
	"apache-commons": "java", "rxjava": "java", "reactor": "java",
	"netty": "java", "opencsv": "java",
	// dotnet — web
	"aspnetcore": "dotnet", "mvc": "dotnet", "razor": "dotnet", "blazor": "dotnet",
	"minimal-api": "dotnet", "grpc-dotnet": "dotnet", "signalr": "dotnet",
	// dotnet — data
	"efcore": "dotnet", "dapper": "dotnet", "npgsql": "dotnet",
	"sqlite-net": "dotnet", "mongodb-driver": "dotnet",
	// dotnet — logging/di
	"serilog": "dotnet", "nlog": "dotnet", "microsoft-extensions-logging": "dotnet",
	"mediatr": "dotnet", "automapper": "dotnet",
	// dotnet — test/other
	"xunit": "dotnet", "nunit": "dotnet", "mstest": "dotnet",
	"fluentassertions": "dotnet", "moq": "dotnet", "polly": "dotnet",
	"newtonsoft-json": "dotnet", "system-text-json": "dotnet",
}

// DetectPackageInfo devolve o rascunho do manifesto a partir de um nome ou
// repositório, por heurística LOCAL (sem rede):
//
//	"prisma"                → id "prisma", ecosystem "typescript"
//	"github:prisma/prisma"  → id "prisma", repository "prisma/prisma"
//	"pgx"                   → ecosystem "go"
//	"tokio"                 → ecosystem "rust"
//
// kind é "library" por default; ecosystem desconhecido vira "unknown"
// (best-effort honesto). Status inicial "manifest".
func DetectPackageInfo(nameOrRepo string) (KnowledgePackage, error) {
	s := strings.TrimSpace(nameOrRepo)
	if s == "" {
		return KnowledgePackage{}, errors.New("knowledge: nome ou repositório vazio — ex.: \"prisma\" ou \"github:prisma/prisma\"")
	}

	pkg := KnowledgePackage{
		ID:             NormalizePackageID(s),
		Kind:           PackageKindLibrary,
		Ecosystem:      guessEcosystem(s),
		KnowledgeLevel: PackageKnowledgeNone,
		Status:         PackageStatusManifest,
	}
	if pkg.ID == "" {
		return KnowledgePackage{}, fmt.Errorf("knowledge: não foi possível derivar um id de %q", nameOrRepo)
	}
	if repo, ok := parseRepoRef(s); ok {
		pkg.Repository = repo
	}
	return pkg, nil
}

// NormalizePackageID deriva o id do pacote: minúsculas, prefixos de host
// removidos ("github:", "github.com/", ...) e o último segmento de caminho
// como id ("prisma/prisma" → "prisma"; "@scope/lib" → "lib").
func NormalizePackageID(nameOrRepo string) string {
	s := strings.TrimSpace(strings.ToLower(nameOrRepo))

	// Remove prefixos de host/scheme quando presentes.
	for _, prefix := range []string{
		"https://github.com/", "https://gitlab.com/", "https://bitbucket.org/",
		"github.com/", "gitlab.com/", "bitbucket.org/",
		"github:", "gitlab:", "bitbucket:", "gh:",
	} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}

	// IDs scoped (@scope/lib) e repos (org/lib) usam o último segmento.
	// Barras finais são ignoradas ("prisma/prisma/" → "prisma").
	s = strings.TrimRight(s, "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}

	// Remove um prefixo de scope eventualmente restante ("@lib").
	s = strings.TrimPrefix(strings.TrimSpace(s), "@")

	// Sanitiza: apenas [a-z0-9._-].
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// parseRepoRef extrai "org/repo" de uma referência de repositório
// ("github:prisma/prisma", "github.com/org/repo" ou "org/repo"). Sem
// referência de repo, ok=false.
func parseRepoRef(nameOrRepo string) (string, bool) {
	s := strings.TrimSpace(nameOrRepo)

	path := s
	for _, prefix := range []string{
		"https://github.com/", "https://gitlab.com/", "https://bitbucket.org/",
		"github.com/", "gitlab.com/", "bitbucket.org/",
	} {
		if strings.HasPrefix(s, prefix) {
			path = strings.TrimPrefix(s, prefix)
			break
		}
	}
	for _, prefix := range []string{"github:", "gitlab:", "bitbucket:", "gh:"} {
		if strings.HasPrefix(s, prefix) {
			path = strings.TrimPrefix(s, prefix)
			break
		}
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
		return parts[0] + "/" + parts[1], true
	}
	return "", false
}

// guessEcosystem consulta a tabela estática best-effort. Desconhecido →
// "unknown" (o Cosca sabe que NÃO sabe).
func guessEcosystem(nameOrRepo string) string {
	if ec, ok := ecosystemGuesses[NormalizePackageID(nameOrRepo)]; ok {
		return ec
	}
	return EcosystemUnknown
}

// ── PackageStore ────────────────────────────────────────────────────────────

// PackageStore persiste os manifestos em .cosca/knowledge/packages/<id>.json.
// A gravação é atômica (tmp + rename) e usa 0700/0600 como o resto da árvore
// .cosca (M6b — o conhecimento não deve ser world-readable).
type PackageStore struct {
	dir string // <projeto>/.cosca/knowledge/packages
}

// NewPackageStore cria a store do projeto raiz dir. Preguiçosa: o diretório
// é criado na primeira escrita.
func NewPackageStore(dir string) *PackageStore {
	return &PackageStore{
		dir: filepath.Join(dir, ".cosca", filepath.FromSlash(PackagesDir)),
	}
}

// Path retorna o diretório dos manifestos (.cosca/knowledge/packages).
func (s *PackageStore) Path() string {
	return s.dir
}

// Add valida e grava (ou sobrescreve) o manifesto <id>.json.
func (s *PackageStore) Add(p KnowledgePackage) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("knowledge: criar %q: %w", s.dir, err)
	}
	if err := os.Chmod(s.dir, 0o700); err != nil {
		return fmt.Errorf("knowledge: restringir %q: %w", s.dir, err)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("knowledge: serializar %s: %w", p.ID, err)
	}
	data = append(data, '\n')

	target := filepath.Join(s.dir, p.ID+".json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("knowledge: escrever tmp %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("knowledge: renomear %q → %q: %w", tmp, target, err)
	}
	return nil
}

// Get lê o manifesto <id>.json.
func (s *PackageStore) Get(id string) (*KnowledgePackage, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("knowledge: id vazio")
	}
	data, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("knowledge: Knowledge Package %q não encontrado (manifestos em %s)", id, s.dir)
		}
		return nil, fmt.Errorf("knowledge: ler %s: %w", id, err)
	}
	var p KnowledgePackage
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("knowledge: parsear %s: %w", id, err)
	}
	return &p, nil
}

// List retorna todos os manifestos, ordenados por ID. Diretório
// ausente/vazio resulta em lista vazia, não erro.
func (s *PackageStore) List() ([]KnowledgePackage, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("knowledge: ler %q: %w", s.dir, err)
	}
	var pkgs []KnowledgePackage
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p, err := s.Get(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, *p)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ID < pkgs[j].ID })
	return pkgs, nil
}

// Exists devolve true quando o manifesto <id>.json existe.
func (s *PackageStore) Exists(id string) bool {
	if strings.TrimSpace(id) == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(s.dir, id+".json"))
	return err == nil
}

// ── GlobalPackageStore ──────────────────────────────────────────────────────

// NewGlobalPackageStore cria uma store no diretório global do Cosca
// (~/.config/cosca/knowledge/packages/), compartilhada entre todos os projetos.
// O diretório é criado na primeira escrita — leitura de store vazia é ok.
func NewGlobalPackageStore() (*PackageStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("knowledge: home dir: %w", err)
	}
	return &PackageStore{
		dir: filepath.Join(home, ".config", "cosca", filepath.FromSlash(PackagesDir)),
	}, nil
}

// ── Federated store helpers ─────────────────────────────────────────────────

// FederatedGet procura um package primeiro na store local, depois na global.
// Retorna nil, nil se não encontrado em nenhuma.
func FederatedGet(local, global *PackageStore, id string) (*KnowledgePackage, error) {
	if local != nil {
		pkg, err := local.Get(id)
		if err == nil {
			return pkg, nil
		}
	}
	if global != nil {
		pkg, err := global.Get(id)
		if err == nil {
			return pkg, nil
		}
	}
	return nil, fmt.Errorf("knowledge: Knowledge Package %q não encontrado (local + global)", id)
}

// FederatedList retorna a união dos packages locais e globais, sem duplicatas.
// Packages locais têm precedência (sobrescrevem globais com mesmo ID).
func FederatedList(local, global *PackageStore) ([]KnowledgePackage, error) {
	seen := make(map[string]bool)
	var pkgs []KnowledgePackage

	// Globais primeiro (locals sobrescrevem)
	if global != nil {
		globals, err := global.List()
		if err != nil {
			return nil, err
		}
		for _, p := range globals {
			seen[p.ID] = true
			pkgs = append(pkgs, p)
		}
	}

	// Locais depois (sobrescrevem globais)
	if local != nil {
		locals, err := local.List()
		if err != nil {
			return nil, err
		}
		for _, p := range locals {
			if seen[p.ID] {
				// Substitui o global pelo local
				for i := range pkgs {
					if pkgs[i].ID == p.ID {
						pkgs[i] = p
						break
					}
				}
			} else {
				pkgs = append(pkgs, p)
			}
		}
	}

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ID < pkgs[j].ID })
	return pkgs, nil
}

// FederatedExists verifica se o package existe na store local OU na global.
func FederatedExists(local, global *PackageStore, id string) bool {
	if local != nil && local.Exists(id) {
		return true
	}
	if global != nil && global.Exists(id) {
		return true
	}
	return false
}
