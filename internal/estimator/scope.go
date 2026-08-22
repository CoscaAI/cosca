package estimator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ExecutionScope descreve o escopo de uma execução planejada. Ele é a entrada
// mínima que o Execution Plan Estimator precisa para produzir a estimativa de
// transparência ANTES de qualquer aprovação de execução.
type ExecutionScope struct {
	// ProjectDir é a raiz do projeto (ex: "/home/cosca/Documents/cosca").
	ProjectDir string

	// TargetFiles é uma lista de globs ou caminhos literais dos arquivos que
	// serão alterados (ex: ["internal/kernel/*.go", "api/rest/**"]). Caminhos
	// literais são incluídos mesmo que o arquivo ainda não exista (será criado).
	TargetFiles []string

	// ChangeType classifica a alteração: "feature", "fix", "security",
	// "refactor", "docs" ou "test".
	ChangeType string

	// Agent é o agente executor (ex: "cosca-backend").
	Agent string
}

// ExpandScope expande os globs do escopo em arquivos reais. Suporta:
//
//   - globs padrão via filepath.Glob ("internal/kernel/*.go", "api/rest/*.go")
//   - globs recursivos com "**" ("api/rest/**" casa qualquer arquivo abaixo de
//     api/rest, em qualquer profundidade)
//   - caminhos literais, que são incluídos mesmo sem existir em disco
//     (arquivos que serão criados pela execução)
//
// Retorna caminhos absolutos, ordenados e sem duplicatas.
func ExpandScope(scope ExecutionScope) ([]string, error) {
	if scope.ProjectDir == "" {
		return nil, fmt.Errorf("estimator: ProjectDir é obrigatório")
	}
	info, err := os.Stat(scope.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("estimator: project dir %q: %w", scope.ProjectDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("estimator: project dir %q não é um diretório", scope.ProjectDir)
	}

	var files []string
	seen := make(map[string]struct{})
	for _, raw := range scope.TargetFiles {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}
		matches, err := expandPattern(scope.ProjectDir, pattern)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			files = append(files, m)
		}
	}
	sort.Strings(files)
	return files, nil
}

// expandPattern resolve um único padrão (glob ou literal) em caminhos absolutos.
func expandPattern(projectDir, pattern string) ([]string, error) {
	if strings.Contains(pattern, "**") {
		return expandDoubleStar(projectDir, pattern)
	}
	if filepath.IsAbs(pattern) {
		return globOrLiteral(pattern)
	}
	return globOrLiteral(filepath.Join(projectDir, filepath.FromSlash(pattern)))
}

// globOrLiteral resolve um caminho sem "**". Se não houver metacaractere de
// glob, trata como literal (o arquivo pode nem existir ainda). Caso contrário,
// delega ao filepath.Glob e descarta diretórios.
func globOrLiteral(full string) ([]string, error) {
	if !hasGlobMeta(full) {
		return []string{full}, nil
	}
	matches, err := filepath.Glob(full)
	if err != nil {
		return nil, fmt.Errorf("estimator: expand glob %q: %w", full, err)
	}
	var out []string
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil && !fi.IsDir() {
			out = append(out, m)
		}
	}
	return out, nil
}

// expandDoubleStar resolve padrões com "**" caminhando a árvore do projeto e
// casando o caminho relativo com uma regex derivada do glob.
func expandDoubleStar(projectDir, pattern string) ([]string, error) {
	slashed := filepath.ToSlash(pattern)
	isAbs := filepath.IsAbs(pattern)
	re, err := globPatternToRegex(slashed)
	if err != nil {
		return nil, fmt.Errorf("estimator: expand glob %q: %w", pattern, err)
	}

	var out []string
	walkErr := filepath.Walk(projectDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil // ignora entradas ilegíveis
		}
		if fi.IsDir() {
			return nil
		}
		subject := filepath.ToSlash(path)
		if !isAbs {
			rel, rerr := filepath.Rel(projectDir, path)
			if rerr != nil {
				return nil
			}
			subject = filepath.ToSlash(rel)
		}
		if re.MatchString(subject) {
			out = append(out, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("estimator: walk %q: %w", projectDir, walkErr)
	}
	return out, nil
}

// globPatternToRegex converte um glob slash-separated em uma regex ancorada.
// "**" casa zero ou mais segmentos de caminho; "*" casa um segmento; "?" casa
// um caractere dentro de um segmento. Metacaracteres de regex são escapados.
func globPatternToRegex(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '(', ')', '+', '|', '^', '$', '[', ']', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// hasGlobMeta reporta se o caminho contém metacaracteres de glob do Go
// (filepath.Match): '*', '?' e '['.
func hasGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

// testFuncRe casa a declaração de uma função de teste Go no início da linha.
// Segue a convenção do "go test": TestMain ou Test<Upper>.
var testFuncRe = regexp.MustCompile(`^func\s+Test[A-Z0-9_]`)

// CountTests conta o número de funções de teste (linhas "func Test..." não
// comentadas) em todos os arquivos *_test.go dentro de cada pacote informado.
//
// Pacotes que não existem em disco são ignorados (contribuem 0), para que a
// estimativa funcione com diretórios ainda não criados.
func CountTests(packages []string) (int, error) {
	total := 0
	for _, pkg := range packages {
		n, err := countTestsInPackage(pkg)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func countTestsInPackage(pkg string) (int, error) {
	info, err := os.Stat(pkg)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("estimator: stat package %q: %w", pkg, err)
	}
	if !info.IsDir() {
		return 0, nil
	}

	count := 0
	walkErr := filepath.Walk(pkg, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		n, err := countTestFuncs(path)
		if err != nil {
			return fmt.Errorf("estimator: count tests in %q: %w", path, err)
		}
		count += n
		return nil
	})
	if walkErr != nil {
		return 0, fmt.Errorf("estimator: walk package %q: %w", pkg, walkErr)
	}
	return count, nil
}

// countTestFuncs lê um arquivo _test.go linha a linha e conta as declarações
// de função de teste, ignorando comentários de linha ("//"), blocos ("/* */")
// e strings que acidentalmente comecem a linha com "func Test".
func countTestFuncs(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	inBlock := false
	count := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if inBlock {
			if strings.Contains(line, "*/") {
				inBlock = false
			}
			continue
		}
		if strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "/*") {
			if !strings.Contains(line, "*/") {
				inBlock = true
			}
			continue
		}
		// comentário de bloco inline ("código /* ... */"): corta no "/*"
		if idx := strings.Index(line, "/*"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if testFuncRe.MatchString(line) {
			count++
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return count, nil
}
