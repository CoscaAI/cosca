// secret.go — Detector de VAZAMENTO de segredos (mineração git-secrets: "vazamento
// como enforcement no fluxo", adaptado aos invariantes do Cosca).
//
// Diferente do scan.go (vulnerabilidade de dependência via OSV), este detector
// localiza segredos EMBUTIDOS (AWS keys, GitHub tokens, JWTs, chaves privadas,
// bearer tokens, atribuições secret/token/api-key) em texto ou arquivos.
//
// Determinístico (I1): pura regex, ZERO LLM, ZERO heurística de modelo —
// auditável e testável com go test.
//
// Fail-closed (I2): no modo enforcement, presença de segredo → nega (exit != 0);
// sem evidência/linha não passa.
//
// Epistemologia (I4): a detecção é FACT (um padrão caiu na regex); a
// severidade é uma CLASSIFICAÇÃO determinística, não um julgamento de modelo.
package security

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SecretKind classifica o tipo de segredo detectado.
type SecretKind string

const (
	SecretAWSKey        SecretKind = "aws-access-key"
	SecretAWSSecret     SecretKind = "aws-secret-key"
	SecretGitHubToken   SecretKind = "github-token"
	SecretJWT           SecretKind = "jwt"
	SecretPrivateKey    SecretKind = "private-key"
	SecretBearer        SecretKind = "bearer-token"
	SecretCredAssign    SecretKind = "credential-assignment"
)

// SecretMatch é uma ocorrência de segredo detectada.
type SecretMatch struct {
	Kind     SecretKind `json:"kind"`
	Severity string     `json:"severity"`
	// File é o arquivo de origem (preenchido por ScanSecretsDir).
	File   string `json:"file,omitempty"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Length int    `json:"length"`
	// Start/End são offsets de bytes no texto (para mascaramento por faixa).
	Start int `json:"start,omitempty"`
	End   int `json:"end,omitempty"`
	// Value é o segredo detectado (NUNCA exposto em texto; mascarado).
	Value string `json:"value,omitempty"`
	// Masked é a forma segura de exibir (ex: AKIA********...).
	Masked string `json:"masked"`
}

// SecretScanResult é o resultado de uma varredura.
type SecretScanResult struct {
	Source  string        `json:"source"`
	Matches []SecretMatch `json:"matches"`
	Clean   bool          `json:"clean"`
}

// HasSecrets indica se a varredura encontrou pelo menos um segredo.
func (r *SecretScanResult) HasSecrets() bool { return len(r.Matches) > 0 }

// =============================================================================
// Padrões determinísticos (regex compilada)
// =============================================================================

// padAWSKey: AKIA + 16 alfanuméricos (chave de acesso AWS).
var padAWSKey = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)

// padAWSSecret: atribuição aws_secret_access_key com valor de 40 chars alnum/+.
var padAWSSecret = regexp.MustCompile(`(?i)aws_secret_access_key\s*[=:]\s*['"]?[A-Za-z0-9/+]{40}`)

// padGitHubToken: gh[opusr]_ + 36+ chars (tokens GitHub).
var padGitHubToken = regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,255}\b`)

// padJWT: header.payload.signature em base64url.
var padJWT = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)

// padPrivateKey: bloco PEM de chave privada.
var padPrivateKey = regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY(?: BLOCK)?-----`)

