# Manual de Autoajuda do Cosca

> **Regra de prioridade:** quando houver dúvida, não preencha lacunas com
> suposições. Pare, formule a dúvida, descubra o que é observável e registre
> o grau de confiança antes de agir.

Este manual descreve um procedimento de trabalho para investigar problemas,
tomar decisões e pedir ajuda no Cosca. Ele é uma orientação operacional; não é
uma declaração de que cada etapa abaixo já existe como comando, agente,
automação ou mecanismo do produto.

## 1. Comece pela descoberta

Antes de editar qualquer coisa:

1. Defina o objetivo em uma frase verificável: “ao final, X deve estar Y”.
2. Localize os arquivos, módulos, documentação, testes e configuração
   relacionados.
3. Leia os pontos de entrada e as referências antes de concluir como o sistema
   funciona.
4. Verifique o estado atual: mudanças locais, arquivos não rastreados,
   branches e processos/sessões que possam estar trabalhando no mesmo diretório.
5. Separe fatos observados, inferências e perguntas abertas (modelo abaixo).

Não trate o nome de um arquivo, uma documentação antiga ou uma intenção de
roadmap como prova de comportamento implementado.

### Registro mínimo de descoberta

| Tipo | Registro |
|---|---|
| Fato | O que foi visto diretamente em arquivo, saída de comando, teste ou log |
| Inferência | Conclusão derivada de um ou mais fatos |
| Desconhecido | O que ainda não foi verificado |
| Fonte | Caminho, comando, teste, commit ou pessoa consultada |
| Confiança | Alta, média ou baixa; explique o motivo |

## 2. Fatos, inferências e variáveis desconhecidas

Use linguagem explícita:

- **Fato:** “o arquivo contém...” ou “o teste retornou...”.
- **Inferência:** “isso sugere...” ou “a hipótese mais provável é...”.
- **Desconhecido:** “não foi possível confirmar...”.

Liste as variáveis que podem mudar a decisão: versão, ambiente, permissões,
dados, configuração, dependência externa, estado do Git, concorrência e
critério de sucesso. Para cada variável, indique como observá-la e o impacto
de deixá-la sem confirmação. Se não puder observá-la, reduza a confiança e
evite uma ação irreversível.

## 3. Contradições

Quando duas fontes discordarem, não escolha silenciosamente uma delas:

1. Registre as duas afirmações e suas fontes.
2. Verifique versão, data, escopo e se uma fonte descreve intenção enquanto a
   outra descreve estado implementado.
3. Dê preferência à evidência direta e atual, sem descartar a fonte divergente.
4. Se a contradição afetar segurança, dados, compatibilidade ou uma ação
   destrutiva, suspenda a execução e escale ao Don.
5. Documente a resolução, ou marque a questão como pendente.

## 4. Confiança e decisão

Confiança não é certeza nem substitui validação. Classifique cada conclusão:

- **Alta:** evidência direta, atual e reproduzível.
- **Média:** evidência consistente, mas com alguma variável não confirmada.
- **Baixa:** hipótese plausível, dependente de interpretação ou sem teste.

Uma decisão deve conter: objetivo, opções consideradas, evidências,
desconhecidos, riscos, reversibilidade, custo e critério de validação. Quando
duas opções forem aceitáveis, prefira a que seja reversível, observável e menor
em escopo. Declare o trade-off em vez de escondê-lo: velocidade versus
segurança, simplicidade versus flexibilidade, custo imediato versus manutenção,
ou precisão versus cobertura.

## 5. Formule o problema antes de pedir solução

Use este formato:

```text
Objetivo:
Sintoma/resultado observado:
Comportamento esperado:
Escopo afetado:
Fatos confirmados:
Inferências:
Desconhecidos e impacto:
Tentativas e resultados:
Restrições (tempo, compatibilidade, segurança, dados):
Critério de sucesso:
Pergunta específica:
```

Uma boa pergunta permite responder “o que devemos decidir ou verificar agora?”
Evite perguntas vagas como “por que não funciona?” sem incluir evidência e
reprodução.

## 6. Consulte especialistas sem terceirizar o julgamento

Consulte o especialista do domínio quando a decisão exigir conhecimento que
não está confirmado no material disponível, ou quando o risco de errar for
alto. Envie o registro de problema, as fontes consultadas e a decisão que
precisa ser tomada; peça que ele distinga fato, hipótese e recomendação.

