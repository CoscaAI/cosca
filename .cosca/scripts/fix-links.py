#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""Corrige links quebrados nos .md de .cosca (defeito de reorganizacao).

Estrategia segura:
  - para cada link quebrado, pega o basename do alvo;
  - procura esse basename em TODA a arvore .cosca;
  - se encontrado em UM unico lugar -> era 'movido' -> corrige o path relativo;
  - se nao encontrado OU ambiguo -> reporta (decisao separada, nao mexe).
Preserva UTF-8 (open encoding='utf-8'), nunca toca '#'/ancora nem links http.
"""
import os, re, glob, sys
from collections import defaultdict

BASE = r"C:\Users\Henrique\Documents\cosca\.cosca"
LINK_RE = re.compile(r"\]\(([^)#]+?)(#[^)]*)?\)|<([^>]+\.md)>")

def all_files():
    for dp, _, fns in os.walk(BASE):
        for fn in fns:
            yield os.path.join(dp, fn)

# indice: basename -> lista de paths (para saber se e unico)
basename_index = defaultdict(list)
for f in all_files():
    if f.endswith((".md", ".yaml", ".yml")):
        basename_index[os.path.basename(f).lower()].append(f)

def resolve_link(fpath, tgt):
    # normaliza, resolve relativo ao arquivo
    return os.path.normpath(os.path.join(os.path.dirname(fpath), tgt))

def fix_link(fpath, tgt):
    """Retorna o novo tgt corrigido, ou None se nao resolvivel com seguranca."""
    # ignora placeholders/exemplos
    if tgt in ("path", "img.png", "") or tgt.startswith(("http", "mailto", "#")):
        return None
    # se ja existe, nao e quebrado
    if os.path.exists(resolve_link(fpath, tgt)):
        return None
    base = os.path.basename(tgt.split("#")[0])
    if not base or base.lower() not in basename_index:
        return None  # nao encontrado em lugar nenhum (removido de verdade)
    cands = basename_index[base.lower()]
    if len(cands) != 1:
        return None  # ambiguo, nao mexe (poderia apontar pro errado)
    # novo path relativo da origem ao candidato unico
    cand = cands[0]
    rel = os.path.relpath(cand, start=os.path.dirname(fpath)).replace("\\", "/")
    # preserva a ancora se havia
    anc = ""
    m = re.match(r"^(.*?)(#.*)?$", tgt)
    if m and m.group(2): anc = m.group(2)
    return rel + anc

fixed, changed_files, unresolved, ambiguous = 0, set(), [], []
for f in all_files():
    if not f.endswith(".md"): continue
    try: txt = open(f, encoding="utf-8", errors="ignore").read()
    except: continue
    orig = txt
    def repl(m):
        global fixed
        tgt = (m.group(1) or m.group(3) or "").strip()
        newt = fix_link(f, tgt)
        if newt is not None and newt != tgt:
            fixed += 1
            return f"]({newt})" if m.group(1) else f"<{newt}>"
        if os.path.exists(resolve_link(f, tgt)):
            return m.group(0)
        # quebrado e nao resolvido
        if tgt != "path" and tgt != "img.png" and not tgt.startswith(("http","#")):
            unresolved.append((os.path.relpath(f, BASE), tgt))
        return m.group(0)
    txt2 = LINK_RE.sub(repl, txt)
    if txt2 != orig:
        open(f, "w", encoding="utf-8", errors="ignore").write(txt2)
        changed_files.add(f)

print(f"LINKS CORRIGIDOS: {fixed} (em {len(changed_files)} arquivos)")
print(f"NAO RESOLVIDOS (ausentes de verdade/ambiguos): {len(unresolved)}")
from collections import Counter
agg = Counter()
for f, t in unresolved:
    if ".opencode" in t: agg["opencode-legado"] += 1
    elif "archive" in t: agg["archive"] += 1
    elif "learnings" in t: agg["learnings"] += 1
    else: agg["outro"] += 1
for k, v in agg.most_common(): print(f"  {k}: {v}")
print("=== amostra nao-resolvidos (8) ===")
for f, t in unresolved[:8]: print(f"  {f} -> {t}")
