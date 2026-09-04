@echo off
REM ----------------------------------------------------------------
REM Launcher do serve (Windows). NAO injeta COSCA_ALLOW_NO_ROOT
REM por default: o opt-in para rodar SEM sandbox e de responsabilidade
REM explicita do operador. Sem a env, o cosca cai no fail-closed
REM (SECURITY WARNING + exit 1) — nao sobe sem sandbox silenciosamente.
REM
REM Para aceitar o risco (ex.: dev trusted-dev), defina VOCE a env:
REM   set COSCA_ALLOW_NO_ROOT=1
REM e LEIA o warning antes de ignorar. Rodar agente/workload nao
REM confiavel sem jail NUNCA e seguro.
REM ----------------------------------------------------------------
"C:\Users\Henrique\go\bin\cosca.exe" serve --data-dir "C:\Users\Henrique\Documents\cosca\.cosca"
