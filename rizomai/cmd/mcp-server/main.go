// Command mcp-server expõe o RIZOMAI como servidor MCP (Model Context Protocol)
// via stdio — o canal de AI agents (produto-zernio §5.2: MCP server hospedado,
// llms.txt, instruction-set). O agente (Claude Code, Cursor, Codex...) fala
// JSON-RPC sobre stdio; cada tool é traduzida em uma chamada HTTP à PRÓPRIA API
// (reusa auth/rate-limit/contrato — ADR-005). Zero SDK MCP externo: o protocolo
// base (initialize, tools/list, tools/call) é JSON-RPC 2.0 sobre linhas JSON.
//
// Uso:
//
//	RIZOMAI_API_URL=http://localhost:8080 RIZOMAI_API_KEY=sk_... \
//	  go run ./cmd/mcp-server --stdio
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// tool descreve uma ferramenta MCP (nome, descrição e input JSON Schema).
type tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// tools é o catálogo exposto aos agentes (espelha a API — ADR-005).
var tools = []tool{
	{
		Name:        "list_profiles",
		Description: "Lista os perfis do tenant autenticado.",
		InputSchema: obj("object", nil),
	},
	{
		Name:        "create_post",
		Description: "Cria um post com fan-out para N plataformas. platforms = [{platform, accountId}] com platform ∈ {x, linkedin, telegram, instagram, facebook, threads, youtube, tiktok, bluesky, reddit, pinterest, snapchat, googlebusiness}.",
		InputSchema: obj("object", map[string]any{
			"content":       obj("string", map[string]any{"description": "Texto do post (1..4000)"}),
			"platforms":     obj("array", map[string]any{"description": "Targets [{platform, accountId}]"}),
			"scheduledFor":  obj("string", map[string]any{"description": "Opcional: momento de publicação (RFC3339)"}),
			"timezone":      obj("string", map[string]any{"description": "Opcional: IANA, default UTC"}),
		}, []string{"content", "platforms"}),
	},
	{
		Name:        "list_posts",
		Description: "Lista posts do tenant (paginado).",
		InputSchema: obj("object", map[string]any{
			"page":  obj("integer", map[string]any{"description": "Opcional, default 1"}),
			"limit": obj("integer", map[string]any{"description": "Opcional, default 20 (máx 100)"}),
		}, nil),
	},
	{
		Name:        "get_post",
		Description: "Obtém um post pelo id, com o status de cada plataforma.",
		InputSchema: obj("object", map[string]any{
			"id": obj("string", map[string]any{"description": "post_..."}),
		}, []string{"id"}),
	},
	{
		Name:        "connect_account",
		Description: "Retorna a URL de OAuth para conectar uma conta de rede social. INSTRUA o usuário a abrir a URL no navegador e autorizar.",
		InputSchema: obj("object", map[string]any{
			"platform":  obj("string", map[string]any{"description": "x | linkedin | instagram | facebook | threads | youtube | tiktok | pinterest | snapchat | googlebusiness (telegram/bluesky/reddit usam credentials)"}),
			"profileId": obj("string", map[string]any{"description": "Opcional: profile_... de destino"}),
		}, []string{"platform"}),
	},
	{
		Name:        "get_usage",
		Description: "Uso de billing do período atual (contas conectadas vs limite do plano) e valor estimado.",
		InputSchema: obj("object", nil),
	},
}

func obj(t string, props map[string]any, required ...[]string) map[string]any {
	s := map[string]any{"type": t}
	if props != nil {
		s["properties"] = props
	}
	for _, r := range required {
		if len(r) > 0 {
			s["required"] = r
		}
	}
	return s
}

// server faz o papel de cliente da API (o agente autentica com a mesma API key).
type server struct {
	apiURL string
	apiKey string
	http   *http.Client
}

func newServer(apiURL, apiKey string) *server {
	return &server{apiURL: strings.TrimSuffix(apiURL, "/"), apiKey: apiKey, http: &http.Client{Timeout: 30 * time.Second}}
}

