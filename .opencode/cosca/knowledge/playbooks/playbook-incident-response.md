# Playbook: Resposta a Incidentes

> **Versão**: 1.0.0 | **Workflow**: incident-response | **Duração típica**: 15-120 min

## Cenário
Queda de produção — API retornando 502 para todos os clientes.

## Passo a Passo

### Minuto 0-5: Detecção e Triage
```bash
# 1. Verificar alertas
kubectl get pods -n production
kubectl logs -n production -l app=api-gateway --tail=50

# 2. Verificar health endpoints
curl -v https://api.example.com/health

# 3. Verificar métricas
# CPU, Memory, 5xx rate, latency p95
```

**Decisão**: SE 5xx > 5% por > 2min → Declarar incidente SEV1

### Minuto 5-15: Investigação
```bash
# 1. Último deploy
kubectl rollout history deployment/api-gateway -n production

# 2. Logs de erro
kubectl logs -n production -l app=api-gateway --since=10m | grep ERROR

# 3. Database connectivity
kubectl exec -n production deploy/api-gateway -- nc -zv $DB_HOST 5432

# 4. Dependências externas
curl -I https://external-api.com/health
```

### Minuto 15-30: Mitigação
```yaml
ações_possíveis:
  - ação: "Rollback do último deploy"
    comando: "kubectl rollout undo deployment/api-gateway -n production"
    risco: baixo
    
  - ação: "Scaling up réplicas"  
    comando: "kubectl scale deployment/api-gateway --replicas=10 -n production"
    risco: baixo
    
  - ação: "Failover para região secundária"
    comando: "cosca workflow execute disaster-recovery --inputs '{\"type\":\"region-failover\"}'"
    risco: médio
    
  - ação: "Desabilitar feature flag"
    comando: "cosca feature-flag disable problematic-feature"
    risco: baixo
```

### Minuto 30-60: Resolução
```bash
# 1. Aplicar fix
kubectl apply -f deployments/hotfix.yaml

# 2. Verificar recuperação
curl -v https://api.example.com/health
# Esperado: 200 OK

# 3. Verificar métricas
# 5xx rate voltou a < 1%?
# Latency p95 < 500ms?

# 4. Notificar stakeholders
cosca workflow execute incident-response \
  --inputs '{"type":"communication","status":"resolved"}'
```

### Pós-incidente (48h): Post-mortem
```markdown
# Post-mortem: Incidente [ID]
**Data**: YYYY-MM-DD
**Severidade**: SEV1
**Duração**: 45 minutos
**Impacto**: 100% dos usuários afetados

## Timeline
- 14:00 — Alerta de 5xx > 10%
- 14:02 — Incidente declarado (SEV1)
- 14:05 — Investigação iniciada
- 14:12 — Causa encontrada: memory leak no novo deploy
- 14:15 — Rollback do deploy
- 14:30 — API recuperada (5xx < 1%)
- 14:45 — Monitoramento confirmando estabilidade

## Root Cause
Nova versão do api-gateway (v2.3.1) introduziu memory leak em conexões websocket. Após ~30min, OOM killer derrubou os pods.

## Ações
- [ ] Adicionar memory leak detection nos testes de performance (Perf Chief)
- [ ] Adicionar alerta de uso de memória > 80% (Mon Chief)
- [ ] Adicionar canary deployment com 10% de tráfego (DevOps Chief)
```

## Playbook Rápido (Card)
```
🔥 SEV1: API 502

1. Verificar health
2. Último deploy? → Rollback
3. DB conectado? → Verificar conexão
4. Dependências OK? → Ping externo
5. Muita carga? → Scale up
6. Comunicar: #incidents no Slack
```

## Related
- [Incident Response workflow](../../workflows/incident-response.md)
- [Monitoring Chief](../../departments/monitoring/SKILL.md)
- [Disaster Recovery workflow](../../workflows/disaster-recovery.md)
