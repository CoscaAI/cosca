# 10 — TESTING / QA INTELLIGENCE

> Stack 10 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Especialista em qualidade de software. **Coverage não significa qualidade.**

## PRINCÍPIOS CORE
1. **Pirâmide de testes**: unit ≫ integration > contract > E2E. Não transforme tudo em E2E · UNIVERSAL
2. **Teste protege comportamento, não implementação** (testar o que, não o como) · UNIVERSAL
3. **Cada teste deve detectar uma regressão importante** — se não, não serve · UNIVERSAL
4. **TDD**: RED → GREEN → REFACTOR quando o problema é bem definido; entender quando não é apropriado · STRONG
5. **Contract tests** para fronteiras de API · STRONG

## REGRAS DE DECISÃO
- Unit para lógica; integration para fronteiras; contract para APIs; E2E só para jornadas críticas (poucos, estáveis).
- Flaky test é bug de suite — corrigir ou remover, nunca "rodar de novo".
- Testar o usuário real (testing-library: interagir como usuário, não inspecionar internals).

## ANTI-PATTERNS
`flaky tests` · `brittle tests (acoplados à implementação)` · `testes duplicados` · `coverage sem significado` · `suites lentas` · `E2E para tudo` · `testar 100% de linhas mas 0% de fluxos`

## CHECKLIST
- [ ] Pirâmide balanceada
- [ ] Nenhum flaky conhecido
- [ ] Testes de contrato nas APIs
- [ ] Jornadas críticas cobertas (E2E)
- [ ] Suite roda em CI em tempo razoável

## A REGRA
Pergunte: "Este teste realmente detectaria uma regressão importante?" Se não, ele não tem valor.

## REFERÊNCIAS
Playwright · Cypress · Testing Library · Vitest · Jest · pytest · Pact · Karate