Use consultas independentes para reduzir viés em decisões importantes. Não
transforme a quantidade de respostas em prova: compare premissas, evidências,
incertezas e critérios de sucesso. Se as respostas divergirem, mantenha as
alternativas visíveis e peça uma decisão baseada no trade-off.

## 7. Quando houver múltiplas respostas

Não produza uma lista sem conclusão. Para cada alternativa, informe:

| Campo | Pergunta |
|---|---|
| Adequação | Resolve qual parte do objetivo? |
| Premissas | O que precisa ser verdadeiro? |
| Risco | O que pode dar errado e quão reversível é? |
| Custo | Qual esforço, complexidade ou dependência cria? |
| Validação | Como saberemos que funcionou? |
| Confiança | O que sustenta a recomendação? |

Depois recomende uma opção, ou diga claramente por que não é possível
recomendar sem uma informação adicional. Se o Don precisar escolher, apresente
as opções e uma pergunta de decisão objetiva.

## 8. Sessões concorrentes e Git

Antes de trabalhar em um diretório compartilhado, confirme o escopo da sua
sessão e o estado do repositório. Não apague, reverta, reformate ou sobrescreva
mudanças que não criou. Separe trabalho por branch/worktree quando isso fizer
parte do procedimento disponível; caso contrário, coordene o arquivo e o
escopo explicitamente.

Ao detectar mudanças concorrentes:

1. Pare antes de editar o mesmo trecho.
2. Preserve o estado encontrado e identifique a origem, se possível.
3. Reavalie o diff após qualquer alteração externa.
4. Resolva conflitos com contexto e validação, não por “última escrita vence”.
5. Informe arquivos tocados, conflitos e verificações realizadas.

Commit, merge, rebase, push e descarte são ações distintas. Só os execute
quando autorizados pelo escopo da tarefa. Um pedido de documentação não é
autorização para modificar código ou criar commit.

## 9. Valide antes de declarar concluído

Validação proporcional ao risco deve cobrir:

- diff limitado ao escopo pedido;
- arquivos esperados presentes e arquivos não relacionados preservados;
- links e referências válidos;
- sintaxe e formatação do formato alterado;
- testes, checks ou reprodução relevantes, quando existirem;
- ausência de segredos, dados sensíveis e mudanças acidentais.

Registre o que foi validado, o resultado e o que não pôde ser validado. “Não
executei” é informação útil; não o apresente como “passou”. Para Markdown,
faça ao menos uma verificação de diff e uma validação de links/sintaxe
disponível no projeto, sem inventar uma ferramenta que não esteja instalada.

## 10. Memória: registrar sem cristalizar hipóteses

Memória deve preservar contexto útil para a próxima sessão, não substituir a
fonte primária. Registre conclusão, evidência, data/versão, confiança e
pendências. Marque explicitamente inferências e decisões provisórias. Ao
reutilizar uma memória, revalide-a contra o estado atual; se estiver obsoleta,
corrija-a ou a marque como desatualizada.

Não registre credenciais, tokens ou dados sensíveis. Uma memória não confirmada
serve como pista para descoberta, nunca como fato técnico.

## 11. Quando escalar ao Don

Escale imediatamente quando houver risco de perda de dados, exposição de
segredo, impacto de segurança, ação irreversível, bloqueio de trabalho, conflito
de autoridade ou contradição que não possa ser resolvida. Escale também quando
o objetivo, o critério de sucesso ou a prioridade estiverem ambíguos.

Mensagem curta de escalonamento:

```text
Decisão necessária:
Contexto e impacto:
Fatos confirmados (fontes):
Contradições/desconhecidos:
Opções e trade-offs:
Recomendação e confiança:
Risco de esperar:
Pergunta objetiva ao Don:
```

Até receber orientação, preserve o estado, evite ações irreversíveis e faça
somente descobertas ou validações seguras. O Don decide prioridades e exceções;
o manual não transforma uma hipótese em autoridade.

## Checklist rápido

- [ ] O objetivo e o critério de sucesso estão claros?
- [ ] Descobri o estado atual antes de editar?
- [ ] Separei fatos, inferências e desconhecidos?
- [ ] Registrei fontes, confiança e contradições?
- [ ] Comparei trade-offs e reversibilidade?
- [ ] Consultei o especialista certo quando necessário?
- [ ] Protegi mudanças de outras sessões e o estado do Git?
- [ ] Validei o resultado e declarei limitações?
- [ ] Registrei memória sem transformar hipótese em fato?
- [ ] Escalei ao Don quando o risco ou a ambiguidade exigiu?
