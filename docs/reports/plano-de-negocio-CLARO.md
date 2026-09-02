# PLANO DE NEGÓCIO — CLARO

> **Projeto**: CLARO — Seu farol de confiança digital
> **Fase**: 3 — Plano de negócio + Identidade visual (PROJECT PROTOCOL §3)
> **Data**: 2026-09-02 | **Status**: PROPOSTA PARA APROVAÇÃO DO DON

---

## 1. A DOR PRINCIPAL (filtrada da pesquisa completa)

**Desconfiança digital: golpes, phishing e fraude.**

Evidência (pesquisa de mercado + GitHub, 2026-09-02):
- **346.681 issues** no GitHub mencionando spam/phishing/scam (vs. 4.743 de
  sobrecarga de notificações — proporção de 73×).
- Fraudes Pix/WhatsApp, phishing e deepfakes crescem ano a ano.
- É a dor classificada como **CRÍTICA** na matriz de oportunidade (intensidade
  crítica, frequência semanal, público universal).

**Por que esta dor?**
1. **Impacto direto e financeiro** — a pessoa perde dinheiro, não apenas tempo.
2. **Universal** — qualquer usuário de celular é alvo potencial.
3. **Urgência emocional** — medo, pânico e sensação de violação.
4. **Crescente** — a sofisticação dos golpes (IA, deepfake) só aumenta.

---

## 2. A IDEIA DIGITAL — CLARO

**Slogan**: "Verifique antes de confiar."

**O que é**: um guardião digital pessoal que, com **um toque**, diz se uma
mensagem, link, número ou chamada é confiável — antes de você agir.

### Os 3 pilares (ordem por prioridade de dor)

| Pilar | Função | Dor que resolve |
|-------|--------|-----------------|
| 🛡️ **VERIFICA** (núcleo) | Analisa links, mensagens, números e chamadas; devolve veredito: 🟢 seguro / 🟡 desconfiado / 🔴 golpe — com explicação clara | Dor 2 (golpes) — **a principal** |
| 📩 **FILTRA** | Resume e prioriza notificações e mensagens do dia (só o que importa) | Dor 1 (sobrecarga) |
| ⚡ **AUTOMATIZA** | Executa tarefas repetitivas simples (respostas padrão, confirmações) | Dor 4 (tarefas) |

### Como funciona o VERIFICA (proposta técnica)

1. O usuário **cola** um link/número/mensagem ou usa **compartilhar → CLARO**.
2. O CLARO analisa em segundos:
   - **Fonte**: reputação do domínio/remetente (base de dados de golpes conhecidos).
   - **Padrões**: heurísticas de phishing (URLs enganosas, urgência, ofertas).
   - **Comunidade**: flag de outros usuários (crowdsourcing anônimo).
   - **Contexto**: análise do texto da mensagem.
3. Devolve um **veredito + explicação em linguagem simples** (não jargão técnico):
   - "Este site foi registrado ontem e imita o banco X. Cuidado."
4. Opcional: o CLARO **bloqueia** o contato/link automaticamente.

**Diferencial competitivo**: simplicidade (1 toque), explicação clara
(por que é golpe), privacidade (não lê suas conversas — só o que você colar).

---

## 3. PLANO DE NEGÓCIO

### 3.1 Segmento de mercado

| Segmento | Prioridade | Por quê |
|----------|-----------|---------|
| **Consumidores** (B2C) — adultos 25-65, Brasil + LATAM | 🥇 Alta | Massivo, dor universal, pronto a pagar para não perder dinheiro |
| **Idosos (50+)** | 🥈 Alta | Mais vulneráveis a golpes; famílias compram proteção para eles |
| **Pequenas empresas** (B2B2C) | 🥉 Média | Donos de negócio recebem golpes de fornecedores/boletos falsos |

### 3.2 Modelo de receita — Freemium + Premium + API

| Camada | Preço | Inclui |
|--------|-------|--------|
| **Gratuito** | R$ 0 | 10 verificações/dia, veredito básico, lista de golpes conhecidos |
| **Premium** | R$ 14,90/mês (ou R$ 129/ano) | Verificações ilimitadas, explicação completa, bloqueio automático, proteção familiar (5 pessoas), modo idoso simplificado |
| **API B2B** | Sob consulta | Integração para bancos, fintechs e e-commerce validarem links/remetentes |

