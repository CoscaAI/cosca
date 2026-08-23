# AI Product Patterns — Inteligência de Produto (34 produtos/empresas)

> **Version**: 1.0.0 | **Confidence**: 0.70 | **Category**: AI Agent Patterns | **Created**: 2026-08-23 | **Source**: Lista do Don (produtos/empresas de IA)

> **Mined by**: cosca-kernel (ordem do Don). **Nota honesta:** a maioria é SaaS **fechado** — a busca no GitHub só achou wrappers/SDKs de terceiros (exceção: `Leonardo-Interactive/leonardo-ts-sdk`). Não há código de primeira-partes para clonar na maior parte. Este doc é **inteligência de produto/design**: para cada um, o **padrão que prova** e a **lição para o Cosca**. Category: AI Agent Patterns (síntese).

## Purpose

Sintetizar o que cada produto/empresa de IA da lista prova em termos de **padrão de produto** e **lições aplicáveis ao Cosca** (plataforma de orquestração de agentes, RAG, simulação/geração). Agrupei por capacidade.

---

## A. Modelos & Providers (a camada de fundação)

### A1. Mistral AI — provider de modelo aberto pragmatico (Mixtral, ecosistema open-weight)
- **O que resolve**: modelos open-weight competitivos com os fechados, com licença permissiva e foco em eficiência/custo (MoE).
- **Padrão**: **MoE (Mixture-of-Experts) + open-weight** — esparsidade para custo, pesos abertos para auto-hostagem/privacidade.
- **Lição Cosca**: `cosca-provider` deve suportar **open-weight local** (não só API) — o "cofre" (local-first) do Cosca pede um provider open-weight como opção primária; a abstração de provider já existe, falta priorizar o caminho local.

### A2. Grok AI (xAI) — modelo de consumo em tempo real (alimentado por dados X/Twitter)
- **O que resolve**: modelo consumer com conhecimento atualíssimo via feed social; integração ao produto de rede.
- **Padrão**: **LLM "de plataforma"** — modelo treinado com o próprio sinal de dados do produto; streaming/real-time.
- **Lição Cosca**: o Cosca deve considerar **treinar/ajustar com o próprio Knowledge Base** (o que já tem no `internal/embed/cosca` + `knowledge.db`) — o fine-tune LoRA no próprio conhecimento (não só RAG).

### A3. Pi AI (Inflection) — LLM empático/conversacional pessoal
- **O que resolve**: conversa emocional/empática, tonalidade, presença.
- **Padrão**: **persona com voz** (não só função) — tom, empatia, memória de relacionamento.
- **Lição Cosca**: o `cosca-kernel`/agentes deveriam ter **persona/voz declarada** (o SOUL.md do Mega Brain) — não só instrução funcional. O Don já tem identidade; formalizar a *voz* nos agentes.

---

## B. Geração de Mídia (imagem/vídeo/áudio/design)

### B1. Midjourney — geração de imagem por prompt (community, estética, iteração)
- **O que resolve**: imagem de alta estética a partir de texto; comunidade/remoção de "text-to-image cru".
- **Padrão**: **fator de estética + iteração por prompt** (o que roda o usuário é o "conceito" → vária para "estética").
- **Lição Cosca**: `cosca-ai`/`cosca-performance` deve incluir um **bloco de geração de mídia** (imagem/design) com invocação por prompt e **revisão estética**, não só análise — o Cosca hoje é forte em análise, fraco em síntese criativa.

### B2. Ideogram — geração de imagem com **texto/tipografia legível**
- **O que resolve**: texto dentro de imagem corretamente renderizado (tipografia), o gap histórico do image-gen.
- **Padrão**: **modalidade especializada** — dedicar um modelo a um sub-problema (tipografia) em vez de one-size.
- **Lição Cosca**: **modelos/especialistas dedicados** para sub-tarefas (o Cosca tem especialistas; mapear para "especialista de tipografia/mídia" se for gerar material).

