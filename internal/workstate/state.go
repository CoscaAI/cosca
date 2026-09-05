// Package workstate — snapshot/restore INCREMENTAL (O(k)) do working-state do
// agente (ADAPTER P5, extraído da mineração ADR-017 do prime-agent: snapshot/
// restore por nome, com caps, escrita atômica tmp+os.Replace e manifest JSON).
//
// PROPOSITO — o Cosca preserva o PROCESSO (internal/runtime: daemon, watchdog,
// auto-backup) mas NÃO preservava o ESTADO DE TRABALHO do agente (variáveis,
// handles de tarefa, análise incremental) como estado durável que sobrevive a
// compaction/restart e permite detach/reattach sem perder andamento. Este
// pacote implementa esse mecanismo de forma DETERMINÍSTICA (I1) e ZERO-LLM (I2).
//
// AVISO DE COMPLEXIDADE (honesto — não existe "O(1) snapshot/restore" aqui):
//   - GRAVAR só o delta (Save) = O(k), k = entradas mudadas desde o checkpoint.
//   - REAPLICAR o delta sobre um estado já em memória (COW) = O(k).
//   - COLD-RESTORE (Load do disco, processo novo) = O(n) para ler o base UMA
//     vez + O(k) para aplicar o delta. Não é O(1); o O(n) é o custo mínimo de
//     materializar o estado commitado.
//   - Checkpoint = O(k); Rollback = O(k); DirtyKeys/DeltaSnapshot = O(k log k);
//     Entries/Len/TotalBytes = O(n) sobre o estado materializado.
// A propriedade que este pacote GARANTE é: o custo de gravar/reaplicar as
// MUDANÇAS escala com o tamanho do DELTA (k), não com o estado total (n) — é
// isso que evita copiar o estado inteiro a cada snapshot incremental.
//
// O que NÃO é este pacote:
//   - NÃO é conhecimento. Conhecimento vive em internal/memory e
//     internal/semantic-memory (Semantic Engine). Aqui é só o working-state
//     (estado de trabalho do agente) — variáveis/handles em andamento.
//   - NÃO é memória de longo prazo nem chain. É estado efêmero durável.
//
// PADRÕES (do prime-agent repl.md, adaptados para Go stdlib-only):
//   - Serialização POR NOME (per-name), com CAPS de tamanho (MaxEntryBytes /
//     MaxTotalBytes) e pruning determinístico (LRU por ordem de atualização).
//   - ESCRITA ATÔMICA: gravar em ficheiro temporário (tmp) e os.Replace por
//     cima do destino — nunca escrita parcial (ver atomicWrite em store.go).
//   - MANIFEST JSON que descreve o snapshot (que entries existem, tamanhos).
//   - DELTA incremental O(k): checkpoint (base copy-on-write) + delta que só
//     registra o que mudou desde o último checkpoint. Checkpoint NÃO é O(1);
//     é O(k) (fold do delta no base).
//
// REGRAS DE PUREZA (ADR-015): primitiva stdlib-only. O modelo copy-on-write
// (base + delta + touched) ESPELHA internal/evolution/checkpoint.go (gema #5),
// mas é auto-contido aqui porque o evolution.Txn não tem caps, pruning, nem
// tombstones. O reuso é por COMPOSIÇÃO na borda (o Store usa o State), nunca
// por importação da primitiva (disciplina ADR-015). `go list -deps` = stdlib only.
package workstate

import "sort"

// =============================================================================
// Caps — limites de tamanho e política de overflow.
// =============================================================================

// Caps define os limites (caps) do working-state.
type Caps struct {
	// MaxEntryBytes é o teto por entrada. Entradas maiores são truncadas
	// (se TruncateOverflow=true) ou omitidas (se false). 0 = ilimitado por entrada.
	MaxEntryBytes int
	// MaxTotalBytes é o teto do state inteiro. Quando excedido, faz pruning
	// determinístico (descarta as entradas menos recentemente atualizadas — LRU).
	// 0 = ilimitado no total.
	MaxTotalBytes int
	// TruncateOverflow escolhe o comportamento do cap por entrada:
	// true  → trunca o valor nos primeiros MaxEntryBytes bytes.
	// false → omite a entrada inteira (a entrada é "dropada").
	TruncateOverflow bool
}

// DefaultCaps devolve uma configuração conservadora: 1 MiB por entrada,
// 16 MiB total, truncando por entrada (nunca perde um handle inteiro).
func DefaultCaps() Caps {
	return Caps{
		MaxEntryBytes:   1 << 20, // 1 MiB
		MaxTotalBytes:   16 << 20, // 16 MiB
		TruncateOverflow: true,
	}
}

// =============================================================================
// State — working-state em memória com copy-on-write (base + delta).
// =============================================================================

