# Public-APIs Patterns — Catálogo de Fontes do Mundo para Capacidades

> **Category**: Agent/Mining | **Repo**: `public-apis/public-apis` (471k★) | **Owner**: Cosca Kernel | **Date**: 2026-08-27
> **Fonte**: mineração do catálogo (README 241KB / 2121 linhas) mapeado para as capacidades de um agente que vive num mundo.

## Resumo do que é

É a coletânea de APIs gratuitas mais famosa do GitHub. Uma única tabela canônica de 5 campos (`API | Description | Auth | HTTPS | CORS`) organizada em **52 categorias** (Animals, AI, Books, Business, Data, Finance, Geocoding, Weather, Music, Transportation, Science, etc.). Valor para o Cosca: é o **índice do que existe no mundo real** que um agente pode usar para perceber e agir.

## Descoberta estrutural

- Não há categoria "Space"/"Vision"/"Voice" nominais — visão está em **Machine Learning** e **Photography**; espaço/astronomia está içada em **Science & Math** (NASA, ISRO, SpaceX, Launch Library 2, Open Notify, TLE); voz/TTS em **Text Analysis**. **Padrão de colapso de capacidades em categorias amplas.**
- Os campos `Auth` e `CORS` são a bússola real (não o nome): quase todo "diamante" é `No`/`apiKey` + `CORS Yes`.

---

### [Categoria] Geocoding (65+ APIs — a maior e mais fértil)
- **O que destrava:** percepção espacial. Qualquer lugar vira coordenada, qualquer coordenada vira lugar.
- **Diamantes:** `Nominatim` (OpenStreetMap, keyless+HTPPS+CORS Yes), `Geokeo` (2500 req/dia grátis, No), `ipapi.co` (IP→localização, No), `OpenCage`/`BigDataCloud` (apiKey), `What3Words` (coordenadas em 3 palavras), `OnWater` ("terra ou mar?"), `openrouteservice.org` (direções/isócronos/elevação/POIs).
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** tool `geocode`/`locate`. Alimenta o contexto de mundo; é o par do weather ("onde estou e como está").

### [Categoria] Weather (~40 APIs)
- **O que destrava:** o estado do mundo "agora" — clima como relógio/batimento do ambiente (leve capa, encontre abrigo, vá de barco).
- **Diamantes:** `Open-Meteo` (keyless + CORS Yes — o default ideal), `Open-Meteo Ensemble` (incerteza multimodelo), `wttr.in` (JSON para terminal — perfeito p/ CLI/debug), `RainViewer` (radar), `NWS`/US Weather (keyless), `AQICN` (qualidade do ar, apiKey), `OpenUV` (índice UV).
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** tool `weather` (cli + tools). Fonte de percepção ambiental para memória ("estava chovendo quando X aconteceu").

### [Categoria] Machine Learning (~38 APIs)
- **O que destrava:** o cérebro — embeddings p/ memória semântica, LLM de fallback, e visão (image captioning/recognition).
- **Diamantes:** `Jina AI` (embeddings/rerank, apiKey), `Hugging Face` (hub + inferência), `Groq` (free tier LLM), `Google Gemini` (multimodal, apiKey), `Roboflow Universe` (visão pré-treinada), `OpenVisionAPI` (visão open-source, **No auth**), `WolframAlpha` (dados+algoritmos).
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** mapeia 1:1 para `internal/providers/` (groq/google/ollama já existem). Embeddings → `semantic-memory`. **Visão** = novo provider de imagem (Gemini/Roboflow + openvisionapi keyless como fallback). WolframAlpha = ferramenta de conhecimento factual.

### [Categoria] Text Analysis (~20 APIs)
- **O que destrava:** linguagem do mundo real — tradução, detecção de idioma, TTS (voz), sentimento, moderação.
- **Diamantes:** `LibreTranslate` (No auth, keyless), `Kiprio Translate` (50+ idiomas), `Audexum` (TTS 43 vozes/33 idiomas — a voz do agente), `Detect Language`, `Perspective`/`Tisane` (moderação de saída).
- **Nível:** 💎💎
- **Aplicação no Cosca:** tool `translate`/`tts`. Plug no provider de áudio/voz. Moderação → compliance/governance.