// handle processa uma mensagem JSON-RPC e devolve a resposta (nil para
// notificações, que não têm resposta).
func (s *server) handle(msg map[string]any) map[string]any {
	method, _ := msg["method"].(string)
	id, _ := msg["id"]

	switch method {
	case "initialize":
		return result(id, map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "rizomai-mcp", "version": "0.1.0"},
		})
	case "notifications/initialized":
		return nil
	case "ping":
		return result(id, map[string]any{})
	case "tools/list":
		out := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			out = append(out, map[string]any{
				"name": t.Name, "description": t.Description, "inputSchema": t.InputSchema,
			})
		}
		return result(id, map[string]any{"tools": out})
	case "tools/call":
		params, _ := msg["params"].(map[string]any)
		name, _ := params["name"].(string)
		args, _ := params["arguments"].(map[string]any)
		return s.callTool(id, name, args)
	default:
		return errorResult(id, -32601, "método não suportado: "+method)
	}
}

func (s *server) callTool(id any, name string, args map[string]any) map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		resp []byte
		err  error
	)
	switch name {
	case "list_profiles":
		resp, err = s.api(ctx, http.MethodGet, "/v1/profiles", nil)
	case "list_posts":
		page, _ := args["page"].(float64)
		limit, _ := args["limit"].(float64)
		q := []string{}
		if page > 0 {
			q = append(q, fmt.Sprintf("page=%d", int(page)))
		}
		if limit > 0 {
			q = append(q, fmt.Sprintf("limit=%d", int(limit)))
		}
		resp, err = s.api(ctx, http.MethodGet, "/v1/posts?"+strings.Join(q, "&"), nil)
	case "get_post":
		id, _ := args["id"].(string)
		if id == "" {
			return toolError(id, "id é obrigatório")
		}
		resp, err = s.api(ctx, http.MethodGet, "/v1/posts/"+id, nil)
	case "create_post":
		body, _ := json.Marshal(map[string]any{
			"content":      args["content"],
			"platforms":    args["platforms"],
			"scheduledFor": args["scheduledFor"],
			"timezone":     args["timezone"],
		})
		resp, err = s.api(ctx, http.MethodPost, "/v1/posts", body)
	case "connect_account":
		platform, _ := args["platform"].(string)
		if platform == "" {
			return toolError(id, "platform é obrigatório")
		}
		profileID, _ := args["profileId"].(string)
		q := ""
		if profileID != "" {
			q = "?profileId=" + profileID
		}
		resp, err = s.api(ctx, http.MethodGet, "/v1/connect/"+platform+q, nil)
	case "get_usage":
		resp, err = s.api(ctx, http.MethodGet, "/v1/billing/usage", nil)
	default:
		return errorResult(id, -32602, "ferramenta desconhecida: "+name)
	}

	if err != nil {
		return toolError(id, err.Error())
	}
	return result(id, map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(resp)}},
	})
}

// api faz uma chamada autenticada à API do RIZOMAI.
func (s *server) api(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.apiURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API indisponível (%s): %w", s.apiURL, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API %s %s → HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}

// --- JSON-RPC helpers --------------------------------------------------------

func result(id any, r map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": r}
}

func errorResult(id any, code int, message string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}}
}

func toolError(id any, message string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0", "id": id,
		"result": map[string]any{
			"content": []map[string]any{{"type": "text", "text": "erro: " + message}},
			"isError": true,
		},
	}
}

// serve lê linhas JSON-RPC do stdin e responde no stdout (transporte stdio MCP).
func serve(s *server, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	w := bufio.NewWriter(out)
	defer w.Flush()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// resposta de erro sem id (parse falhou)
			resp, _ := json.Marshal(errorResult(nil, -32700, "parse error"))
			if _, err := w.Write(append(resp, '\n')); err != nil {
				return err
			}
			_ = w.Flush()
			continue
		}
		resp := s.handle(msg)
		if resp == nil {
			continue // notificação
		}
		b, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func main() {
	log.SetPrefix("rizomai-mcp: ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	// Flag --stdio (transporte padrão do MCP).
	if len(os.Args) > 1 && os.Args[1] == "--stdio" {
		// ok
	}

	apiURL := os.Getenv("RIZOMAI_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	apiKey := os.Getenv("RIZOMAI_API_KEY")
	if apiKey == "" {
		log.Fatal("RIZOMAI_API_KEY ausente — o agente autentica com uma API key sk_... do tenant")
	}

	s := newServer(apiURL, apiKey)
	if err := serve(s, os.Stdin, os.Stdout); err != nil {
		log.Fatalf("stdio: %v", err)
	}
}
