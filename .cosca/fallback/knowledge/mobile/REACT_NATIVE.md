# 05 — REACT NATIVE INTELLIGENCE

> Stack 05 da Cosca Engineering Intelligence Matrix.
> Doutrina do mobile-app TypeScript da casa: separação UI vs lógica, service-layer com boundary validada, TanStack Query como fonte de estado server.

## MISSÃO
Construir apps React Native com uma arquitetura de pastas e responsabilidades que separe UI de lógica de negócio, valide dados no boundary da API (zod), trate estado server via TanStack Query e estado local via Zustand — com tema multi-variante, i18n, storage MMKV e segurança mobile de primeira classe.

## PRINCÍPIOS CORE
1. **Separação UI vs lógica** — screens/hooks/composição na UI; schema+service+use na camada de domínio; nenhum fetch solto no componente · UNIVERSAL
2. **Service-layer com validação no boundary** — `schema.ts` (zod) valida a resposta no limite da API, antes de tocar o estado · STRONG
3. **TanStack Query como fonte de estado server** — queries/mutações/cache; Zustand só para estado local de UI · STRONG
4. **Storage MMKV + secure-store** — MMKV para persistência de dados; Keychain/rn-secure-storage para tokens · UNIVERSAL
5. **Segurança mobile como cidadã de primeira classe** — tokens em secure-store, SSL pinning, obfuscation, sem CodePush em prod · STRONG

## REGRAS DE DECISÃO
- `App.tsx` é composição raiz, na ordem: GestureHandler → QueryClient → ThemeProvider → Navigator.
- Navegação tipada: React Navigation 7 + `navigation/types.ts` (`RootStackParamList`).
- Feature organizada em `src/hooks/domain/<feature>/`: `schema.ts` (zod) → `*Service.ts` (ky) → `use*.ts` (TanStack Query).
- Cliente HTTP com **ky** (fetch-based) — **axios quebra no RN por causa de interceptors/streams**; instância única em `services/instance.ts`.
- Theme com tokens + variantes dark/light e types gerados (persistência MMKV).
- Componentes em atomic design: atoms/molecules/organisms/templates.
- Testes com Jest + RN Testing Library: `tests/` com mocks de libs nativas e `TestAppWrapper`.
- `Startup` screen faz fetch pré-navegação; dados necessários resolvem antes do fluxo principal.

## ANTI-PATTERNS
`fetch solto dentro de componente` · `axios em RN` · `estado server em store local` · `tokens em AsyncStorage puro` · `validação ausente no boundary da API` · `navigation sem types` · `CodePush ativo em prod` · `SSL pinning ignorado` · `theme hardcoded em vez de tokens` · `teste montando app real em vez de TestAppWrapper`

## CHECKLIST
- [ ] `App.tsx` composição raiz na ordem: GestureHandler → QueryClient → ThemeProvider → Navigator
- [ ] `navigation/types.ts` tipa `RootStackParamList` (React Navigation 7)
- [ ] Feature segue `schema.ts` (zod) → `*Service.ts` (ky) → `use*.ts` (TanStack Query)
- [ ] Cliente HTTP ky em `services/instance.ts`; sem axios
- [ ] Tokens em secure-store (Keychain/rn-secure-storage); MMKV para o resto
- [ ] TanStack Query para estado server; Zustand apenas para estado local
- [ ] Theme via tokens + variantes dark/light com types gerados (MMKV persist)
- [ ] i18next configurado em `translations/`
- [ ] SSL pinning ativo; CodePush desligado em prod; obfuscation no build
- [ ] Testes com Jest + RN Testing Library usando `TestAppWrapper` e mocks de libs nativas

## A REGRA
React Native não é "um web app na tela do celular": a fonte da verdade é o TanStack Query, a fronteira da confiança é o zod, e a segurança do token decide se o app sobrevive — UI separada da lógica, sempre.

