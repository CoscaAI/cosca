package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/store"
)

// CreatePost — POST /v1/posts.
//
// Fluxo (ADR-005 §1.3 + ADR-007):
//   1. Idempotency-Key (1ª camada): mesma key → mesma resposta em 24h
//   2. Content-hash dedup (2ª camada): mesmo (team, content+media+targets) → 409
//   3. Validação na borda (422 VALIDATION_ERROR + fields)
//   4. Transação: Post + N PostTargets atômicos
//   5. Enfileira o fan-out (Fase 2: stub síncrono; Fase 3: River)
func (h *Handlers) CreatePost(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())

	// --- 1ª camada de idempotência ----------------------------------------
	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey != "" {
		if status, body, err := h.Store.GetIdempotency(r.Context(), teamID, idemKey); err == nil {
			replayResponse(w, status, body)
			return
		}
	}

	var payload domain.PostCreatePayload
	if !decodeJSON(w, r, &payload) {
		return
	}

	// --- 2ª camada de idempotência (content-hash dedup) --------------------
	hash := postContentHash(&payload)
	if existingID, err := h.Store.GetPostIDByContentHash(r.Context(), teamID, hash); err == nil {
		respond.Error(w, http.StatusConflict, "CONFLICT", "Post duplicado nos últimos 24h",
			map[string]any{"existingPostId": existingID})
		return
	}

	// --- Validação na borda (spec é a fonte da verdade do formato) ---------
	if verr := payload.Validate(); verr != nil {
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Dados inválidos",
			map[string]any{"fields": verr.Fields})
		return
	}

	// --- Resolve o profile dono dos accounts (todos devem ser do MESMO profile)
	accountIDs := make([]string, 0, len(payload.Platforms))
	for _, t := range payload.Platforms {
		accountIDs = append(accountIDs, t.AccountID)
	}
	profileID, err := h.Store.ValidateAccountsForPost(r.Context(), teamID, accountIDs)
	switch {
	case isNotFound(err):
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Dados inválidos",
			map[string]any{"fields": map[string]string{"platforms": "alguma conta não existe ou é de outro tenant"}})
		return
	case errors.Is(err, store.ErrMultipleProfiles):
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Dados inválidos",
			map[string]any{"fields": map[string]string{"platforms": "targets devem pertencer ao MESMO profile"}})
		return
	case err != nil:
		log.Printf("validate accounts: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	// --- Monta Post + PostTargets ------------------------------------------
	now := time.Now().UTC()
	postID, err := domain.NewPostID()
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	targetStatus := domain.TargetStatusPending
	if payload.ScheduledFor != nil && payload.ScheduledFor.After(now) {
		targetStatus = domain.TargetStatusScheduled // agendado: job dispara no due time
	}
	tz := strings.TrimSpace(payload.Timezone)
	if tz == "" {
		tz = "UTC"
	}

	post := &domain.Post{
		ID:           postID,
		ProfileID:    profileID,
		Content:      payload.Content,
		MediaURLs:    payload.MediaURLs,
		ScheduledFor: payload.ScheduledFor,
		Timezone:     tz,
		Status:       domain.PostStatusScheduled,
		ContentHash:  hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	targets := make([]domain.PostTarget, 0, len(payload.Platforms))
	for _, tp := range payload.Platforms {
		tid, err := domain.NewTargetID()
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
			return
		}
		targets = append(targets, domain.PostTarget{
			ID:                   tid,
			PostID:               postID,
			Platform:             domain.Platform(tp.Platform),
			AccountID:            tp.AccountID,
			Status:               targetStatus,
			PlatformSpecificData: tp.PlatformSpecificData,
		})
	}
	post.Platforms = targets

	// --- Transação: Post + Targets atômicos (ADR-002/003) ------------------
	if err := h.Store.CreatePostWithTargets(r.Context(), post, targets); err != nil {
		log.Printf("create post: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	// --- Enfileira o fan-out (Fase 2: stub; Fase 3: River/ADR-003) ---------
	if err := h.Jobs.PublishPost(r.Context(), post); err != nil {
		// O post já foi criado — falha de enfileiramento não derruba o request;
		// em Fase 3 o job seria transacional com a criação.
		log.Printf("publish job: %v", err)
	}

	respBody := writeData(w, http.StatusCreated, post)
	if idemKey != "" {
		if err := h.Store.SaveIdempotency(r.Context(), teamID, idemKey, hash, http.StatusCreated, respBody); err != nil {
			log.Printf("save idempotency: %v", err)
		}
	}
}

// GetPost — GET /v1/posts/{id} (escopo do team; inclui targets e status).
func (h *Handlers) GetPost(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())
	id := r.PathValue("id")

	post, err := h.Store.GetPost(r.Context(), teamID, id)
	if isNotFound(err) {
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "Post não encontrado",
			map[string]any{"resource": "post", "id": id})
		return
	}
	if err != nil {
		log.Printf("get post: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	writeData(w, http.StatusOK, post)
}

// ListPosts — GET /v1/posts (paginado, escopo do team).
func (h *Handlers) ListPosts(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())
	page, limit := pageLimit(r)
	offset := (page - 1) * limit

	posts, total, err := h.Store.ListPosts(r.Context(), teamID, limit, offset)
	if err != nil {
		log.Printf("list posts: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	writeList(w, http.StatusOK, posts, page, limit, total)
}

// postContentHash normaliza (content + mediaUrls + targets ordenados) e hash
// SHA-256 — base da 2ª camada de idempotência (ADR-005 §1.3b). scheduledFor/
// timezone NÃO entram: o mesmo conteúdo publicado agora ou agendado é o mesmo post.
func postContentHash(p *domain.PostCreatePayload) string {
	h := sha256.New()
	fmt.Fprintf(h, "content:%s\n", p.Content)

	media := append([]string(nil), p.MediaURLs...)
	sort.Strings(media)
	for _, m := range media {
		fmt.Fprintf(h, "media:%s\n", m)
	}

	targets := append([]domain.PostTargetCreatePayload(nil), p.Platforms...)
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Platform == targets[j].Platform {
			return targets[i].AccountID < targets[j].AccountID
		}
		return targets[i].Platform < targets[j].Platform
	})
	for _, t := range targets {
		spd, _ := json.Marshal(t.PlatformSpecificData)
		fmt.Fprintf(h, "target:%s:%s:%s\n", t.Platform, t.AccountID, spd)
	}
	return hex.EncodeToString(h.Sum(nil))
}
