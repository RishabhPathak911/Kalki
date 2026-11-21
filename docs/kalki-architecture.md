# Kalki architecture

Five modules across two tiers, with no required external service:

```text
Tier 1: Prana → Kavach → Nadi → Kavach gate → Setu disk WAL
                                                   ↓ HTTPS + ACK/hash
Tier 2: Setu regional ingest → Medha analysis → authenticated console
```

| Module | Code | Responsibility |
| --- | --- | --- |
| Prana | tier1/collect, tier1/collector | OS metrics, TCP probes, NAB/Loghub replay |
| Kavach | tier1/filter, tier1/policy, internal/telemetry | Secrets, tenant HMAC pseudonyms, fail-closed schemas |
| Nadi | tier1/aggregate, tier1/analyse | Five-second rollups, local brute-force detection |
| Setu | tier1/send, tier1/wal, tier2/ingest | WAL, retry/ACK, deduplication, event-time ordering, egress proof |
| Medha | tier2/medha, tier2/workspace, tier2/ai, dashboard | Health, evidence, auth, audits, safe simulation, AI briefs and console |

Data is policy-approved before durable append and send. No raw-log field exists
in outbound schemas. ACKs include contiguous sequence/gaps and received-byte
SHA-256. Receiver restart triggers retained-WAL replay. Tier 2 uses memory rings,
not a persistent database. Tokens/sessions are hashed in memory.

Workspace scope comes from bearer tokens. The console verifies regional TLS
with the local CA. Loopback proof and opt-in demo controls require workspace
ownership. Medha AI reconstructs prompts from numeric evidence and approved
enums, excluding raw questions and arbitrary playbook strings. Default deterministic
mode is offline. Optional configured providers receive only that projection.
AI has no execution tool; remediation needs approval and stays simulated.

Validation and load tools are evaluation utilities, not product modules.
There is no eBPF, Gorilla TSDB, OpenTelemetry tracing or bundled LLM.
See README and docs/phase7.md for runbooks, measurements and limitations.
