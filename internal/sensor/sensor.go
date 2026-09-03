// Package sensor — DTO sensorial normalizado (DOGMA percepção-como-evidência).
//
// Este é o "DNA" que todo sensor emite. Um sensor (OCR, CLIP, tracking, web,
// STT) NÃO entrega "verdade" — entrega EVIDÊNCIA com um perfil de erro
// conhecido. O kernel decide o que confiar, como combinar e quando escalar.
//
// Contrato do DTO (todas as peças carregam a MESMA forma):
//
//	{tipo, conteúdo, confiança, fonte, estilo_epistêmico, timestamp, trace_id}
//
// Receita da arquitetura (professor+Don, 2026-09-02):
//   1. DTO sensorial normalizado  (este pacote).
//   2. Fusão + detecção de contradição.
//   3. Gate de escalação (VLM no topo, nunca na base).
//   4. Ledger epistêmico (Estado do Mundo).
//
// Este pacote é apenas a peça (1) — a fundação. As outras três consomem
// Evidência e não criam outra forma de transporte.
package sensor

import "time"

// EpistemicState é a classe epistêmica de uma observação. Espelha a convenção
// já usada em todo o Cosca (knowledge/adapter/engine): um rótulo que diz a
// NATUREZA da evidência, não o seu conteúdo. Um item INFERRED nunca é lido
// como fato.
type EpistemicState string

const (
	// EpistemicMEASURED é uma observação medida diretamente pelo sensor
	// (ex: OCR leu "Punta Cana" na imagem). A mais forte das observações.
	EpistemicMEASURED EpistemicState = "MEASURED"
	// EpistemicINFERRED é derivada de outras evidências (ex: região de texto
	// detectada mas não lida; objeto acompanhado por tracking). Não é fato.
	EpistemicINFERRED EpistemicState = "INFERRED"
	// EpistemicEVIDENCE é uma evidência corroborada por múltiplos sensores
	// ou fontes independentes (ex: OCR + CLIP concordam).
	EpistemicEVIDENCE EpistemicState = "EVIDENCE"
	// EpistemicDECISION é uma conclusão tomada pelo kernel (não observada).
	EpistemicDECISION EpistemicState = "DECISION"
)

// Modality é o canal sensorial de uma observação. Cada sensor produz um
// pacote com Modality — o kernel roteia e compara pelo canal, não pelo motor.
type Modality string

const (
	// ModalityVisual é o canal visual (imagem, CLIP, detector de regiões).
	ModalityVisual Modality = "visual"
	// ModalityText é o canal de texto (OCR).
	ModalityText Modality = "text"
	// ModalityAudio é o canal de áudio (STT).
	ModalityAudio Modality = "audio"
	// ModalityCode é o canal de código (análise estática, AST).
	ModalityCode Modality = "code"
	// ModalityWeb é o canal de dados externos (busca, notícia, github).
	ModalityWeb Modality = "web"
	// ModalityState é o canal de estado interno (filesystem, runtime).
	ModalityState Modality = "state"
)

// Region localiza uma observação no espaço (ex: região da tela onde o OCR leu
// um texto; o objeto rastreado no frame 847). É opcional — nem toda evidência
// é espacial. Quando presente, é em coordenadas absolutas do frame/contexto.
type Region struct {
	// X/Y é o canto superior esquerdo (posição absoluta).
	X int `json:"x"`
	Y int `json:"y"`
	// W/H é o tamanho.
	W int `json:"w"`
	H int `json:"h"`
	// Kind é um rótulo opcional do tipo de região (text/graphic/object).
	Kind string `json:"kind,omitempty"`
}

// Confidence é a confiança da observação em [0,1]. É o número que o kernel
// usa para decidir. Cada sensor declara a própria confiança; o kernel NÃO
// inventa. Um sensor pode estar errado mesmo com 0.95 — confiança alta não é
// verdade, é apenas "este sensor tem pouco ruído".
type Confidence float64