### [Categoria] Open Data (~55 APIs)
- **O que destrava:** conhecimento estruturado e enciclopédico — a "memória de longo prazo" externa.
- **Diamantes:** `Wikipedia` (No), `Wikidata` (**grafo de conhecimento estruturado**, OAuth — pessoa/lugar/evento), `Microlink.io` (extrai dados estruturados de qualquer URL, No+CORS Yes), `REST Countries` (No), `Socrata` (dados abertos de gov), `OpenAlex` (ciência).
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** casa natural = catálogo de conhecimento + memória semântica. `Wikipedia`+`Wikidata` são o modelo para o catálogo de conhecimento do Cosca. `Microlink` = tool "fetch-and-extract" para o agente ler o mundo.

### [Categoria] Science & Math (~43 APIs)
- **O que destrava:** o planeta como entidade viva e mensurável — astronomia, terremotos, biodiversidade.
- **Diamantes:** `NASA` (No), `USGS Earthquake Hazards` (No — **eventos do mundo como gatilhos**), `GBIF` (biodiversidade, No+CORS), `Open Notify` (posição ISS/astronautas, No), `OpenAlex`/`arXiv` (pesquisa), `Sunrise-Sunset` (ciclo dia/noite), `Newton` (cálculo simbólico).
- **Nível:** 💎💎💎 (NASA + USGS + GBIF = *Earth-as-a-world*)
- **Aplicação no Cosca:** tools `earthquake`/`iss`/`sunrise`. Conectam-se ao motor de eventos do plugin (`internal/plugins/events.go`) — terremoto/meteoro como **evento de mundo que o agente deve reagir**. NASA imagery alimenta visão/contexto de espaço.

### [Categoria] Transportation (~80 APIs)
- **O que destrava:** movimento — o mundo não está parado (aviões, trânsito, trens, barcos). Navegação A→B.
- **Diamantes:** `OpenSky Network` (ADS-B aviões em tempo real, **No keyless**), `ADS-B Exchange` (No), `GraphHopper` (roteamento turn-a-turn, apiKey), `TransitLand` (No), `Open Charge Map` (EV, apiKey+CORS).
- **Nível:** 💎💎
- **Aplicação no Cosca:** tools `airspy`/`route`. `OpenSky`+`TransitLand` keyless = percepção de mobilidade. `GraphHopper` pluga navegação no provider de mundo (Unreal).

### [Categoria] Environment (~22 APIs)
- **O que destrava:** saúde do ambiente — qualidade do ar, fogo, carbono, energia.
- **Diamantes:** `OpenAQ` (apiKey), `IQAir` (apiKey), `kanari` (incêndios em tempo real, **No+CORS**), `UK Carbon Intensity`/`Climatiq`, `GrünstromIndex` (energia verde, No+CORS).
- **Nível:** 💎💎
- **Aplicação no Cosca:** tool `air`/`airquality`. Alimenta sensores de saúde no contexto do agente.

### [Categoria] Social (~55 APIs)
- **O que destrava:** agência — o agente pode ler/posticar/responder num corpo próprio nas redes. Ação real.
- **Diamantes:** `Bluesky` (protocolo AT, **No keyless+CORS**), `HackerNews` (No), `Telegram Bot` (apiKey), `Discord` (OAuth).
- **Nível:** 💎💎
- **Aplicação no Cosca:** o "corpo social" do agente. `Bluesky` keyless é o mais plugável. Conecta-se a `internal/plugins/hooks.go`/`events.go`.

### [Categoria] Games & Comics (~105 APIs) — para viver num mundo de jogo
- **O que destrava:** o Cosca vive num mundo (Unreal) — dados de criaturas/objetos/itens = ontologia de mundo jogável.
- **Diamantes:** `Pokéapi` (No), `Mojang`/`Minecraft Server Status` (No), `DnD 5e`/`Open5e` (No), `RAWG.io` (500k jogos), `SpaceTradersAPI` (MMO espacial, OAuth), `PokéSprite`.
- **Nível:** 💎 (marginal agora; 💎💎 se o Cosca almeja *entender* mundos de jogos).
- **Aplicação no Cosca:** bases de itens/sprites enriquecem o *conhecimento de entidade* do agente.

