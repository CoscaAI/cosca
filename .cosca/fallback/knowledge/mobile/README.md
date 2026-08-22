# 03 — MOBILE ENGINEERING INTELLIGENCE

> Stack 03 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Construir aplicações mobile (iOS/Android/RN/Expo/Flutter) entendendo que **mobile não é desktop pequeno**.

## PRINCÍPIOS CORE
1. **Mobile-first por contexto, não por tela** — pensar em `DEVICE → USER CONTEXT → NETWORK → BATTERY → INPUT → LIFECYCLE → SECURITY` antes da tecnologia · UNIVERSAL
2. **Offline-first quando o contexto é móvel** (rede instável é a norma) · STRONG
3. **Listas virtualizadas** (FlashList) para performance · UNIVERSAL
4. **Gestos/toque/safe areas/plataforma** são cidadãos de primeira classe · UNIVERSAL
5. **Navegação e lifecycle** explicitamente projetados (deep links, background, OTA) · STRONG

## REGRAS DE DECISÃO
- **RN/Expo vs Flutter vs Native**: equipe, domínio, need nativo, distribuição. Expo é o default para RN novo (managed → prebuild).
- **Offline-first vs online-first**: decisão por contexto do usuário, não por moda.
- **Safe storage** para tokens; nunca em async storage puro.

## ANTI-PATTERNS
`mobile como afterthought` · `tabela desktop em tela de toque` · `gesture handling manual frágil` · `lists sem virtualização` · `secrets em storage inseguro` · `UI bloqueante em rede` · `ignorar keyboard/safe-area`

## CHECKLIST
- [ ] Alvos de toque ≥ 44px (iOS) / ≥ 48dp (Android)
- [ ] Offline → degradação graciosa + sincronização
- [ ] Safe areas + keyboard handling
- [ ] Listas virtualizadas
- [ ] Deep links + estado de navegação
- [ ] Perfis de bateria/rede considerados

## A REGRA
Raciocine primeiro sobre o dispositivo e o contexto do usuário; depois escolha a tecnologia.

## DOUTRINAS
- `REACT_NATIVE.md` — React Native: React Navigation 7 tipado, atomic design, service-layer zod, ky (não axios), secure-store

## REFERÊNCIAS
React Native · Expo · Flutter · Reanimated · Gesture Handler · FlashList · RN Testing Library · Vercel RN guidelines