### B3. Recraft AI — design/vector (ilustração, SVG, art board)
- **O que resolve**: geração vetorial/vector (SVG) editável, não só raster.
- **Padrão**: **saída editável/estruturada** (vector) em vez de pixel — produz assets que o usuário consegue editar.
- **Lição Cosca**: quando o Cosca gera output criativo, deve gerar **artefato editável/estruturado** (não só imagem final), integrando ao pipeline de design.

### B4. Leonardo AI — geração de imagem/asset para game/estética (control, fine-tune)
- **O que resolve**: controle fino + fine-tune de estilo, assets para game.
- **Padrão**: **fine-tune de estilo + controles estruturais** (pose, inpainting).
- **Lição Cosca**: suportar **fine-tune no estilo do projeto** + controles — o Cosca tem o pipeline de treino LoRA (do commit `eb73971`); integrar.

### B5. Magnific AI — **upscale/resolução** (ampliar com inferência de detalhe)
- **O que resolve**: upscale de imagem com "inferência de detalhe" (não só interpolação).
- **Padrão**: **modelo de pós-processamento** que adiciona detalhe (super-resolution generativa).
- **Lição Cosca**: o pipeline do Cosca deve ter **estágios de pós-processamento** (enhance) — não só geração primária.

### B6. Runway AI — **vídeo generativo** + ferramentas de criação (Gen-3, motion)
- **O que resolve**: vídeo a partir de texto/imagem, com controle de motion.
- **Padrão**: **modelo temporal** (vídeo) + suíte de ferramentas de edição.
- **Lição Cosca**: o Cosca vai além de texto — se entrar em **mídia temporal**, precisa de abstração de vídeo (frames, motion, scene-graph), que o `cosca-runtime`/`media` já sinalizam. Referência para `internal/media`.

### B7. Luma AI — vídeo 3D/neural (Dream Machine, 3D/NeRF)
- **O que resolve**: geração de vídeo/3D a partir de imagens (NeRF/3D).
- **Padrão**: **3D/NeRF** — reconstrução de cena a partir de vistas.
- **Lição Cosca**: o `scene-graph`/`procgen` do Cosca (docs/architecture/scene-graph.md) — a Luma prova a evolução p/ **reconstrução 3D** como capacidade; relevante se o Cosca gerar cenas.

### B8. Synthasia AI — **avatares de vídeo** (talking-head, lipsync, avatar corporativo)
- **O que resolve**: vídeo de avatar falando (lipsync) com roteiro.
- **Padrão**: **avatar dublado** — síntese de voz + imagem em sincronia; pipeline "texto → roteiro → avatar".
- **Lição Cosca**: para o Cosca **apresentar saídas** (vídeo-narrativa de relatórios/insights), o padrão "texto → roteiro → avatar/vídeo" é um caminho de produto. O `cosca-documentation`/`output` pode gerar mídia narrativa.

### B9. HeyGen AI — **avatar/vídeo** (língua, lipsync, tradução)
- **O que resolve**: avatar + tradução de vídeo (lipsync em outra língua).
- **Padrão**: **tradução dublada** (mantém o vídeo, troca a voz/língua).
- **Lição Cosca**: **localização de conteúdo** — o Cosca gera docs em PT; a HeyGen prova a "tradução preservando mídia". Lição de i18n cross-mídia.

### B10. Suno AI — **música generativa** (áudio, letra + música)
- **O que resolve**: música completa (letra + melodia) a partir de prompt.
- **Padrão**: **geração de áudio/música** como modalidade de primeira classe.
- **Lição Cosca**: o `cosca-ai` deve reconhecer **áudio/música** como modalidade se o produto for multimídia.

### B11. Imgs.ai — geração de imagem/design (marketplace de prompts)
- **O que resolve**: acesso a geração de imagem/AI com curadoria de prompts.
- **Padrão**: **camada de curadoria** (prompts pré-feitos) para não-especialistas.
- **Lição Cosca**: **templates/curadoria** para o usuário — o Cosca deve ter biblioteca de prompts/presets (o `skills.sh.json`/catálogo de skills).

