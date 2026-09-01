// server.go — O SERVIDOR MCP do COSCA (JSON-RPC 2.0, stdio/NDJSON).
//
// O servidor é a interface FINA (nervo) que traduz JSON-RPC 2.0 do stdin para
// os órgãos do COSCA (via Engine), e devolve context packets (não funções).
// Ele ESPELHA exatamente o contrato do cliente MCP (internal/chat/mcp):
// mesmos métodos (initialize / tools/list / tools/call), mesmo formato de
// mensagens, mesma estrutura ToolInfo. Simetria cliente↔servidor.
//
// Testável: aceita um io.Reader (entrada) e um io.Writer (saída) injetáveis.
package mcpserver

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/chat/mcp"
)

// ─── JSON-RPC 2.0 Types (espelham o cliente MCP) ─────────────────────────

// request é uma requisição JSON-RPC 2.0.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// response é uma resposta JSON-RPC 2.0.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError é um objeto de erro JSON-RPC 2.0.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implementa error.
func (e *rpcError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// toolsListResult é o payload de tools/list (mesmo shape do cliente).
type toolsListResult struct {
	Tools []mcp.ToolInfo `json:"tools"`
}

// toolCallParams é o params de tools/call.
type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// toolCallResult é o payload de tools/call (formato MCP content).
type toolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ─── Códigos de erro JSON-RPC (convenção MCP/JSON-RPC) ────────────────────
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeServerError    = -32000
)

// ─── Server ───────────────────────────────────────────────────────────────

// Server é o servidor MCP stdio. Lê JSON-RPC 2.0 (uma linha por mensagem,
// NDJSON) do input e escreve no output.
type Server struct {
	engine *Engine
	scanner *bufio.Scanner
	out     io.Writer

	// outMu serializa a escrita de respostas no stdout (uma por vez).
	outMu sync.Mutex

	// replayCache protege contra REPLAY de tools/call: o OpenCode (ou um retry
	// do cliente) pode reenviar a MESMA chamada (mesmo name+arguments) quando o
	// stream trava ou o usuário manda continuar. Sem proteção, cada reenvio
	// RE-EXECUTA a tool — custo duplicado de LLM/tokens e efeitos repetidos.
	// A chave é um hash determinístico de name+arguments; o valor é o resultado
	// já computado, servido do cache em vez de re-executar.
	replayCache *replayStore
}

// replayEntry guarda um resultado de tools/call já computado, com expiração
// (TTL curto — o replay só importa na janela de retry do cliente).
type replayEntry struct {
	result  *toolCallResult
	rpcErr  *rpcError
	expires time.Time
}

// replayStore é o cache de replay com mutex (concorrente: o Serve é
// single-threaded, mas testes e chamadas futuras podem paralelizar).
type replayStore struct {
	mu    sync.Mutex
	items map[string]replayEntry
	// ttl é o tempo de vida de uma entrada de replay. 5min cobre retries
	// humanos e de rede sem segurar memória indefinidamente.
	ttl time.Duration
}

func newReplayStore(ttl time.Duration) *replayStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &replayStore{items: make(map[string]replayEntry), ttl: ttl}
}

// get devolve a entrada de replay para a chave, se existir e não tiver
// expirado (expirada é removida e tratada como ausente).
func (r *replayStore) get(key string) (replayEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.items[key]
	if !ok {
		return replayEntry{}, false
	}
	if time.Now().After(entry.expires) {
		delete(r.items, key)
		return replayEntry{}, false
	}
	return entry, true
}

// set armazena o resultado de replay para a chave.
func (r *replayStore) set(key string, entry replayEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.expires = time.Now().Add(r.ttl)
	r.items[key] = entry
}

// NewServer cria um servidor MCP sobre um engine e um par in/out injetável.
// O input é lido como NDJSON; o output recebe NDJSON. Usado em produção com
// os.Stdin/os.Stdout e em testes com bytes.Buffer.
func NewServer(engine *Engine, input io.Reader, output io.Writer) *Server {
	s := bufio.NewScanner(input)
	s.Buffer(make([]byte, 0, 256*1024), 256*1024)
	return &Server{
		engine:      engine,
		scanner:     s,
		out:         output,
		replayCache: newReplayStore(5 * time.Minute),
	}
}

// Serve processa requisições até EOF ou cancelamento do contexto. Retorna nil
// no EOF limpo (o subprocesso termina com o fluxo de entrada).
func (s *Server) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				return fmt.Errorf("mcp server read: %w", err)
			}
			return nil // EOF — fluxo de entrada encerrado
		}
		line := s.scanner.Bytes()
		resp, shouldRespond := s.handleLine(line)
		if !shouldRespond {
			continue
		}
		if err := s.write(resp); err != nil {
			return fmt.Errorf("mcp server write: %w", err)
		}
	}
}

// handleLine interpreta uma linha NDJSON. Devolve a resposta e um bool que
// indica se deve ser escrita (notificações sem id não geram resposta).
func (s *Server) handleLine(line []byte) (response, bool) {
	req, err := parseRequest(line)
	if err != nil {
		return errorResponse(nil, codeParseError, "parse error: "+err.Error()), true
	}
	// JSON-RPC: requisição sem id é NOTIFICAÇÃO → não responde.
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return response{}, false
	}
	if req.JSONRPC != "2.0" {
		return errorResponse(req.ID, codeInvalidParams, "jsonrpc deve ser '2.0'"), true
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req), true
	case "tools/list":
		return s.handleToolsList(req), true
	case "tools/call":
		return s.handleToolCall(req), true
	default:
		return errorResponse(req.ID, codeMethodNotFound, "method not found: "+req.Method), true
	}
}

// ─── Handlers por método ─────────────────────────────────────────────────

