# cosca-security — Patterns

## P1 — Secrets Detection
**Quando usar**: Auditoria de código.
**Padrão**: Regex para padrões conhecidos (API key, token, password) + verificação de .env e .bashrc.
**Exemplo**: Token ghp_ no .bashrc (L98) — removido, revogado.

## P2 — Fail-Closed Security
**Quando usar**: Toda decisão de segurança.
**Padrão**: Se não conseguir validar, NEGUE. Nunca faça fallback para "permitir".
**Exemplo**: isLoopbackPeer() retorna false se não conseguir extrair peer → exige JWT.

## P3 — Loopback Exemption
**Quando usar**: Autenticação entre processos locais.
**Padrão**: gRPC com peer.IsLoopback() → skip auth. Se exposto em rede (0.0.0.0), exigir TLS + JWT.
**Exemplo**: AuthInterceptor no gRPC server.