## REFERÊNCIAS
React Native docs · React Navigation 7 · TanStack Query · Zustand · ky · MMKV · react-native-keychain / rn-secure-storage · zod · i18next · Jest + RN Testing Library · PADRAO-COSCA §6 · mobile/README.md · jondot/awesome-react-native · thecodingmachine/react-native-boilerplate

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: mobile-001
DOMAIN: mobile
TITLE: Service-layer com validação zod no boundary da API
PROBLEM: Resposta de API chega malformada/mudada e quebra a UI com crash ou estado corrompido silencioso
CONTEXT: RN + TypeScript, qualquer feature com chamada remota
PRINCIPLE: A fronteira de confiança é o boundary da API; o que entra no app só existe validado
RECOMMENDATION: `schema.ts` (zod) valida a resposta no `*Service.ts` antes de propagar; UI recebe tipo garantido
WHEN_TO_USE: Toda chamada de API; especialmente em times grandes onde o contrato muda sem aviso
WHEN_NOT_TO_USE: Payloads 100% controlados e versionados internamente com contrato imutável
TRADE_OFFS: Custo do parse por request vs confiança no dado e erro cedo (fail-closed)
EXAMPLE: `users.schema.ts` → `UsersService.list` valida com `.parse()` → `useUsers` (TanStack Query)
COUNTER_EXAMPLE: Componente fazendo `.map((u: any) => u.name)` direto na resposta bruta do fetch
FAILURE_MODES: Erro de validação engolido e dado `any` vazando para a UI
REFERENCES: zod docs · thecodingmachine/react-native-boilerplate
CONFIDENCE: STRONG
SOURCE: awesome-react-native + react-native-boilerplate
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: mobile-002
DOMAIN: mobile
TITLE: ky como cliente HTTP em vez de axios
PROBLEM: axios quebra e se comporta mal no runtime React Native (streams/interceptors); adiciona peso desnecessário
CONTEXT: RN com cliente HTTP baseado em fetch
PRINCIPLE: Use o mínimo que o runtime suporta nativamente; fetch é o fundação do RN
RECOMMENDATION: ky (fetch-based, tipado, leve) em `services/instance.ts`; nunca axios em RN
WHEN_TO_USE: Qualquer app RN novo; apps existentes em fetch puro sem infraestrutura axios
WHEN_NOT_TO_USE: Codebase consolidada em axios com timeout/retry custom já funcionando — migre incrementalmente
TRADE_OFFS: Retry/cancel via plugins do ky vs ecossistema de interceptors do axios
EXAMPLE: `const api = ky.create({ prefixUrl: API_URL, hooks })` exportado de `instance.ts`
COUNTER_EXAMPLE: `axios.create()` com interceptor de token que quebra em ambiente RN
FAILURE_MODES: Migração parcial misturando ky e axios com semantics de erro diferentes
REFERENCES: ky docs · awesome-react-native
CONFIDENCE: STRONG
SOURCE: thecodingmachine/react-native-boilerplate
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: mobile-003
DOMAIN: mobile
TITLE: Atomic design para componentes
PROBLEM: Componentes crescem sem hierarquia e a UI vira uma sopa de átomos gigantes acoplados
CONTEXT: RN, design system em apps de médio/grande porte
PRINCIPLE: Composição por granularidade: atoms (base) → molecules → organisms → templates
RECOMMENDATION: `src/components/` dividido em atoms/molecules/organisms/templates; screens compõem templates
WHEN_TO_USE: Apps com design system, múltiplas telas e equipe distribuída
WHEN_NOT_TO_USE: Protótipo/CRUD mínimo onde a camada de abstração é custo puro
TRADE_OFFS: Hierarquia de pastas + indireção vs consistência e reuso real
EXAMPLE: `Button` (atom) → `SearchBar` (molecule) → `UserCard` (organism) → `UserListTemplate`
COUNTER_EXAMPLE: `components/` plano com `BigListScreen.thing.tsx` de 500 linhas reutilizado por cópia
FAILURE_MODES: Atom que importa serviço/hook de domínio, quebrando a separação UI vs lógica
REFERENCES: atomic design (bradfrost) · react-native-boilerplate
CONFIDENCE: STRONG
SOURCE: thecodingmachine/react-native-boilerplate
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: mobile-004
DOMAIN: mobile
TITLE: Tokens em secure-store (Keychain/rn-secure-storage), nunca em AsyncStorage
PROBLEM: Token/sessão vazados de AsyncStorage/MMKV ficam legíveis para outras apps e dumps de backup
CONTEXT: RN, autenticação, dados sensíveis
PRINCIPLE: Segredo vive em cofre do SO, não em storage de app
RECOMMENDATION: Tokens e segredos em Keychain (iOS) / Keystore (Android) via react-native-keychain ou rn-secure-storage; MMKV/AsyncStorage só para dados não-sensíveis
WHEN_TO_USE: Sempre que houver token, refresh token, biometria ou credencial
WHEN_NOT_TO_USE: Preferências de UI e cache — dados públicos não precisam de cofre
TRADE_OFFS: Latência/API nativa do secure-store vs segurança real do material sensível
EXAMPLE: `Keychain.setGenericPassword("token", jwt)` no login; leitura no interceptor/hook
COUNTER_EXAMPLE: `AsyncStorage.setItem("token", jwt)` no login (material sensível legível)
FAILURE_MODES: Secure-store indisponível (simulador/keystore corrompida) sem fallback seguro → logout explícito, nunca downgrade
REFERENCES: react-native-keychain · rn-secure-storage · mobile/README.md
CONFIDENCE: UNIVERSAL
SOURCE: awesome-react-native + PADRAO-COSCA §6 (segurança)
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: mobile-005
DOMAIN: mobile
TITLE: TanStack Query como fonte de estado server + Zustand só para estado local
PROBLEM: Estado server duplicado em store global dessincroniza cache, e o código de fetching/retry/cache é reescrito à mão
CONTEXT: RN/TypeScript, qualquer app com chamadas remotas
PRINCIPLE: Estado que vem do servidor tem ciclo de vida próprio (cache, invalidação, retry); estado de UI é efêmero
RECOMMENDATION: Queries/mutações em TanStack Query; `useState`/Zustand apenas para UI local (tema, modais, formulário em memória)
WHEN_TO_USE: App com >1 feature consumindo API remota
WHEN_NOT_TO_USE: App offline-only sem estado server compartilhado (Query add é peso)
TRADE_OFFS: Cache/API de Query a aprender vs sincronização e retry quase grátis
EXAMPLE: `useUsers` chama `UsersService.list` via `useQuery`; loading/refetch/invalidation por `queryClient`
COUNTER_EXAMPLE: Mapa global "users" em Zustand preenchido à mão em cada tela e jamais invalidado
FAILURE_MODES: Mismo dado server e local misturados no mesmo store → stale UI e bugs de dupla fonte
REFERENCES: TanStack Query docs · Zustand docs · react-native-boilerplate
CONFIDENCE: STRONG
SOURCE: thecodingmachine/react-native-boilerplate
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
