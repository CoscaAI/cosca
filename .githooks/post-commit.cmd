@echo off
rem ============================================================================
rem post-commit.cmd — Cosca post-commit hook (Windows / cmd.exe)
rem Porta Windows do hook bash `post-commit`. Reindexa a knowledge base
rem (cosca knowledge index) quando o framework muda e valida o catálogo
rem (.opencode\cosca) via `cosca gate catalog --check --summary`.
rem
rem IMPORTANTE: o git-for-windows executa hooks SEM extensao (post-commit)
rem via bash embutido e NAO invoca post-commit.cmd sozinho. Por isso o
rem script bash `post-commit` (mesmo diretorio) detecta Windows e delega
rem para este .cmd via: cmd //c "$(cygpath -w "$(dirname "$0")")\post-commit.cmd"
rem
rem Comportamento (espelha o bash original):
rem   - Catálogo: se .opencode\cosca\ mudou no ultimo commit, roda
rem     `cosca gate catalog --check --summary`. Se falhar (drift), imprime
rem     a orientacao de re-gerar o snapshot e segue — post-commit NUNCA
rem     trava o commit (exit 0).
rem   - Framework: se internal\embed\cosca\ mudou, roda `cosca knowledge index`
rem     em BACKGROUND (start /b) sem travar o commit.
rem   - Usa %COSCA_BIN% ou default %USERPROFILE%\.cosca\bin\cosca.exe
rem   - Erros silenciosos (equivalente ao `|| true` do bash)
rem ============================================================================

setlocal EnableExtensions EnableDelayedExpansion

rem --- Repo root ---
for /f "usebackq delims=" %%i in (`git rev-parse --show-toplevel 2^>nul`) do set "REPO_ROOT=%%i"
if not defined REPO_ROOT exit /b 0

if not defined COSCA_BIN set "COSCA_BIN=%USERPROFILE%\.cosca\bin\cosca.exe"

rem --- Catálogo (.opencode\cosca) — generate-and-diff ---
set "CATALOG_CHANGED="
for /f "usebackq delims=" %%f in (`git diff --name-only HEAD~1 HEAD 2^>nul`) do (
    set "FILE=%%f"
    rem Prefixo ".opencode/cosca/" (16 chars) — qualquer mudanca no catálogo
    set "CPREFIX=!FILE:~0,16!"
    if /i "!CPREFIX!"==".opencode/cosca/" set "CATALOG_CHANGED=1"
)
if defined CATALOG_CHANGED (
    if exist "%COSCA_BIN%" (
        pushd "%REPO_ROOT%"
        "%COSCA_BIN%" gate catalog --check --summary >"%TEMP%\cosca-catalog.log" 2>&1
        if errorlevel 1 (
            echo catálogo desatualizado — rode cosca gate catalog --generate e commite 1>&2
        )
        popd
    )
)

rem --- Framework internal\embed\cosca (embeddings) ---
set "FRAMEWORK_DIR=%REPO_ROOT%\internal\embed\cosca"
if not exist "%FRAMEWORK_DIR%\" exit /b 0

rem --- Verifica se algo do framework mudou no ultimo commit ---
set "CHANGED="
for /f "usebackq delims=" %%f in (`git diff --name-only HEAD~1 HEAD 2^>nul`) do (
    set "FILE=%%f"
    rem Prefixo "internal/embed/cosca/" (21 chars) + extensao .md ou .yaml
    set "PREFIX=!FILE:~0,21!"
    if /i "!PREFIX!"=="internal/embed/cosca/" (
        set "EXT3=!FILE:~-3!"
        if /i "!EXT3!"==".md" set "CHANGED=1"
        set "EXT5=!FILE:~-5!"
        if /i "!EXT5!"==".yaml" set "CHANGED=1"
    )
)
if not defined CHANGED exit /b 0

if not exist "%COSCA_BIN%" exit /b 0

rem --- Reindexa em background, desacoplado do git (nao trava o commit) ---
start "" /b cmd /c ""%COSCA_BIN%" knowledge index "%FRAMEWORK_DIR%" >> "%TEMP%\cosca-index.log" 2>&1"

exit /b 0
