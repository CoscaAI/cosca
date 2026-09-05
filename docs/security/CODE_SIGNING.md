# Assinatura de Código do Cosca (WDAC / Code Signing)

> **Status**: ativo | **Owner**: Kernel | **Última atualização**: 2026-08-27
> **Motivo**: o Windows **WDAC (integridade de código)** bloqueava os binários Go
> não-assinados gerados pelo `go build` / `go run` / `go test`. A solução é
> **assinar** os binários com o certificado Cosca, mantendo o WDAC ativo
> (Lei do Cofre — não desligar a proteção do sistema).

---

## Por que existe

O Cosca roda no Windows com **WDAC Enforced** (`CodeIntegrityPolicyEnforcementStatus: 2`),
que exige binários **assinados por um publicador de confiança**. Um `go build` gera
um `.exe` em `C:\Users\Henrique\cosca-test-tmp\go-buildXXXX\b001\exe\` (não-assinado)
e o WDAC o **bloqueia antes de executar** (`fork/exec ... Uma política de Controle
de Aplicativo bloqueou este arquivo`).

**Importante:** `Add-MpPreference` (exceção do Defender) **NÃO** resolve — o bloqueio
é do WDAC, camada mais forte e independente do antivírus.

## a solução (assinatura de código)

1. **Certificado auto-assinado Cosca** (`CN=CoscaAI Code Signing (Dev)`), tipo
   `CodeSigningCert`, 1 ano, thumbprint `6B5BA6E2DAC7FDD862020EDA7E913F3E475BDC0E`.
2. O **certificado público** (`.cer`) foi instalado em **`TrustedPublisher`** (Machine)
   → o WDAC passou a confiar **nas assinaturas** do publisher Cosca.
3. Cada binário é assinado com `signtool sign /f cosca-signing.pfx`.

### Arquivos

| Arquivo | Papel | Versionado? |
|---|---|---|
| `build/cert/cosca-signing.pfx` | certificado + **chave privada** (password `cosca-signing-dev`) | ❌ **NÃO** (fora do git) |
| `build/cert/cosca-signing.cer` | certificado público (para TrustedPublisher) | ❌ NÃO |
| `build/cosca-signed.exe` | binário compilado + assinado | ❌ NÃO |
| `build/build-signed.ps1` | script que compila + assina + verifica | ❌ NÃO (build/ é gitignore) |
| `docs/security/CODE_SIGNING.md` | esta documentação | ✅ SIM |

**`build/` está no `.gitignore`** — nunca versionar a chave privada do certificado.

## Como re-gerar o binário assinado

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File build\build-signed.ps1
```

O script: (1) `go build -o build\cosca-signed.exe ./cmd/cosca`, (2) assina com o
certificado, (3) verifica. Depois para rodar o benchmark sem bloqueio:

```powershell
& build\cosca-signed.exe gate recall --audit --arm both --baseline internal/grounding/testdata/qrels-baseline.json --json
```

## Recriar o certificado (se expirar/perder)

```powershell
# Criar o certificado (CurrentUser\My — sem precisar de admin para criar)
$cert = New-SelfSignedCertificate -Subject "CN=CoscaAI Code Signing (Dev)" -Type CodeSigningCert -CertStoreLocation Cert:\CurrentUser\My -KeyExportPolicy Exportable -KeyAlgorithm RSA -KeyLength 2048 -HashAlgorithm SHA256 -NotAfter (Get-Date).AddYears(1)

# Exportar público (.cer) e privado (.pfx)
Export-Certificate -Cert $cert -FilePath build\cert\cosca-signing.cer -Type Cert
$sec = ConvertTo-SecureString -String "cosca-signing-dev" -Force -AsPlainText
Export-PfxCertificate -Cert $cert -FilePath build\cert\cosca-signing.pfx -Password $sec
```

## Instalar o certificado na confiança (admin)

> Depois de criar/recriar, o Windows precisa **confiar** no certificado. Requer
> **PowerShell como Administrador**:

```powershell
Import-Certificate -FilePath "C:\Users\Henrique\Documents\cosca\build\cert\cosca-signing.cer" -CertStoreLocation Cert:\LocalMachine\TrustedPublisher
Import-Certificate -FilePath "C:\Users\Henrique\Documents\cosca\build\cert\cosca-signing.cer" -CertStoreLocation Cert:\LocalMachine\Root
```

⚠️ Usar o `.cer` (público), **não** o `.pfx`, na confiança. `Import-Certificate`
rejeita `.pfx` (que contém chave privada) — use o **`.cer`**.

---

## Regras de segurança (Lei do Cofre)

- **Nunca** versionar `build/cert/` (contém chave privada). Está no `.gitignore`.
- **Não** desligar o WDAC para "deixar rodar" — a assinatura resolve sem desproteger.
- O certificado Cosca é `Dev` e auto-assinado: somente para assinar binários Cosca
  neste ambiente. **Não** usar para outros fins.
