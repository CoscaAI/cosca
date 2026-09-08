// Package staterepair implementa o REPAIR ASSISTIDO DE ESTADO para bancos de
// estado DERIVADOS/REGENERÁVEIS do Cosca (ADR-043 §8, pacote 1 — "marcha" do
// Don em 2026-09-08). Ele executa a Camada C do ADR-014
// (cosca recover --auto para a classe derivado/estado) com as salvaguardas do
// `hermes_state_repair.py` (Hermes Agent — adaptado, NUNCA portado):
//
//   - Fingerprint do arquivo doente: hash sha256 sobre (tamanho + amostra de
//     conteúdo com os ranges voláteis do header SQLite mascarados — bytes
//     24-28 change counter e 92-96 version-valid-for). Objetivo: escrita viva
//     (WAL ativo) não muda o fingerprint e não "rearma" o ledger de tentativas.
//   - Health(dbPath): `PRAGMA quick_check` em conexão read-only (mode=ro,
//     mesmo padrão do internal/dbhealth).
//   - Ledger de tentativas sidecar `<db>.repair-attempts.json`: recusa nova
//     cirurgia quando o MESMO fingerprint já falhou 3 vezes
//     (MaxAttempts/AttemptsExhausted).
//   - ForensicBackup(sickPath): cópia forense do arquivo doente com dedupe por
//     conteúdo (hash) e retenção de no máximo MaxBackups por banco.
//   - Repair(dbKind, opts): Health → (dry-run: plano) / (apply: ledger guard →
//     backup forense → quarentena → recipe de rebuild → Health pós → sucesso
//     registra / falha restaura o original e devolve instruções manuais).
//
// ESCOPO (invariante de segurança — ADR-014 regra 3/4): bancos DERIVADOS e
// regeneráveis somente — session(s), memory/index.db, knowledge.db e
// vector-*.db. FORA DE ESCOPO por construção: family_chain.dat (chain),
// internal/embed/cosca (embed) e identidade/chaves — nunca há caminho
// automático para eles (SupportedKinds não os contém e nenhuma recipe os
// referencia). Repair nunca é "meio-repair": se a recipe não existe ou falha,
// o original é devolvido da quarentena e o chamador recebe instruções manuais
// explícitas.
//
// O pacote é uma biblioteca (manager pattern): o comando `cosca db repair`
// (internal/cli) injeta o Manager. Nenhum caminho existente (serve/boot/engine)
// é alterado por este pacote.
package staterepair