// Observation é a unidade atômica de percepção. Todo sensor emite este mesmo
// shape; o kernel nunca precisa saber se veio de OCR, CLIP, web ou STT.
type Observation struct {
	// ID é um identificador único da observação (gerado pelo sensor).
	ID string `json:"id"`
	// Modality é o canal sensorial (visual/text/audio/code/web/state).
	Modality Modality `json:"modality"`
	// Kind é um rótulo fino do tipo de conteúdo dentro da modalidade.
	// Ex: text:"word", text:"line"; visual:"embedding", visual:"entity".
	Kind string `json:"kind,omitempty"`
	// Content é o conteúdo observado (o texto lido, a entidade detectada,
	// o resultado da web, a transcrição). É o "o quê".
	Content string `json:"content"`
	// Confidence é a confiança do SENSOR em [0,1]. Nunca inventado.
	Confidence Confidence `json:"confidence"`
	// Source é a identidade do sensor (ex: "ocr", "clip", "web", "stt").
	// Permite ao kernel rastrear qual moter produziu — e conhecer seu perfil.
	Source string `json:"source"`
	// Epistemic é a classe epistêmica (MEASURED/INFERRED/EVIDENCE/DECISION).
	// É o que separa "li" de "deduzi".
	Epistemic EpistemicState `json:"epistemic"`
	// Timestamp é quando a observação foi capturada (wall-clock).
	Timestamp time.Time `json:"timestamp"`
	// TraceID liga a observação a um trace de execução (rastreabilidade).
	TraceID string `json:"trace_id,omitempty"`
	// Region localiza a observação no espaço (opcional).
	Region *Region `json:"region,omitempty"`
}

// New cria uma evidência MEASURED (medida diretamente pelo sensor) com a
// fonte e o canal dados. É o construtor padrão de um sensor que leu algo do
// mundo de verdade — a classe epistêmica mais forte.
//
// Ex: New(ModalityText, "line", "Punta Cana", 0.93, "ocr")
func New(mod Modality, kind, content string, conf Confidence, source string) Observation {
	return Observation{
		Modality:   mod,
		Kind:       kind,
		Content:    content,
		Confidence: conf,
		Source:     source,
		Epistemic:  EpistemicMEASURED,
		Timestamp:  time.Now().UTC(),
	}
}

// Inferred cria uma evidência inferida — um sensor não leu, deduziu. Sempre
// menos confiável que New (que mede). Usar para tracking, região deduzida,
// estimativa.
func Inferred(mod Modality, kind, content string, conf Confidence, source string) Observation {
	o := New(mod, kind, content, conf, source)
	o.Epistemic = EpistemicINFERRED
	return o
}

// Corroborated cria uma evidência EVIDENCE: dois ou mais sensores independentes
// concordaram. A classe mais forte e confiável — o kernel pode tratar como
// quase-fato, mas ainda não é FATO (isso é decisão).
func Corroborated(mod Modality, kind, content string, conf Confidence, source string) Observation {
	o := New(mod, kind, content, conf, source)
	o.Epistemic = EpistemicEVIDENCE
	return o
}

// WithRegion anexa a localização espacial da observação.
func (o Observation) WithRegion(r Region) Observation {
	o.Region = &r
	return o
}

// WithTrace anexa o trace de execução que produziu a observação.
func (o Observation) WithTrace(traceID string) Observation {
	o.TraceID = traceID
	return o
}

// WithID anexa um identificador único à observação.
func (o Observation) WithID(id string) Observation {
	o.ID = id
	return o
}

// IsTrustworthy reporta se a evidência atinge um limiar de confiança do kernel.
// É a primeira semente do GATE de escalação: uma observação abaixo do limiar
// NÃO é o kernel decidir "é falso", é o kernel saber que "não é confiável" e
// precisar escalar ou corroborar antes de agir. O limiar é escolha de política
// de quem consome (o kernel), não do sensor.
func (o Observation) IsTrustworthy(threshold float64) bool {
	return float64(o.Confidence) >= threshold
}
