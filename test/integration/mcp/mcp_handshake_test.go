//go:build integration

// Package mcp — testes de integração do SERVIDOR MCP do COSCA.
//
// Garantem a não-recorrência do bug "cosca Operation timed out after 30000ms":
// o handshake initialize/tools-list deve responder rápido ao lançar o binário
// real como subprocesso (o mesmo caminho que o OpenCode usa).
//
// Executar com:
//
//	go test -tags integration -count=1 -v ./test/integration/mcp/
package mcp

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// handshakeTimeout é o limite do handshake. Deve ser BEM MENOR que os 30s de
// timeout do host OpenCode — se o binário real não responder rápido, o bug
// original voltou.
const handshakeTimeout = 20 * time.Second

// TestMCPHandshakeRealBinary lança o binário real `bin/cosca.exe mcp serve`
// (como o OpenCode faz via stdio), injeta initialize + tools/list, fecha o
// stdin e mede a latência do handshake. Falha lento/travado => regressão do
// bug do timeout de 30s.
func TestMCPHandshakeRealBinary(t *testing.T) {
	bin := findBin(t)

	// Roda o subprocesso num diretório temporário: o runtime (knowledge.db,
	// activity, etc.) é derivado e NÃO deve poluir o repositório do projeto.
	workDir := t.TempDir()

	cmd := exec.Command(bin, "mcp", "serve")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"COSCA_ENABLE_MCP=1",
		"COSCA_ALLOW_NO_ROOT=1",
		"COSCA_JAILED=1",
	)
	cmd.Stderr = os.Stderr // stderr direto — logs nunca vão pro stdout (protocolo)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Garante que o processo é morto se o teste falhar no meio.
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	// Escreve initialize + tools/list e FECHA o stdin (EOF real).
	start := time.Now()
	fmt.Fprint(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`+"\n")
	fmt.Fprint(stdin, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`+"\n")
	_ = stdin.Close()

	// Lê o stdout até encontrar ambas as respostas ou estourar o timeout.
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 4*1024*1024), 4*1024*1024)
	var gotInit, gotList bool
	var initLatency, listLatency time.Duration
	routes := make(chan struct{}, 2)

	go func() {
		for sc.Scan() {
			line := sc.Text()
			if bytes.Contains([]byte(line), []byte(`"id":1`)) && !gotInit {
				gotInit = true
				initLatency = time.Since(start)
				routes <- struct{}{}
			}
			if bytes.Contains([]byte(line), []byte(`"id":2`)) && !gotList {
				gotList = true
				listLatency = time.Since(start)
				routes <- struct{}{}
			}
		}
	}()

	timeout := time.After(handshakeTimeout)
	for !gotInit || !gotList {
		select {
		case <-routes:
		case <-timeout:
			t.Fatalf("TIMEOUT %s: gotInitialize=%v gotToolsList=%v (o bug do 'Operation timed out' voltou?)",
				handshakeTimeout, gotInit, gotList)
		}
	}

	if initLatency > 5*time.Second || listLatency > 5*time.Second {
		t.Fatalf("handshake lento: initialize=%dms tools/list=%dms — boot ainda no caminho crítico",
			initLatency.Milliseconds(), listLatency.Milliseconds())
	}

	t.Logf("initialize=%dms tools/list=%dms — handshake MCP rápido, sem regressão",
		initLatency.Milliseconds(), listLatency.Milliseconds())
}

// TestMCPStdinEOF termina limpo: após fechar o stdin, o subprocesso deve
// encerrar (Serve retorna nil no EOF) em vez de ficar vivo pendurado.
func TestMCPStdinEOF(t *testing.T) {
	bin := findBin(t)
	cmd := exec.Command(bin, "mcp", "serve")
	cmd.Dir = t.TempDir() // runtime derivado fora do repo
	cmd.Env = append(os.Environ(),
		"COSCA_ENABLE_MCP=1",
		"COSCA_ALLOW_NO_ROOT=1",
		"COSCA_JAILED=1",
	)
	cmd.Stderr = os.Stderr

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	fmt.Fprint(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`+"\n")
	_ = stdin.Close() // EOF

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-time.After(8 * time.Second):
		t.Fatal("processo não terminou após EOF — Serve não está respeitando o fim do stdin")
	case err := <-done:
		if err != nil {
			t.Logf("processo encerrou (exit não-zero, aceitável se já respondeu): %v", err)
		}
	}
	_ = stdout.Close()
}

// findBin localiza o binário cosca.exe no caminho esperado.
func findBin(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// test/integration/mcp → raiz do projeto é 3 níveis acima.
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	candidates := []string{
		filepath.Join(root, "bin", "cosca.exe"),
		filepath.Join(root, "cosca.exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Fatalf("binário cosca não encontrado; procure em %s", strings.Join(candidates, " ou "))
	return ""
}
