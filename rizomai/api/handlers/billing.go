// Handlers de billing (ADR-010): checkout Stripe, webhook, uso e plano.
package handlers

import (
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/billing"
	"github.com/rizomai/rizomai/internal/domain"
)

// BillingCheckout — POST /v1/billing/checkout: cria sessão Stripe (subscription)
// para o plano pedido e devolve {checkoutUrl}.
func (h *Handlers) BillingCheckout(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())

	var payload struct {
		PlanID string `json:"planId"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	if payload.PlanID == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Dados inválidos",
			map[string]any{"fields": map[string]string{"planId": "obrigatório"}})
		return
	}

	plan, err := h.Store.GetPlan(r.Context(), payload.PlanID)
	if isNotFound(err) {
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Plano não encontrado",
			map[string]any{"fields": map[string]string{"planId": "use free|growth|escala|agencia"}})
		return
	}
	if err != nil {
		log.Printf("billing checkout plan: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	checkoutURL, err := h.Stripe.CreateCheckoutSession(r.Context(), teamID, plan.Codename, plan.StripePriceID)
	if errors.Is(err, billing.ErrNotConfigured) {
		respond.Error(w, http.StatusServiceUnavailable, "STRIPE_NOT_CONFIGURED",
			"Stripe não configurado — defina STRIPE_SECRET_KEY e STRIPE_WEBHOOK_SECRET",
			map[string]any{"hint": "modo demo: conecte até o limite do plano gratuitamente"})
		return
	}
	if err != nil {
		log.Printf("stripe checkout: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Falha ao criar o checkout", map[string]any{"error": err.Error()})
		return
	}

	writeData(w, http.StatusOK, map[string]any{"checkoutUrl": checkoutURL, "planId": plan.Codename})
}

// BillingWebhook — POST /v1/billing/webhook (PÚBLICO): valida a assinatura
// Stripe e ativa o plano após checkout.session.completed.
func (h *Handlers) BillingWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Falha ao ler o corpo", nil)
		return
	}
	sig := r.Header.Get("Stripe-Signature")

	ev, err := h.Stripe.HandleWebhook(r.Context(), payload, sig)
	if errors.Is(err, billing.ErrNotConfigured) {
		respond.Error(w, http.StatusServiceUnavailable, "STRIPE_NOT_CONFIGURED", "Stripe não configurado", nil)
		return
	}
	if err != nil {
		log.Printf("stripe webhook inválido: %v", err)
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Assinatura do webhook inválida", nil)
		return
	}

	if ev.Type == "checkout.session.completed" && ev.TeamID != "" && ev.PlanID != "" {
		if err := h.Store.SetTeamPlan(r.Context(), ev.TeamID, ev.PlanID, nil); err != nil {
			log.Printf("billing webhook: ativar plano %s p/ %s: %v", ev.PlanID, ev.TeamID, err)
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"received":true}`))
}

// BillingUsage — GET /v1/billing/usage: uso do período + limite do plano
// (alimenta o dashboard: "3/3 contas (Free)" + upgrade — ADR-010 §1.2).
func (h *Handlers) BillingUsage(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())

	plan, err := h.Store.GetTeamPlan(r.Context(), teamID)
	if err != nil {
		log.Printf("billing usage plan: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	current, err := h.Store.CountActiveAccounts(r.Context(), teamID)
	if err != nil {
		log.Printf("billing usage accounts: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	periodEnd := ""
	if plan.CurrentPeriodEnd != nil {
		periodEnd = plan.CurrentPeriodEnd.Format(time.RFC3339)
	}
	usage := domain.UsageInfo{
		PlanName:         plan.Plan.Name,
		ContasConectadas: current,
		ContasLimite:     plan.Plan.ContasIncluidas,
		PeriodStart:      plan.CurrentPeriodStart.Format(time.RFC3339),
		PeriodEnd:        periodEnd,
		PeriodValorCents: planPeriodValue(plan.Plan, current),
	}
	writeData(w, http.StatusOK, usage)
}

// BillingPlan — GET /v1/billing/plan: plano atual + features.
func (h *Handlers) BillingPlan(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())

	plan, err := h.Store.GetTeamPlan(r.Context(), teamID)
	if err != nil {
		log.Printf("billing plan: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	writeData(w, http.StatusOK, plan)
}

// planPeriodValue calcula o valor mensal: fixo (agência) ou por conta
// excedente sobre as incluídas (growth/escala — ADR-010 §1.1).
func planPeriodValue(p domain.BillingPlan, contas int) int64 {
	if p.PrecoFixoCents > 0 {
		return p.PrecoFixoCents
	}
	extra := contas - p.ContasIncluidas
	if extra < 0 {
		extra = 0
	}
	return int64(extra) * p.PrecoPorContaExtraCents
}