### B12. Free AI (free-ai) — agregador de ferramentas de IA gratuitas
- **O que resolve**: hub de ferramentas de IA gratuitas.
- **Padrão**: **agregação descoberta** (catálogo de capacidades).
- **Lição Cosca**: o Cosca já tem o gate de catálogo/capability inventory — a Free AI prova o valor de um **hub de descoberta de capacidades** para o usuário.

---

## C. Escrita & Linguagem

### C1. Copy.ai — copywriting por IA (marketing)
- **O que resolve**: copy de marketing (anúncio, email, landing) a partir de brief.
- **Padrão**: **geração de copy por templates/tone-of-voice** — não texto livre, mas direcionado por marca.
- **Lição Cosca**: **modelo de geração de conteúdo** com tone-of-voice presets por marca (o Cosca poderia ter persona de escrita).

### C2. Jasper AI — marketing/escrita de marca (brand voice)
- **O que resolve**: conteúdo de marca com voice consistente (diferente de copy genérico).
- **Padrão**: **memória de marca/voice** (persistir o estilo da marca entre gerações).
- **Lição Cosca**: o Cosca deveria ter **voice-preserving memory** (estilo persistente) — liga ao DNA da Mega Brain (voice_dna).

### C3. QuillBot — **paráfrase/resumo/gramática** (reescrever)
- **O que resolve**: reescrever texto (paraphrase, summarize, grammar).
- **Padrão**: **transformação de texto** como serviço (rewrite) — não só geração.
- **Lição Cosca**: o Cosca precisa de **operadores de transformação de texto** (resumir, reescrever, condensar) — o `context_compactor`/condensação já existe; formalizar como tool.

### C4. Rephrase AI — reescrita/variação de texto (social, marketing)
- **O que resolve**: variação de texto (ler como outra pessoa, variações).
- **Padrão**: **paráfrase com angulação** (mudar o "persona" da escrita).
- **Lição Cosca**: transformação de texto por angulação/persona — mapear a tools do Cosca.

### C5. MarketMuse — **SEO/otimização de conteúdo** (insights de topic)
- **O que resolve**: SEO com dados — topic coverage, content gaps.
- **Padrão**: **análise de conteúdo orientada a dados** (não só geração) — gap analysis.
- **Lição Cosca**: o Cosca pode fazer **content-gap analysis** (o que falta cobrir) usando o knowledge base — uma feature de `cosca-analytics`.

### C6. Speechify — **texto-para-fala** (leitura em voz alta, velocidade)
- **O que resolve**: TTS de alta qualidade, velocidade, documentos em áudio.
- **Padrão**: **TTS acessível** (ler qualquer documento).
- **Lição Cosca**: o Cosca tem voice (TTS?) — o Speechify prova o **acesso por áudio** a conteúdo; liga ao `cosca-ai`/media.

---

## D. Reuniões & Transcrição

### D1. Fireflies AI — **transcrição de reuniões + notas + RAG** (grava, transcreve, resume, ações)
- **O que resolve**: captura de reunião → transcrição → notas/insights/ações, integrado ao calendário.
- **Padrão**: **pipeline de captura→transcrição→extração→ação** (RAG sobre a reunião), com quem-falou e métricas.
- **Lição Cosca**: o Cosca tem `voice_diarizer`/`speaker_labeler`/`email/insights` (do Mega Brain pipeline). O padrão Fireflies = **meeting intelligence** — uma feature de produto forte para o Cosca (gravar/diarizar/extrair do que foi falado). Ligado a `internal/messaging`/`voice`.

### D2. Krisp AI — **noise cancellation + transcrição** (áudio limpo)
- **O que resolve**: remoção de ruído em tempo real (melhora o áudio antes da transcrição).
- **Padrão**: **pré-processamento de áudio** (clean) antes de qualquer downstream (transcrição).
- **Lição Cosca**: **limpar o input antes de processar** — o Cosca deve ter um estágio de "higiene de áudio" se consumir voz. Padrão genérico: *clean-input-before-process*.

### D3. Vidyo AI — **clipes de vídeo de reuniões** (destaque, social clips)
- **O que resolve**: extrair clipes virais de reuniões longas (highlight, shorts).
- **Padrão**: **destaque de conteúdo** — achar os momentos importantes de mídia longa.
- **Lição Cosca**: **sumarização de destaque** (não só resumo de texto) — extração de momentos-chave de mídia. Ligado ao `cosca-media`/`performance` (destaque).

