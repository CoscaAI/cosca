# Context Memory — Session Quick-Load

> **Version**: 2.0.0 | **Status**: active | **Last Updated**: 2026-07-28
>
> **v2.0.0**: Context Compression Engine ativo. `cognitive-state.md` é o novo ponto de entrada (compacto, ~400 tokens). `session.md` mantido como fallback verbose.

## Records
| Key | Description | Load Priority |
|-----|-------------|---------------|
| [cognitive-state](cognitive-state.md) | **Compressed cognitive state** — arquitetura + decisões + pendências + riscos + próximos passos. ~400 tokens. | 1 (first) |
| [session](session.md) | Session context verbose — stack, state, active files. ~1,250 tokens. Fallback. | 2 |

## Compression
Powered by [Context Compression Engine](../../engines/context-compression/CONTEXT_COMPRESSION_ENGINE.md).
Target: 9.7k tokens → 2k tokens (79% reduction).
Token savings: ~154k tokens/mês (20 sessions).

## Usage
1. Bootstrap loads `cognitive-state.md` first (fast, 400 tokens)
2. If more context needed, loads `session.md` (verbose, 1,250 tokens)
3. On-demand: any memory category via INDEX
4. Session end: compression runs automatically