// State é o working-state do agente, com semântica copy-on-write:
//
//	base    = estado COMMITADO (último checkpoint) — nunca muta no meio;
//	delta   = mutações desde o base (Set/Delete);
//	deleted = tombstones (chaves apagadas no delta — escondem o base);
//	touched = união de delta+deleted (as "chaves sujas" = o delta de tamanho k).
//
// COMPLEXIDADE REAL (por operação):
//   - Set / Get / Has / Delete = O(1) médio (map).
//   - Checkpoint = O(k) (k = edits + tombstones) — fold do delta no base.
//   - Rollback = O(k) — descarta k referências do delta.
//   - DirtyKeys / DeltaSnapshot = O(k log k) (coleta k + sort.Strings).
//   - Entries / Len / TotalBytes = O(n) sobre o estado materializado.
//   - BaseSnapshot = O(n) (cópia do base).
//
// Não há "checkpoint O(1)": o fold percorre o delta. A vantagem COW real é que
// o Rollback é barato (descarta só o delta) e o snapshot incremental grava/reaplica
// só O(k) — sem materializar o estado inteiro.
type State struct {
	base     map[string]string
	delta    map[string]string
	deleted  map[string]bool
	touched  map[string]bool
	touchSeq map[string]uint64
	caps     Caps
	seq      uint64
}

// NewState cria um working-state vazio com os caps dados.
func NewState(caps Caps) *State {
	return &State{
		base:     make(map[string]string),
		delta:    make(map[string]string),
		deleted:  make(map[string]bool),
		touched:  make(map[string]bool),
		touchSeq: make(map[string]uint64),
		caps:     caps,
	}
}

// NewStateWithBase cria um working-state partindo de um base (checkpoint) já
// carregado — usado no cold-restore (Load). O base é o estado COMMITADO.
// O mapa base é COPIADO (não referenciado): mutar o mapa do chamador depois
// não afeta o State, e vice-versa. O(n) por copiar o base.
func NewStateWithBase(base map[string]string, caps Caps) *State {
	b := make(map[string]string, len(base))
	for k, v := range base {
		b[k] = v
	}
	return &State{
		base:     b,
		delta:    make(map[string]string),
		deleted:  make(map[string]bool),
		touched:  make(map[string]bool),
		touchSeq: make(map[string]uint64),
		caps:     caps,
	}
}

// Caps devolve os caps em vigor.
func (st *State) Caps() Caps { return st.caps }

// applyEntryCap aplica o cap por entrada e devolve o valor resultante
// (truncado ou omitido), mais um bool indicando se a entrada deve ser mantida.
func (st *State) applyEntryCap(name, value string) (string, bool) {
	if st.caps.MaxEntryBytes <= 0 {
		return value, true
	}
	if len(value) > st.caps.MaxEntryBytes {
		if st.caps.TruncateOverflow {
			return value[:st.caps.MaxEntryBytes], true
		}
		// omitir: a entrada inteira é dropada (como um delete efêmero).
		return "", false
	}
	return value, true
}

// Set grava (ou atualiza) uma entrada. O valor é submetido ao cap por entrada.
// A escrita vai para o DELTA (copy-on-write) — o base fica INTACTO.
// O(1) médio para o insert; O(n) no pior caso quando pruning dispara (o total
// excede MaxTotalBytes e o estado é re-avaliado).
func (st *State) Set(name, value string) {
	value, keep := st.applyEntryCap(name, value)
	st.seq++
	if !keep {
		// entrada omitida por cap: trata como remoção.
		st.deleteInternal(name)
		return
	}
	st.delta[name] = value
	delete(st.deleted, name)
	st.touched[name] = true
	st.touchSeq[name] = st.seq
	st.pruneTotal()
}

// Delete remove uma entrada (tombstone no delta) — o base não é tocado.
// Um tick posterior pode Checkpoint() para selar a remoção.
func (st *State) Delete(name string) {
	st.seq++
	st.deleteInternal(name)
}

// deleteInternal marca a chave como deletada no delta.
func (st *State) deleteInternal(name string) {
	delete(st.delta, name)
	st.deleted[name] = true
	st.touched[name] = true
	st.touchSeq[name] = st.seq
}

// Get lê uma entrada: delta primeiro (respeitando tombstone), senão base.
// O(1) médio por chave (2-3 lookups de map).
func (st *State) Get(name string) (string, bool) {
	if st.deleted[name] {
		return "", false
	}
	if v, ok := st.delta[name]; ok {
		return v, true
	}
	v, ok := st.base[name]
	return v, ok
}

// Has informa se a entrada existe no estado corrente (base+delta). Deletes
// escondem a entrada (retorna false). O(1) médio.
func (st *State) Has(name string) bool {
	_, ok := st.Get(name)
	return ok
}