---

## E. Produtividade & Design

### E1. Gamma — **decks/apresentações** a partir de texto (docs→slides)
- **O que resolve**: transformar conteúdo em apresentação (slides) com design auto.
- **Padrão**: **texto → estrutura visual** (gerar deck a partir de um brief/conteúdo).
- **Lição Cosca**: o `cosca-documentation`/`uiux` poderia ter **text→deck** — uma feature de síntese visual. Ligado ao que o Don faz (apresentar).

### E2. SlidesAI — slides/decks a partir de texto (Google Slides)
- **O que resolve**: slides a partir de prompt/tópicos.
- **Padrão**: **decks a partir de tópicos** (em vez de texto completo).
- **Lição Cosca**: entrada de **tópicos** (não texto) para geração — o Cosca deveria aceitar ambos (brief estruturado).

### E3. Uizard — **design de UI a partir de prompt** (wireframe, mockup)
- **O que resolve**: gerar wireframes/designs de UI a partir de descrição.
- **Padrão**: **design-to-code/diagram** — de prompt a mockup.
- **Lição Cosca**: o `cosca-frontend`/`uiux` pode usar **prompt→design** como capacidade; liga ao template/generação de UI.

### E4. Looka — **branding/logo** (logo + identidade visual)
- **O que resolve**: gerar logo/identidade visual.
- **Padrão**: **geração de identidade** (a partir de nome/indústria).
- **Lição Cosca**: se o Cosca gerar produtos, ter **brand identity generation** — uma feature de iniciação de projeto (o `project-init`).

### E5. Durable AI — **site/negócio em segundos** (gerar site completo)
- **O que resolve**: gerar um site de negócio completo (landing) em segundos.
- **Padrão**: **gerar um produto completo** (não uma peça) a partir de um prompt — "one-shot site" com estrutura, copy, design.
- **Lição Cosca**: o Cosca pode **gerar um artefato completo** (site/campaign) de ponta a ponta — o `cosca-project`/template. O padrão "gerar o todo" em vez de "uma página".

### E6. Taskade AI — **productivity/agentes colaborativos** (tarefa, mindmap)
- **O que resolve**: workspace de produtividade com agentes/AI (tarefas, mapas).
- **Padrão**: **agentes de produtividade integrados** a uma ferramenta de trabalho (task/mindmap).
- **Lição Cosca**: **AI dentro do fluxo de trabalho** (o Don trabalha no Cosca; a AI deve estar embed na ferramenta, não separada). Ligado ao `cosca-cli`/`desktop`.

---

## F. Consumer & Serviços

### F1. DoNotPay AI — **"advogado robô"** (contestar ação, multas, assinaturas)
- **O que resolve**: automatizar assuntos legais/burocráticos do consumidor.
- **Padrão**: **agente de "ação"** (não só informação) — o produto gera a comunicação/ação real (contestar, cancelar).
- **Lição Cosca**: o Cosca pode ser **agentic action** (não só análise) — executar a ação no mundo (gerar/assinar documentos). Mas cuidado: legal = **compliance** (o `cosca-compliance`).

### F2. AdCreative AI — **criativos de anúncio** (ads) gerados
- **O que resolve**: gerar criativos de anúncio (ad) em massa (variantes).
- **Padrão**: **geração em massa de variantes** (A/B de criativos) — mesma ideia, N variações.
- **Lição Cosca**: **variância controlada** (gerar N variações de um output) — liga ao meta-loop A/B (o Cosca gera + avalia variantes).

### F3. InVideo AI — **vídeo a partir de texto** (script→vídeo)
- **O que resolve**: vídeo a partir de script (edicão automática com stock).
- **Padrão**: **texto→vídeo completo** (com b-roll, transições) — pipeline de montagem.
- **Lição Cosca**: **síntese de mídia completa** (montagem) a partir de texto — o `cosca-media`/`output` poderia ter esse pipeline (o Don trabalha com vídeo).

