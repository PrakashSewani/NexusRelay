# Xiaomi MiMo Provider Research

## Verification Record

| Field | Value |
| --- | --- |
| Provider type | `xiaomi_mimo` |
| Intended adapter | Undecided |
| Status | `blocked` |
| Researched at | 2026-07-26 |

## Authoritative Evidence

- [XiaomiMiMo/MiMo official repository](https://github.com/XiaomiMiMo/MiMo), retrieved 2026-07-26.
- The repository documents open model checkpoints and deployment through Xiaomi's vLLM fork, SGLang, and Transformers. It does not define a Xiaomi-hosted provider API contract.

## Blocking Gaps

No authoritative Xiaomi source was located that specifies a hosted API base URL, authentication, model listing, request/response schemas, SSE grammar, errors, rate limits, usage/cost, cancellation, or billing behavior. A self-hosted MiMo checkpoint is an operator-managed custom inference endpoint and is not proof of a `xiaomi_mimo` provider contract.

Keep the provider type unavailable. Before V1 scope freeze, record one design-14 disposition: unblock only after a first-party hosted API specification covers every mandatory field and the profile reaches `contract_verified`; explicitly redefine it through requirements/design as a reviewed bounded self-hosted deployment profile; or remove/defer Xiaomi MiMo from the V1 provider baseline. No decision means V1 cannot claim this baseline provider complete.
