@echo off
rem ============================================================================
rem post-commit.cmd — Cosca post-commit hook (Windows / cmd.exe)
rem Porta Windows do hook bash `post-commit`. Reindexa a knowledge base
rem (cosca knowledge index) quando o framework muda, valida o catálogo
rem (.opencode\cosca) e RE-ASSINA A CHAIN automaticamente quando o embed
rem mudou (ORDEM SAGRADA L199 — enforcement, não disciplina).
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
rem   - ORDEM SAGRADA: se internal\embed\cosca\ mudou, roda `cosca-check --sign-auto`
rem     automaticamente (a chain e re-assinada). Assim a ordem sagrada e cumprida
rem     por codigo, nao por disciplina.
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

rem --- AUTO-BUILD global (repo bin) + restart do serve quando o CODIGO mudou ---
rem Mudou codigo = paths que alimentam o binario do serve: cmd/, internal/, pkg/,
rem api/, sdk/, go.mod, go.sum, Makefile. Docs/.opencode/cosca/.cosca/.githooks NAO disparam.
rem IMPORTANTE: em cmd, `set "VAR="` define VAR como string vazia MAS definida, e
rem `if defined VAR` seria SEMPRE true. Por isso usamos flag por VALOR (0/1).
set "CODE_CHANGED=0"
for /f "usebackq delims=" %%f in (`git diff --name-only HEAD~1 HEAD 2^>nul`) do (
    set "FILE=%%f"
    if /i "!FILE!"=="go.mod"     set "CODE_CHANGED=1"
    if /i "!FILE!"=="go.sum"     set "CODE_CHANGED=1"
    if /i "!FILE!"=="Makefile"   set "CODE_CHANGED=1"
    if /i "!FILE:~0,4!"=="cmd/"      set "CODE_CHANGED=1"
    if /i "!FILE:~0,4!"=="pkg/"      set "CODE_CHANGED=1"
    if /i "!FILE:~0,4!"=="api/"      set "CODE_CHANGED=1"
    if /i "!FILE:~0,4!"=="sdk/"      set "CODE_CHANGED=1"
    if /i "!FILE:~0,9!"=="internal/" set "CODE_CHANGED=1"
)
if "%CODE_CHANGED%"=="1" (
    rem --- Rebuild do binario (repo bin) quando o codigo mudou ---
    if not exist "bin" mkdir "bin"
    echo [cosca-hook] codigo mudou - rebuild bin\cosca.exe 1>&2
    rem Build para arquivo temporario primeiro (nao conflita com o binario em execucao)
    go build -mod=mod -o "bin\cosca.exe.new" "cmd\cosca\main.go" >"%TEMP%\cosca-rebuild.log" 2>&1
    if errorlevel 1 (
        echo [cosca-hook] build FALHOU - mantendo binario anterior, sem restart 1>&2
        del /q "bin\cosca.exe.new" >nul 2>&1
    ) else (
        rem cosca-check tambem (opcional): manter o sign-auto com o build novo
        if exist "cmd\cosca-check\main.go" (
            go build -mod=mod -o "bin\cosca-check.exe.new" "cmd\cosca-check\main.go" >>"%TEMP%\cosca-rebuild.log" 2>&1
            if not errorlevel 1 (
                if exist "bin\cosca-check.exe" del /q "bin\cosca-check.exe" >nul 2>&1
                move /y "bin\cosca-check.exe.new" "bin\cosca-check.exe" >nul 2>&1
            ) else (
                del /q "bin\cosca-check.exe.new" >nul 2>&1
            )
        )
        rem --- Restart do serve: matar o que escuta na porta 14120 (se cosca.exe) e relancar ---
        set "SERVE_PID="
        for /f "tokens=5" %%p in ('netstat -ano 2^>nul ^| findstr ":14120" ^| findstr "LISTENING"') do set "SERVE_PID=%%p"
        if defined SERVE_PID (
            tasklist /FI "PID eq !SERVE_PID!" | findstr /I "cosca" >nul 2>&1
            if not errorlevel 1 (
                echo [cosca-hook] encerrando serve (PID !SERVE_PID!, porta 14120) 1>&2
                taskkill /PID !SERVE_PID! /F >nul 2>&1
                ping 127.0.0.1 -n 2 >nul
            )
        )
        rem Troca atomica: substitui o binario do repo pelo build novo
        move /y "bin\cosca.exe.new" "bin\cosca.exe" >nul 2>&1
        if not errorlevel 1 (
            echo [cosca-hook] install ok: bin\cosca.exe 1>&2
            if not exist ".cosca" mkdir ".cosca"
            set "COSCA_ALLOW_NO_ROOT=1"
            start "" /b cmd /c "bin\cosca.exe serve >> .cosca\serve-update.log 2>&1"
            echo [cosca-hook] serve relancado em background (porta 14120) 1>&2
        ) else (
            echo [cosca-hook] aviso: nao foi possivel trocar bin\cosca.exe.new 1>&2
        )
    )
)

rem --- Framework internal\embed\cosca (embeddings + ORDEM SAGRADA) ---
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

rem --- ORDEM SAGRADA em codigo: re-assina a chain em TODO commit ---
rem Motivo: QUALQUER commit avanca o HEAD e desalinha o anchor do ultimo bloco
rem (o git-anchor opera por GIT_COMMIT = HEAD). Para a chain estar SEMPRE alinhada
rem ao HEAD, o sign-auto roda sempre que o repo avanca. Assim o fail-closed do
rem serve nunca dispara por desalinhamento benigno.
rem O cosca-check e um binario separado (bin\cosca-check.exe), nao subcomando.
set "CSCA_CHECK=%REPO_ROOT%\bin\cosca-check.exe"
if not exist "%CSCA_CHECK%" set "CSCA_CHECK=%REPO_ROOT%\bin\cosca-check"
pushd "%REPO_ROOT%"
if exist "%CSCA_CHECK%" (
    "%CSCA_CHECK%" --sign-auto >"%TEMP%\cosca-sign.log" 2>&1
    if errorlevel 1 (
        echo ... ordem sagrada: re-assinar falhou — rode 'cosca-check --sign-auto' manualmente 1>&2
    ) else (
        echo ... ordem sagrada cumprida: family chain re-assinada automaticamente ^(HEAD avancou^) 1>&2
    )
) else (
    go run ./cmd/cosca-check --sign-auto >"%TEMP%\cosca-sign.log" 2>&1
    if errorlevel 1 (
        echo ... ordem sagrada: re-assinar falhou — rode 'cosca-check --sign-auto' manualmente 1>&2
    ) else (
        echo ... ordem sagrada cumprida: family chain re-assinada automaticamente 1>&2
    )
)
popd

rem --- Reindexa em background quando o embed muda (so nesse caso faz sentido) ---
set "CHANGED="
for /f "usebackq delims=" %%f in (`git diff --name-only HEAD~1 HEAD 2^>nul`) do (
    set "FILE=%%f"
    set "PREFIX=!FILE:~0,21!"
    if /i "!PREFIX!"=="internal/embed/cosca/" (
        set "EXT3=!FILE:~-3!"
        if /i "!EXT3!"==".md" set "CHANGED=1"
        set "EXT5=!FILE:~-5!"
        if /i "!EXT5!"==".yaml" set "CHANGED=1"
    )
)
if defined CHANGED (
    if exist "%COSCA_BIN%" (
        start "" /b cmd /c ""%COSCA_BIN%" knowledge index "%FRAMEWORK_DIR%" >> "%TEMP%\cosca-index.log" 2>&1"
    )
)

exit /b 0