### F4. OpusClip — **clipes de vídeo** (longo→shorts virais)
- **O que resolve**: cortar vídeo longo em clipes virais com legendas.
- **Padrão**: **redistribuição de conteúdo** — um vídeo longo vira N shorts (multi-platform).
- **Lição Cosca**: **repurposing de conteúdo** (transformar 1 artefato em N formatos/canais) — o Cosca poderia "publicar" em múltiplos formatos. Ligado a `cosca-messaging`/`media`.

---

## G. Agentes & Som

### G1. Claude Artifacts — **artefatos interativos gerados pelo agente** (código/UI em runtime)
- **O que resolve**: o agente gera um artefato executável/visual (app, diagrama, HTML) lado a lado na conversa.
- **Padrão**: **artefato-vivo no contexto** — o resultado do agente vira um objeto interativo que o usuário edita/roda (não só texto). É o padrão "agent outputs objects, not just text".
- **Lição Cosca**: **saída como objeto** (não só texto) — o Cosca deveria gerar artefatos interativos (o `cosca-desktop`/Wails; o output de um agente como sandbox interativo). Direto pro `cosca-ui`/`runtime`.

### G2. Mistral (A1 repeat) — já coberto.

---

## H. Síntese — os padrões transversais que valem pro Cosca

| # | Padrão | Produtos que provam | Aplicação no Cosca |
|---|--------|---------------------|--------------------|
| 1 | **Saída como objeto vivo** (não só texto) | Claude Artifacts, Gamma, Durable, Uizard | `cosca-ui`/`desktop`: agent gera artefato interativo |
| 2 | **Texto → mídia completa** (pipeline de montagem) | Suno, InVideo, Synthesia, Heygen, Recraft | `cosca-ai`/`media`: síntese criativa, não só análise |
| 3 | **Capture → transcribe → extract → act** (meeting intelligence) | Fireflies, Krisp, OpusClub | `cosca-voice`/`messaging`: do áudio à ação |
| 4 | **Repurposing de conteúdo** (1 artefato → N formatos) | OpusClip, Vidyo, AdCreative | `cosca-media`/`output`: publicar multi-formato |
| 5 | **Variância controlada + avaliação** | AdCreative, Jasper, Copy.ai | meta-loop A/B já existente; ampliar p/ criativos |
| 6 | **Voice/persona persistente** | Jasper, Pi, Copy.ai | DNA voice (Mega Brain) como memória de estilo |
| 7 | **Fine-tune no próprio conhecimento** | Mistral open-weight, Leonardo, Magnific | LoRA no knowledge base (não só RAG) |
| 8 | **Agente de ação (não só informação)** | DoNotPay, Durable | agentic action com gate de compliance |

## Known Uses (referência) / Honestidade

- **Fechado (sem código clonável):** Midjourney, Ideogram, Runway, OpusClub, Recraft, Durable, DoNotPay, Krisp, SlidesAI, Pi, Heygen, Luma, Fireflies, Gamma, Vidyo, Magnific, Grok, Leonardo, Synthasia, Taskade, AdCreative, InVideo, Copy.ai, Rephrase, Suno, Uizard, Jasper, Looka, Imgs.ai, MarketMuse, QuillBot, Speechify, Free AI.
- **Open (só SDK/wrapper de terceiros):** Leonardo (`Leonardo-Interactive/leonardo-ts-sdk`), Mistral (SDKs em `mistralai`), Suno (`suno-api` wrapper), Fireflies (wrapper).

> **Nota de método:** por serem produtos SaaS fechados, este doc é **síntese de padrão de produto** (o que cada um prova + lição para o Cosca), NÃO mineração de código. Para padrões de código reais, o caderno tem deepseek/ruflo/mega-brain/kubernetes/etc.

## Related Patterns

- [`mega-brain-patterns.md`](mega-brain-patterns.md) — DNA cognitivo + voz + meeting intelligence (overlap grande)
- [`ruflo-patterns.md`](ruflo-patterns.md) — capability inventory + memória
- [`google-agent-patterns.md`](google-agent-patterns.md) — agent artifact/interactive