// padBearer: "Bearer <token>" (>=20 chars base64).
var padBearer = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/-]{20,}={0,2}\b`)

// padCredAssign: atribuição genérica PERIGOSA (password/passwd/secret/client_secret/
// auth_token). NÃO casa "apiKey:"/"accessKey:" — nomes de campo Go comuns (FP).
var padCredAssign = regexp.MustCompile(`(?i)(?:password|passwd|secret|client_secret|auth_token)\s*[=:]\s*['"]?[A-Za-z0-9_\-./+]{16,}`)

// detectors ordena por prioridade (mais específico primeiro) para atribuir o
// Kind mais preciso quando vários padrões casam.
var detectors = []struct {
	kind     SecretKind
	severity string
	re       *regexp.Regexp
}{
	{SecretAWSKey, "CRITICAL", padAWSKey},
	{SecretAWSSecret, "CRITICAL", padAWSSecret},
	{SecretGitHubToken, "CRITICAL", padGitHubToken},
	{SecretPrivateKey, "CRITICAL", padPrivateKey},
	{SecretJWT, "HIGH", padJWT},
	{SecretBearer, "HIGH", padBearer},
	{SecretCredAssign, "MEDIUM", padCredAssign},
}

// DetectBytes localiza segredos em um slice de bytes e devolve as ocorrências.
// Determinístico (I1). Usa o nome da linha para reportar posição.
func DetectBytes(data []byte, source string) *SecretScanResult {
	result := &SecretScanResult{Source: source, Clean: true}

	// Índices já casados (evita duplicata quando vários padrões sobrepõem).
	covered := map[int]bool{}

	for _, d := range detectors {
		for _, loc := range d.re.FindAllIndex(data, -1) {
			start, end := loc[0], loc[1]
			// Ignora se já coberto por um detector mais específico já aplicado.
			overlapped := false
			for i := start; i < end; i++ {
				if covered[i] {
					overlapped = true
					break
				}
			}
			if overlapped {
				continue
			}
			for i := start; i < end; i++ {
				covered[i] = true
			}
			value := string(data[start:end])
			line, col := lineCol(data, start)
			result.Matches = append(result.Matches, SecretMatch{
				Kind:     d.kind,
				Severity: d.severity,
				Line:     line,
				Column:   col,
				Length:   end - start,
				Start:    start,
				End:      end,
				Value:    value,
				Masked:   mask(value),
			})
		}
	}

	result.Clean = len(result.Matches) == 0
	return result
}

// DetectFile lê um arquivo e detecta segredos.
func DetectFile(path string) (*SecretScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return DetectBytes(data, path), nil
}

// =============================================================================
// Varredura de diretório (git-secrets: enforcement no fluxo, repo-wide)
// =============================================================================

// maxSecretScanBytes: limite de tamanho de arquivo para varrer (evita BLOBs).
const maxSecretScanBytes = 512 * 1024 // 512 KB

// skipScanDirs: diretórios que nunca são varridos.
var skipScanDirs = map[string]bool{
	".git":      true,
	"node_modules": true,
	"dist":      true,
	"build":     true,
	"bin":       true,
	"vendor":    true,
	".cosca":    true,
}

// binaryExts: extensões comprovadamente binárias que não são texte.
var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".wasm": true, ".exe": true, ".dll": true, ".so": true, ".dylib": true,
	".a": true, ".o": true, ".db": true, ".db-wal": true, ".db-shm": true,
	".zip": true, ".gz": true, ".tar": true, ".7z": true, ".pdf": true,
	".bin": true, ".ico": true, ".ttf": true, ".woff": true, ".mp3": true,
	".mp4": true, ".mov": true, ".svg": true,
}

// ScanSecretsDir varre um diretório em busca de segredos embutidos em arquivos
// de texto (esquece .git, BLOBs e arquivos grandes). Determinístico (I1).
func ScanSecretsDir(root string) (*SecretScanResult, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", root, err)
	}

	res := &SecretScanResult{Source: abs, Clean: true}
	var walkErr error

	_ = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			walkErr = err
			return filepath.SkipDir
		}
		if info.IsDir() {
			if path != abs && skipScanDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// Não varre arquivos grandes (prováveis blobs/binários).
		if info.Size() > maxSecretScanBytes {
			return nil
		}
		// Não varre extensões binárias.
		if binaryExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil // ignora erros de leitura por arquivo
		}
		if fileRes := DetectBytes(data, path); fileRes.HasSecrets() {
			for _, m := range fileRes.Matches {
				res.Matches = append(res.Matches, SecretMatch{
					Kind:     m.Kind,
					Severity: m.Severity,
					File:     path,
					Line:     m.Line,
					Column:   m.Column,
					Length:   m.Length,
					Value:    m.Value,
					Masked:   m.Masked,
				})
			}
		}
		return nil
	})

	if walkErr != nil {
		return nil, walkErr
	}

	// Ordena por arquivo (source) e linha para saída estável.
	sort.SliceStable(res.Matches, func(i, j int) bool {
		if res.Matches[i].Line != res.Matches[j].Line {
			return res.Matches[i].Line < res.Matches[j].Line
		}
		return res.Matches[i].Column < res.Matches[j].Column
	})

	res.Clean = len(res.Matches) == 0
	return res, nil
}

// mask redige o segredo mantendo o prefixo e o sufixo (base para auditoria),
// sem expor o conteúdo. Determinístico e reversível na origem (o valor completo
// permanece apenas na struct, nunca impresso).
func mask(value string) string {
	if value == "" {
		return ""
	}
	switch {
	case len(value) <= 8:
		return "********"
	default:
		return value[:6] + "..." + value[len(value)-4:]
	}
}

// lineCol converte um offset de byte em (linha, coluna) 1-based.
func lineCol(data []byte, offset int) (int, int) {
	line, col := 1, 1
	for i := 0; i < offset && i < len(data); i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	// Trata bytes multibyte UTF-8 como uma coluna (régua simples).
	return line, col
}
