> **Version**: 1.1.0 | **Status**: active | **Owner**: Provider Chief | **Last Updated**: 2026-08-01

# PROVIDER DISCOVERY SKILL

## Description
Discover, evaluate, and compare AI providers (LLM, embedding, image, audio) for specific use cases. Assesses providers on capability, cost, latency, reliability, compliance, and integration complexity.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| use_case | Yes | Description of the AI use case |
| requirements | Yes | `capability`, `cost`, `latency`, `reliability`, `compliance`, `all` |
| budget_constraint | No | Maximum budget per month |
| latency_requirement | No | Maximum acceptable latency (ms) |
| compliance_needs | No | Required compliance certifications |

## Outputs
| Output | Description |
|--------|-------------|
| Provider comparison | Matrix of providers vs requirements |
| Recommendations | Top providers ranked for use case |
| Cost estimates | Estimated monthly cost per provider |
| Integration guide | Steps to integrate recommended provider |

## Evaluation Criteria

### Capability
- Model performance on relevant benchmarks (MMLU, HumanEval, etc.)
- Supported modalities (text, image, code, audio)
- Context window size
- Fine-tuning availability
- Rate limits and concurrency

### Cost
- Per-token pricing (input and output)
- Volume discounts and reservations
- Cost projection for estimated usage
- Data transfer and API call costs
- Commitment-free vs reserved pricing

### Latency
- P50/P95/P99 response times
- Time-to-first-token (streaming)
- Geographic availability and edge locations
- Queue times under load

### Reliability
- SLA guarantees (uptime percentage)
- Historical outage data
- Error rates and retry policies
- Rate limiting and throttling behavior

### Compliance
- Data processing location (GDPR, LGPD)
- Data retention policies
- SOC2/HIPAA certifications
- Model training data usage
- Privacy and confidentiality guarantees

## Local Voice Providers

For the voice capability, the following 100% local, free providers are the recommended default. No cloud audio services.

| Provider | Capability | License | Notes |
|----------|-----------|---------|-------|
| Kokoro | TTS (text-to-speech) | Apache-2.0 | Kokoro-82M, real-time on CPU, PT-BR native (voice code `p`) |
| whisper.cpp | STT (speech-to-text) | MIT | CPU via OpenBLAS, faster than real-time |

Zero cost per use, zero audio exfiltration. See the [Voice Engine](../../engines/voice/SKILL.md) for integration details.

## Process
1. Define use case requirements and constraints
2. Identify candidate providers (OpenAI, Anthropic, Google, AWS, local)
3. Collect capability, cost, latency, and compliance data
4. Score each provider against requirements
5. Generate comparison matrix
6. Rank providers by weighted score
7. Calculate cost estimates for projected usage
8. Create integration guide for top recommendations

## Success Criteria
- [ ] At least 3 providers evaluated per use case. Evaluation includes: latency (p50/p99), cost per 1K tokens, accuracy on benchmark prompts, rate limits. Recommendation documented with trade-offs.
- [ ] Providers scored against requirements
- [ ] Cost estimates calculated for each provider
- [ ] Compliance requirements verified
- [ ] Integration guide provided for recommended provider
- [ ] Evaluation documented in memory

## Related
- [Provider Chief](../../departments/provider/SKILL.md)
- [AI Chief](../../departments/ai/SKILL.md)
- [Prompt Engineering](./PROMPT_ENGINEERING.md)
- [Embedding Pipeline](./EMBEDDING_PIPELINE.md)
- [PROVIDER_INTERFACE.md](../../PROVIDER_INTERFACE.md)
- [Voice Engine](../../engines/voice/SKILL.md)
