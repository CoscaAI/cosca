package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/acquisition"
	"github.com/CoscaAI/cosca/internal/knowledge"
)

func main() {
	home, _ := os.UserHomeDir()
	kbDir := filepath.Join(home, ".config", "cosca")

	gStore, err := knowledge.NewGlobalPackageStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "global store: %v\n", err)
		os.Exit(1)
	}

	// All npm packages to register + acquire
	pkgs := []struct{ id, repo string }{
		{"bcryptjs", "dcodeIO/bcrypt.js"},
		{"bull", "OptimalBits/bull"},
		{"cache-manager", "jaredwray/cache-manager"},
		{"class-transformer", "typestack/class-transformer"},
		{"class-validator", "typestack/class-validator"},
		{"cli", "nestjs/nest-cli"},
		{"client", "prisma/prisma"},
		{"common", "nestjs/nest"},
		{"compression", "expressjs/compression"},
		{"config", "nestjs/nest"},
		{"core", "nestjs/nest"},
		{"eslint", "eslint/eslint"},
		{"eslint-config-prettier", "prettier/eslint-config-prettier"},
		{"eslint-plugin-prettier", "prettier/eslint-plugin-prettier"},
		{"eslintrc", "eslint/eslint"},
		{"express", "expressjs/express"},
		{"globals", "sindresorhus/globals"},
		{"helmet", "helmetjs/helmet"},
		{"jest", "jestjs/jest"},
		{"js", "eslint/eslint"},
		{"jwt", "nestjs/jwt"},
		{"multer", "expressjs/multer"},
		{"nest-winston", "gremo/nest-winston"},
		{"node", "DefinitelyTyped/DefinitelyTyped"},
		{"passport", "jaredhanson/passport"},
		{"passport-jwt", "mikenicholson/passport-jwt"},
		{"passport-local", "jaredhanson/passport-local"},
		{"platform-express", "nestjs/nest"},
		{"prettier", "prettier/prettier"},
		{"prisma", "prisma/prisma"},
		{"reflect-metadata", "rbuckton/reflect-metadata"},
		{"rxjs", "ReactiveX/rxjs"},
		{"schedule", "nestjs/nest"},
		{"schematics", "nestjs/schematics"},
		{"source-map-support", "evanw/node-source-map-support"},
		{"supertest", "ladjs/supertest"},
		{"swagger", "nestjs/swagger"},
		{"swagger-ui-express", "scottie1984/swagger-ui-express"},
		{"testing", "nestjs/nest"},
		{"throttler", "nestjs/throttler"},
		{"ts-jest", "kulshekhar/ts-jest"},
		{"ts-loader", "TypeStrong/ts-loader"},
		{"ts-node", "TypeStrong/ts-node"},
		{"tsconfig-paths", "dividab/tsconfig-paths"},
		{"typescript", "microsoft/TypeScript"},
		{"typescript-eslint", "typescript-eslint/typescript-eslint"},
		{"uuid", "uuidjs/uuid"},
		{"winston", "winstonjs/winston"},
	}

	// Sort for predictable order
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].id < pkgs[j].id })

	ctx := context.Background()
	reg := 0
	acq := 0
	skip := 0
	fail := 0

	for _, e := range pkgs {
		// Register in JSON store
		if !gStore.Exists(e.id) {
			pkg := knowledge.KnowledgePackage{
				ID:         e.id,
				Kind:       "library",
				Ecosystem:  "typescript",
				Versions:   []string{"*"},
				Sources:    []string{"official-docs", "official-repository"},
				Repository: e.repo,
			}
			if aErr := gStore.Add(pkg); aErr != nil {
				fmt.Printf("[REG-FAIL] %s — %v\n", e.id, aErr)
				fail++
				continue
			}
			reg++
		} else {
			p, _ := gStore.Get(e.id)
			if p != nil && p.Status == knowledge.PackageStatusAcquired {
				fmt.Printf("[SKIP] %s — already acquired\n", e.id)
				skip++
				continue
			}
			// Update repo if needed
			if p != nil && p.Repository == "" {
				p.Repository = e.repo
				gStore.Add(*p)
			}
		}

		// Acquire
		fmt.Printf("[ACQUIRE] %s → %s\n", e.id, e.repo)
		url := "https://api.github.com/repos/" + e.repo + "/readme"
		client := acquisition.NewClient(30*time.Second, 3, 50<<20)
		client.AllowRemote = true
		client.GitHubToken = os.Getenv("GITHUB_TOKEN")

		art, body, fErr := client.FetchAll(ctx, url)
		if fErr != nil {
			url = url + "?ref=master"
			art, body, fErr = client.FetchAll(ctx, url)
			if fErr != nil {
				fmt.Printf("  [FAIL] %s — %v\n", e.id, fErr)
				fail++
				continue
			}
		}

		var resp struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		if jErr := json.Unmarshal(body, &resp); jErr == nil && resp.Encoding == "base64" && resp.Content != "" {
			clean := strings.ReplaceAll(resp.Content, "\n", "")
			if dec, dErr := base64.StdEncoding.DecodeString(clean); dErr == nil {
				body = dec
			}
		}

		// Store artifact
		astore := acquisition.NewArtifactStore(kbDir)
		artID, _ := astore.Add(art, body)

		// Write markdown
		dir := filepath.Join(kbDir, "knowledge", "acquired", e.id)
		os.MkdirAll(dir, 0700)
		mdPath := filepath.Join(dir, "docs.md")
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("title: %q\n", e.id+" — Official Documentation"))
		sb.WriteString(fmt.Sprintf("tags: [%q, %q, %q, %q]\n", e.id, "typescript", "acquired", "documentation"))
		sb.WriteString(fmt.Sprintf("source: %q\n", url))
		sb.WriteString(fmt.Sprintf("acquired_at: %q\n", time.Now().UTC().Format(time.RFC3339)))
		sb.WriteString("confidence: 0.85\n")
		sb.WriteString("---\n\n")
		sb.WriteString(fmt.Sprintf("# %s — Official Documentation\n\n", e.id))
		sb.WriteString(fmt.Sprintf("> Source: %s\n\n", url))
		const maxBody = 500 << 10
		if len(body) > maxBody {
			body = body[:maxBody]
		}
		sb.Write(body)
		os.WriteFile(mdPath, []byte(sb.String()), 0600)

		// Update manifest
		p, _ := gStore.Get(e.id)
		if p != nil {
			p.Status = knowledge.PackageStatusAcquired
			p.KnowledgeLevel = "partial"
			p.AcquiredAt = time.Now()
			p.SHA256 = art.SHA256
			p.ArtifactIDs = append(p.ArtifactIDs, artID)
			gStore.Add(*p)
		}

		fmt.Printf("  [OK] %s — %d KiB\n", e.id, len(body)>>10)
		acq++
	}

	fmt.Printf("\n=== RESUMO ===\n")
	fmt.Printf("Registrados JSON: %d | Adquiridos: %d | Skip: %d | Fail: %d\n", reg, acq, skip, fail)
}