### [Categoria] Photography / News / Books / Dictionaries
- **Photography:** `Pexels` (apiKey+CORS), `Unsplash`, `Lorem Picsum` (No keyless placeholders), `Remove.bg`/`ObjectCut` (remover fundo). 💎💎.
- **News:** `NewsAPI`/`GNews`/`NYT` — conhecimento temporal. 💎💎 (consciência do presente).
- **Books:** `Open Library` (No), `Gutendex` (No), `Google Books` (OAuth). 💎💎.
- **Dictionaries:** conhecimento linguístico. 💎.

---

## 🏆 DIAMANTES (para o Cosca) — e POR QUÊ

1. **Open-Meteo (Weather)** — keyless + HTTPS + CORS Yes, previsão global. Sem fricção p/ o MVP. Percepção do "agora".
2. **Nominatim (Geocoding)** — keyless + CORS Yes, forward/reverse sobre OSM. O par do Weather: local + clima = "onde estou e como está".
3. **Jina AI + Hugging Face (ML)** — embeddings/rerank + inferência. Plugam direto no semantic-memory/RAG via `providers/openaicompat/embeddings`.
4. **Wikipedia + Wikidata (Open Data)** — conhecimento enciclopédico + **grafo estruturado**. Wikidata é o template de "ficha de entidade".
5. **Groq + Google Gemini (ML)** — LLM free-tier + multimodal. Cérebro com **visão** (Gemini) e fallback de baixo custo ao lado do Ollama local.
6. **USGS Earthquake + Open Notify + NASA (Science)** — keyless, eventos do planeta em tempo real = **gatilhos de mundo** conectáveis a `internal/plugins/events.go`.
7. **OpenSky Network (Transportation)** — keyless, aviões em tempo real. Dá "mundo-em-movimento".
8. **OpenAQ / IQAir + wttr.in (Environment/Weather)** — qualidade do ar + terminal-JSON keyless p/ debug da CLI.
9. **APILayer suite** — **uma conta, uma chave, ~10 capacidades**: geocoding (`positionstack`), weather (`weatherstack`), news (`mediastack`), stocks (`marketstack`), email, IP (`ipstack`). O argumento mais forte para começar a integração.

## 🧭 LEITURA DE PADRÃO — como inspirar o catálogo do Cosca

1. **Formato tabular canônico de 5 campos** = taxonomia mínima de integração. Molde de um struct Go:
```go
type Integration struct {
    Name      string // "Open-Meteo"
    Category  string // "Environment" / "world"
    Auth      string // "none" | "apiKey" | "oauth"
    HTTPS     bool
    CORS      bool   // browser-safe p/ Wails/React desktop
    Endpoints []Endpoint
    Level     int    // 💎 rating p/ o agente
}
```
2. **Auth + CORS são a verdadeira bússola, não o nome.** Priorize e faça ranked por `Auth == none` → `apiKey` → `OAuth`; e `CORS Yes` para endpoints chamáveis direto do desktop.
3. **Não copiar a fragmentação por domínio.** O Cosca deve construir uma **taxonomia por capacidade do agente** (`perception`, `knowledge`, `action`, `spatial`, `language`, `world-events`), que é mais acionável.
4. **Curadoria acompanha a evolução do mundo.** Categorias com demanda massiva (geo/transporte/games) inflaram; categorias "novas/riche" (environment/visão/AI) têm os keys mais frescos.
5. **Registry versionado + validador.** Se o Cosca criar o seu catálogo de integrações, deve imitar o `CONTRIBUTING.md`/scripts: registro central com schema versionado + validador de formato (como o `slopguard`). O `internal/integrations/` (que hoje só existe como SKILL.md conceitual) deveria se materializar como pacote Go.

## ⚠️ NOTA DE LACUNA NO COSCA

O Cosca **não tem** um `internal/integrations/` como pacote Go — só há a SKILL.md conceitual em `internal/embed/cosca/departments/integrations/`. E não tem **"World Data-Source Discovery"**: o `CAPABILITY_CATALOG.md` e `internal/skills/catalog.go` só conhecem capacidades internas e skills de GitHub, não fontes de dados do mundo. Ver `world-acquisition-relations.md` para o mapa completo.