// initialize responde o handshake MCP. O servidor ECOA a versão de protocolo
// que o cliente pediu (params.protocolVersion) — no MCP moderno o servidor
// suporta a versão compatível com o cliente, não impõe a sua. Fallback: a
// versão moderna padrão "2024-11-05" (a "0.1.0" é legacy e o OpenCode rejeita).
func (s *Server) handleInitialize(req request) response {
	protocolVersion := "2024-11-05"
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err == nil && params.ProtocolVersion != "" {
			protocolVersion = params.ProtocolVersion
		}
	}
	result := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]string{
			"name":    "cosca-mcp",
			"version": "1.5.0",
		},
	}
	return successResponse(req.ID, mustMarshal(result))
}

// tools/list devolve o inventário de tools cognitivas anunciado (mesmo shape
// do cliente: ToolInfo name/description/inputSchema).
func (s *Server) handleToolsList(req request) response {
	var tools []mcp.ToolInfo
	if s.engine != nil {
		tools = s.engine.Tools()
	}
	if tools == nil {
		tools = []mcp.ToolInfo{}
	}
	return successResponse(req.ID, mustMarshal(toolsListResult{Tools: tools}))
}

// handleToolCall roteia tools/call para o Engine.Call. Tool desconhecida ou
// erro de execução → erro JSON-RPC (fail-closed). Sucesso → resultado MCP.
//
// PROTEÇÃO DE REPLAY: a mesma chamada (mesmo name+arguments) reenviada dentro
// da janela de TTL (ex.: o OpenCode reenviou porque o stream travou, ou o
// usuário mandou continuar) NÃO re-executa — devolve o resultado em cache.
// Isso elimina o custo duplicado de LLM/tokens e efeitos repetidos.
func (s *Server) handleToolCall(req request) response {
	params := toolCallParams{}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errorResponse(req.ID, codeInvalidParams, "invalid params: "+err.Error())
		}
	}
	if params.Name == "" {
		return errorResponse(req.ID, codeInvalidParams, "tools/call: 'name' é obrigatório")
	}
	if s.engine == nil {
		return errorResponse(req.ID, codeServerError, "cosca.mcp: servidor sem engine (dependências não injetadas)")
	}

	// Valida a tool CONHECIDA antes de executar (default-deny, I8).
	if !s.engine.hasTool(params.Name) {
		return errorResponse(req.ID, codeInvalidParams, fmt.Sprintf("tool %q não encontrada", params.Name))
	}

	// ── Replay guard: chave determinística da chamada (name + arguments). ──
	key := replayKey(params.Name, params.Arguments)
	if s.replayCache != nil {
		if entry, ok := s.replayCache.get(key); ok {
			if entry.rpcErr != nil {
				return errorResponse(req.ID, entry.rpcErr.Code, entry.rpcErr.Message)
			}
			return successResponse(req.ID, mustMarshal(entry.result))
		}
	}

	// ── Timeout de execução (cada tool tem teto; o MCP nunca trava o loop
	// para sempre). O OpenCode tem timeout próprio — se a tool exceder,
	// devolvemos erro claro em vez de bloquear o stdio indefinidamente.
	callCtx, cancel := context.WithTimeout(context.Background(), defaultToolCallTimeout)
	defer cancel()

	callResult, err := s.engine.Call(callCtx, params.Name, params.Arguments)
	if err != nil {
		// Falha dura (sem órgão, kernel halted, erro interno) → erro JSON-RPC.
		rpcErr := errorResponse(req.ID, codeServerError, err.Error()).Error
		if s.replayCache != nil {
			s.replayCache.set(key, replayEntry{rpcErr: rpcErr})
		}
		return errorResponse(req.ID, codeServerError, err.Error())
	}
	res := toolCallResult{
		Content: callResult.Content,
		IsError: callResult.IsError,
	}
	if s.replayCache != nil {
		s.replayCache.set(key, replayEntry{result: &res})
	}
	return successResponse(req.ID, mustMarshal(res))
}

// defaultToolCallTimeout é o teto de execução de uma tool MCP. Um retry do
// cliente reenviaria a chamada dentro da janela de replay (5min), então o
// timeout não perde trabalho — apenas evita o bloqueio infinito do stdio.
const defaultToolCallTimeout = 60 * time.Second

// replayKey devolve a chave determinística de uma chamada de tool — o hash
// SHA-256 de name + arguments. A mesma intenção reenviada gera a mesma chave;
// intenções diferentes (mesmo nome, args diferentes) geram chaves diferentes.
func replayKey(name string, arguments json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte{0})
	if len(arguments) > 0 {
		h.Write(arguments)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ─── JSON-RPC helpers ─────────────────────────────────────────────────────

// parseRequest decodifica uma linha NDJSON em uma requisição.
func parseRequest(line []byte) (request, error) {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return req, err
	}
	return req, nil
}

// successResponse monta uma resposta com result.
func successResponse(id json.RawMessage, result json.RawMessage) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

// errorResponse monta uma resposta de erro JSON-RPC.
func errorResponse(id json.RawMessage, code int, message string) response {
	data := json.RawMessage(nil)
	return response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: strings.TrimSpace(message), Data: data},
	}
}

// mustMarshal serializa um valor para json.RawMessage (impossível de falhar
// com os tipos usados internamente; em caso de erro devolve "null").
func mustMarshal(v interface{}) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return raw
}

// write envia uma resposta como NDJSON (uma linha por objeto) ao output.
// Serializa a escrita para nunca entrelaçar duas respostas.
func (s *Server) write(resp response) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	raw = append(raw, '\n')
	s.outMu.Lock()
	defer s.outMu.Unlock()
	_, err = s.out.Write(raw)
	return err
}
