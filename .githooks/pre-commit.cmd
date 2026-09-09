@echo off
rem ============================================================================
rem pre-commit.cmd — Cosca pre-commit hook (Windows / cmd.exe)
rem Porta Windows do hook bash `pre-commit`. Roda o COSCA FORMAT
rem (format-cosca.ps1) em --fix antes de todo commit e re-adiciona os
rem arquivos limpos ao indice. Padrao da casa por CODIGO.
rem
rem IMPORTANTE: o git-for-windows executa hooks SEM extensao (pre-commit)
rem via bash embutido e delega para este .cmd (ver pre-commit bash).
rem ============================================================================
setlocal EnableExtensions

rem --- Repo root ---
for /f "usebackq delims=" %%i in (`git rev-parse --show-toplevel 2^>nul`) do set "REPO_ROOT=%%i"
if not defined REPO_ROOT exit /b 0

rem --- COSCA FORMAT --fix (raiz do projeto: .cosca + .opencode) ---
set "FORMAT_SCRIPT=%REPO_ROOT%\.cosca\scripts\format-cosca.ps1"
if exist "%FORMAT_SCRIPT%" (
    powershell -NoProfile -ExecutionPolicy Bypass -File "%FORMAT_SCRIPT%" -fix >nul 2>&1
)

rem --- GOFMT: formata os arquivos .go staged (padrao Go da casa) ---
rem Fecha o buraco do padrao: nada de .go nao-formatado entra no commit.
where gofmt >nul 2>&1
if %errorlevel%==0 (
    for /f "usebackq delims=" %%g in (`git diff --cached --name-only --diff-filter=ACM 2^>nul ^| findstr /i /e ".go"`) do (
        if exist "%REPO_ROOT%\%%g" gofmt -w "%REPO_ROOT%\%%g"
    )
)

rem --- Re-adiciona o que o formatador limpou (whitespace/encoding/BOM/gofmt) ---
git add -u >nul 2>&1

exit /b 0