**Metas de receita (conservadoras)**: 0,5% conversão free→paid; meta de 100 mil
usuários no 1º ano → ~500 pagantes → ~R$ 7,5 mil/mês de MRR no ano 1;
escala para 1 milhão de usuários no ano 3 → ~R$ 75 mil/mês de MRR.

### 3.3 Go-to-market

1. **Parcerias de confiança**: bancos digitais e cooperativas (que sofrem com
   golpes Pix) oferecendo CLARO como proteção aos clientes.
2. **Conteúdo viral**: guia "os 10 golpes mais comuns de 2026" + demo do app
   (marketing de conteúdo educativo).
3. **Indicação familiar**: pacote família (quem compra para os pais conta pra todo mundo).
4. **App stores**: posicionamento por busca ("verificar golpe", "como saber se é golpe").

### 3.4 Estrutura de custos (estimada, MVP)

| Item | Custo mensal (ano 1) |
|------|----------------------|
| Infra (verificação + banco de dados) | R$ 300 |
| APIs de reputação de domínio/anti-phishing | R$ 500 |
| App stores (taxa 15-30% das assinaturas) | % variável |
| Ferramentas (desenvolvimento/hosting) | R$ 200 |
| **Total fixo estimado** | **~R$ 1.000/mês** |

Estrutura enxuta: desenvolvimento próprio (Cosca), sem escritório físico.

### 3.5 Métricas de sucesso (KPIs)

| Métrica | Meta (1º ano) |
|---------|---------------|
| Downloads | 100.000 |
| Usuários ativos (MAU) | 40.000 |
| Conversão free→paid | 0,5% |
| Verificações/usuário/mês | 30+ |
| **Golpes evitados (narrativa de impacto)** | 50.000+ |
| NPS | 50+ |

### 3.6 Riscos e mitigação

| Risco | Mitigação |
|-------|-----------|
| Base de dados de golpes desatualizada | Atualização em tempo real via fontes públicas + comunidade |
| Falso positivo (marcar site legítimo como golpe) | Veredito em níveis (nunca "100% golpe"), revisão por humanos/IA |
| Privacidade | Processar no dispositivo quando possível; política transparente |
| Concorrentes grandes (Google/banco) | Foco em simplicidade + explicação clara + pacote família |

---

## 4. IDENTIDADE VISUAL

### 4.1 Conceito

**Farol** — o que ilumina a navegação segura. Luz ciano sobre o mar escuro
(azul profundo) do mundo digital. O farol **avisa, guia e protege**.

### 4.2 Paleta

| Cor | Hex | Uso |
|-----|-----|-----|
| Azul Profundo (mar/confiança) | `#0B1E33` | Fundo principal, seriedade |
| Ciano (farol/clareza) | `#00D4FF` | Ações principais, marca |
| Verde (seguro) | `#16C784` | Veredito "seguro" |
| Âmbar (atenção) | `#F5A623` | Veredito "desconfiado" |
| Vermelho (perigo) | `#FF4757` | Veredito "golpe" |

### 4.3 Tipografia e tom

- **Voz**: clara, direta, protetora. "Você não cai nessa. A gente te ajuda."
- **Tom**: acolhedor para idosos, técnico o bastante para jovens.

### 4.4 Marca

- **Nome**: CLARO (clareza em um mundo ruidoso e perigoso).
- **Slogan**: "Verifique antes de confiar."
- **Ícone**: farol com feixe de luz — símbolo de alerta e segurança.

---

## 5. PRÓXIMOS PASSOS (após aprovação do Don)

1. **Fase 5**: definição do stack (avaliação: Go/Node backend + mobile
   cross-platform, ou PWA primeiro) — delegação ao CTO/Architecture.
2. **Fase 6**: protótipo funcional do VERIFICA (colar link → veredito).
3. **Fase 7**: validação com usuários reais (10 pessoas, cenário de golpes).
4. **Fase 8**: decisão de investimento e build completo.

---

## 6. FONTES / EVIDÊNCIA

- Estudo de dores completo: `docs/reports/estudo-dores-2026-09-01.md`
- GitHub API (2026-09-02): 346.681 issues de spam/phishing/scam; 4.743 de
  sobrecarga de notificações
- Pesquisa de mercado (Our World in Data, Pew, ITU) — ver relatório
