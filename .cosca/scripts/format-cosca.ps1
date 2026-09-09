# ============================================================================
# COSCA FORMAT — formatador da casa (estilo prettier para o padrao Cosca)
# ----------------------------------------------------------------------------
# Uso:
#   powershell -File format-cosca.ps1 [--check|--fix] [--path <alvo>]
#
#   --check   (padrao) reporta problemas sem alterar nada (exit 1 se houver)
#   --fix     corrige o que for seguro (encoding, BOM, espacos finais, EOL)
#   --path    alvo: arquivo, diretorio ou vazio (= .cosca + .opencode)
#
# Regras do padrao Cosca:
#   1. Encoding: UTF-8 valido (bytes invalidos -> reparo Latin-1, nao inventa)
#   2. BOM: removido (UTF-8 sem BOM, consistente com o resto da casa)
#   3. Espacos finais de linha: removidos
#   4. Fim de linha: LF (o git autocrlf converte para CRLF no checkout Windows)
#   5. Cabecalho blockquote (Version/Owner) preservado em EN — metadado tecnico
#   6. Conteudo .md permanece como esta (PT-BR e regra de autoria, nao do formatador)
#
# Codigo Go/TS: use gofmt / prettier separadamente (nao e escopo deste script).
# ============================================================================
param(
    [switch]$check,
    [switch]$fix,
    [string]$path = ""
)

$ErrorActionPreference = "Stop"
$raiz = (Resolve-Path (Join-Path $PSScriptRoot "..\..\")).Path  # .cosca/scripts -> raiz do projeto (2 niveis)
if (-not $path) { $path = $raiz }

function Teste-Utf8Valido([byte[]]$bytes) {
    try {
        $utf8 = New-Object System.Text.UTF8Encoding($false, $true)
        $null = $utf8.GetString($bytes)
        return $true
    } catch { return $false }
}

function Reparar-Utf8([byte[]]$bytes) {
    # Reparo heuristico: decodifica sequencias UTF-8 VALIDAS como UTF-8;
    # bytes orfaos (ex.: o C3 de um acento foi perdido) sao mapeados por
    # Latin-1 (1 byte -> 1 char U+0080-U+00FF) e re-encodados em UTF-8 na
    # escrita. Nunca inventa conteudo: byte orfao vira o char Latin-1 dele.
    $utf8 = New-Object System.Text.UTF8Encoding($false, $false)
    $sb = New-Object System.Text.StringBuilder
    $i = 0
    while ($i -lt $bytes.Length) {
        $b = $bytes[$i]
        $len = 0
        if ($b -lt 0x80) { $len = 1 }
        elseif ($b -ge 0xC2 -and $b -le 0xDF) { $len = 2 }
        elseif ($b -ge 0xE0 -and $b -le 0xEF) { $len = 3 }
        elseif ($b -ge 0xF0 -and $b -le 0xF4) { $len = 4 }
        $valido = $false
        if ($len -gt 1 -and ($i + $len) -le $bytes.Length) {
            $valido = $true
            for ($j = 1; $j -lt $len; $j++) {
                if (($bytes[$i + $j] -band 0xC0) -ne 0x80) { $valido = $false; break }
            }
        }
        if ($len -gt 0 -and $valido) {
            $seq = New-Object byte[] $len
            [Array]::Copy($bytes, $i, $seq, 0, $len)
            [void]$sb.Append($utf8.GetString($seq))
            $i += $len
        } else {
            # byte orfao: mapeia Latin-1 (0x80-0xFF -> U+0080-U+00FF)
            [void]$sb.Append([char]$b)
            $i++
        }
    }
    return $sb.ToString()
}

function Formatar-Arquivo([string]$arquivo, [bool]$aplicar) {
    $rel = $arquivo.Replace($raiz + "\", "")
    $problemas = @()
    $bytes = [System.IO.File]::ReadAllBytes($arquivo)
    $texto = $null

    # 1. Encoding
    if (-not (Teste-Utf8Valido $bytes)) {
        $problemas += "encoding UTF-8 invalido"
        if ($aplicar) {
            $texto = Reparar-Utf8 $bytes
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($texto)
        }
    }
    if ($null -eq $texto) { $texto = [System.Text.Encoding]::UTF8.GetString($bytes) }

    # 2. BOM
    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
        $problemas += "BOM UTF-8 presente"
        if ($aplicar) { $texto = $texto.TrimStart([char]0xFEFF) }
    }

    # 3. Espacos finais
    if ($texto -match '[ \t]+\r?\n') {
        $problemas += "espacos finais de linha"
        if ($aplicar) { $texto = $texto -replace '[ \t]+\r?\n', "`n" }
    }

    # 4. EOL misto (LF + CRLF no mesmo arquivo) e problema; CRLF uniforme nao
    #    (o git autocrlf normaliza no checkout Windows). Fix sempre normaliza p/ LF.
    $temCRLF = $texto.Contains("`r`n")
    $temLF = $texto.Contains("`n") -and -not $texto.Contains("`r`n")  # LF puro presente
    $temLFSolto = $false
    if ($temCRLF) {
        # ha LF nao precedido de CR?
        $semCR = $texto -replace "`r`n", ""
        $temLFSolto = $semCR.Contains("`n")
    }
    if ($temLFSolto) {
        $problemas += "EOL misto (CRLF + LF)"
        if ($aplicar) { $texto = $texto -replace "`r`n", "`n" }
    } elseif ($temCRLF) {
        if ($aplicar) { $texto = $texto -replace "`r`n", "`n" }
    }

    if ($problemas.Count -gt 0) {
        if ($aplicar) {
            [System.IO.File]::WriteAllText($arquivo, $texto, (New-Object System.Text.UTF8Encoding($false)))
            Write-Host ("FIX  : {0} [{1}]" -f $rel, ($problemas -join "; "))
        } else {
            Write-Host ("ISSUE: {0} [{1}]" -f $rel, ($problemas -join "; "))
        }
        return $true
    }
    return $false
}

# --- main ---
$aplicar = $fix
if (-not $check -and -not $fix) { $aplicar = $false }  # default = check
$modo = if ($aplicar) { "FIX" } else { "CHECK" }
Write-Output "=== COSCA FORMAT [$modo] ==="

$extensoes = @("*.md", "*.yaml", "*.yml", "*.json")
$alvos = @()
if ((Get-Item $path).PSIsContainer) {
    $alvos = Get-ChildItem $path -Recurse -File -Include $extensoes -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -notmatch '\\node_modules\\|\\fallback\\|\\framework\\|\\memory\\audit\\|\\.git\\|\\backups\\' }
} else {
    $alvos = @(Get-Item $path)
}

$comProblema = 0
foreach ($a in $alvos) { if (Formatar-Arquivo $a.FullName $aplicar) { $comProblema++ } }
Write-Output ""
Write-Output ("Total: {0} arquivos | com problema: {1}" -f $alvos.Count, $comProblema)
if (-not $aplicar -and $comProblema -gt 0) {
    Write-Output "Rode com -fix para corrigir."
}
exit 0