// Checkpoint sela (commit) o delta no base. Depois do checkpoint o
// working-state fica LIMPO de dirty — equivalente ao commit do evolution.Txn,
// mas aqui também materializa tombstones (deletes definitivos).
// O(k) — percorre delta + deleted (k = edits + tombstones). NÃO é O(1).
func (st *State) Checkpoint() {
	for name, v := range st.delta {
		st.base[name] = v
	}
	for name := range st.deleted {
		delete(st.base, name)
	}
	st.resetDelta()
}

// Rollback descarta o delta — o base fica INTACTO. Sempre deixa o
// working-state no estado do último checkpoint. O(k) para largar as referências
// das k entradas sujas (a realocação interna é O(1)).
func (st *State) Rollback() {
	st.resetDelta()
}

func (st *State) resetDelta() {
	st.delta = make(map[string]string)
	st.deleted = make(map[string]bool)
	st.touched = make(map[string]bool)
	st.touchSeq = make(map[string]uint64)
}

// DirtyKeys devolve, deterministicamente (ordenado), as chaves que mudaram
// desde o último checkpoint — o "delta" de tamanho k. Usado pelo snapshot para
// gravar SÓ o que mudou. O(k log k) por causa do sort.Strings.
func (st *State) DirtyKeys() []string {
	keys := make([]string, 0, len(st.touched))
	for k := range st.touched {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// DirtyCount devolve quantas chaves estão sujas (|delta| = k). O(1).
func (st *State) DirtyCount() int { return len(st.touched) }

// DeltaSnapshot devolve o par (sets, deletes) do delta para serialização.
// 'sets' = chaves com valor novo; 'deletes' = chaves removidas. Só o delta.
// O(k log k) por causa do sort.Strings(deletes).
func (st *State) DeltaSnapshot() (sets map[string]string, deletes []string) {
	sets = make(map[string]string, len(st.delta))
	for k, v := range st.delta {
		sets[k] = v
	}
	for k := range st.deleted {
		deletes = append(deletes, k)
	}
	sort.Strings(deletes)
	return sets, deletes
}

// BaseSnapshot devolve uma COPIA do base (state commitado). O(n).
func (st *State) BaseSnapshot() map[string]string {
	out := make(map[string]string, len(st.base))
	for k, v := range st.base {
		out[k] = v
	}
	return out
}

// Entries devolve a visão FUSIONADA do working-state (base ∪ delta − deletes),
// com as chaves que existem corrente (para o manifest). O(n) — materializa uma
// nova cópia do estado.
func (st *State) Entries() map[string]string {
	out := make(map[string]string, len(st.base)+len(st.delta))
	for k, v := range st.base {
		if !st.deleted[k] {
			out[k] = v
		}
	}
	for k, v := range st.delta {
		out[k] = v
	}
	return out
}

// Len devolve quantas entradas existem no estado corrente. O(n) (materializa).
func (st *State) Len() int { return len(st.Entries()) }

// TotalBytes devolve o tamanho total (soma dos valores) do estado corrente.
// O(n) (materializa via Entries).
func (st *State) TotalBytes() int {
	total := 0
	for _, v := range st.Entries() {
		total += len(v)
	}
	return total
}

// DrainBase devolve e limpa o base (usado no Load quando o antigo base é
// substituído por um checkpoint recém-lido).
func (st *State) DrainBase() map[string]string {
	b := st.base
	st.base = make(map[string]string)
	// preserva delta/deleted/touched intactos (o chamador reaplica o delta).
	return b
}

// pruneTotal faz pruning determinístico (LRU) quando o total excede MaxTotalBytes.
// Descarta as entradas menos recentemente atualizadas até caber. O(n) a O(n²) no
// pior caso (cada iteração re-materializa Entries().TotalBytes() e varre o estado).
func (st *State) pruneTotal() {
	if st.caps.MaxTotalBytes <= 0 {
		return
	}
	for st.TotalBytes() > st.caps.MaxTotalBytes {
		name, ok := st.leastRecentlyTouched()
		if !ok {
			return
		}
		st.dropEntry(name)
	}
}

func (st *State) leastRecentlyTouched() (string, bool) {
	var minName string
	var minSeq uint64 = ^uint64(0)
	found := false
	cur := st.Entries()
	for name := range cur {
		s, ok := st.touchSeq[name]
		if !ok {
			// nunca tocado no delta: trata como mais antigo (base estável).
			if !found {
				minName, minSeq, found = name, 0, true
			}
			continue
		}
		if s < minSeq || !found {
			minName, minSeq, found = name, s, true
		}
	}
	return minName, found
}

// dropEntry remove uma entrada do estado inteiro (base+delta+deleted+touch).
func (st *State) dropEntry(name string) {
	delete(st.base, name)
	delete(st.delta, name)
	delete(st.deleted, name)
	delete(st.touched, name)
	delete(st.touchSeq, name)
}
