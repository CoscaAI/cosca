#!/usr/bin/env python3
"""
COSCA - Autenticação limpa do Google Colab CLI (resolve o bug "Scope has changed")
================================================================================
Gera o token.json com os escopos EXATOS que o colab-cli espera, evitando o
descompasso de escopo. Este script configura o PYTHONPATH automaticamente para
encontrar as libs do ambiente colab-cli (uv tool), entao rode com:
    python3 ~/colab_auth.py
"""
import os
import sys
import json
from importlib import resources

# Adiciona o site-packages do ambiente colab-cli (uv tool) ao PYTHONPATH,
# para que google_auth_oauthlib/google.oauth2 sejam encontrados mesmo com o
# python3 do sistema.
_SITE = os.path.expanduser(
    "~/.local/share/uv/tools/google-colab-cli/lib/python3.12/site-packages")
if os.path.isdir(_SITE) and _SITE not in sys.path:
    sys.path.insert(0, _SITE)

from google_auth_oauthlib.flow import InstalledAppFlow

PUBLIC_SCOPES = [
    "openid",
    "https://www.googleapis.com/auth/userinfo.profile",
    "https://www.googleapis.com/auth/userinfo.email",
    "https://www.googleapis.com/auth/cloud-platform",
    "https://www.googleapis.com/auth/colaboratory",
    "https://www.googleapis.com/auth/drive.file",
]
TOKEN_PATH = os.path.expanduser("~/.config/colab-cli/token.json")
REMOTE_REDIRECT = "https://sdk.cloud.google.com/applicationdefaultauthcode.html"

cfg = json.loads(resources.files("colab_cli").joinpath("oauth_config.json").read_text())

flow = InstalledAppFlow.from_client_config(cfg, PUBLIC_SCOPES)
flow.redirect_uri = REMOTE_REDIRECT
auth_url, _ = flow.authorization_url(prompt="consent", token_usage="remote")

print("\nURL de autorizacao (abra no navegador):\n")
print("  " + auth_url)
print("\nAprove e copie o authorization code.\n")

code = input("Enter the authorization code: ").strip()
flow.fetch_token(code=code)
creds = flow.credentials

os.makedirs(os.path.dirname(TOKEN_PATH), exist_ok=True)
with open(TOKEN_PATH, "w") as f:
    f.write(creds.to_json())

print("\nAUTENTICADO! token salvo em " + TOKEN_PATH)
