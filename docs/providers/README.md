# Provider Profiles

Provider profiles record the authoritative external contract that an adapter may implement. They supplement the provider-neutral designs in `docs/design/04-providers-secrets.md`, `docs/design/05-inference-protocol.md`, and `docs/design/13-api-compatibility-matrix.md`; they do not widen the public NexusRelay API.

## Status Meaning

- `contract_verified` means the documented external contract has been reviewed and is sufficiently explicit to begin adapter implementation and deterministic mock-server fixtures.
- `profile_drafted` means authoritative research is recorded but at least one material mapping, stream lifecycle, usage, error, cost, or capability contract remains unresolved; adapter implementation is blocked.
- It does not mean the adapter exists, its contract tests pass, or the provider is release-supported.
- For the custom OpenAI-compatible profile, `contract_verified` applies only to NexusRelay's bounded configuration and adapter contract. Each configured endpoint remains `operator_asserted`; no arbitrary endpoint is externally verified by this status.
- `blocked` means first-party evidence was reviewed but one or more material implementation contracts remain missing.
- Release support additionally requires the implementation, deterministic contract fixtures, capability wiring, security review, and documentation gates in `docs/design/14-provider-verification.md`.

## Profiles

| Provider | Profile | Status | Recorded/verified at | Review due |
| --- | --- | --- | --- | --- |
| OpenAI | [openai.md](openai.md) | `contract_verified` | 2026-07-26 | 2026-10-26 |
| Anthropic | [anthropic.md](anthropic.md) | `contract_verified` | 2026-07-26 | 2026-10-26 |
| Google Gemini | [gemini.md](gemini.md) | `profile_drafted` | - | - |
| OpenRouter | [openrouter.md](openrouter.md) | `profile_drafted` | - | - |
| Ollama | [ollama.md](ollama.md) | `contract_verified` | 2026-07-26 | 2026-10-26 |
| Groq | [groq.md](groq.md) | `profile_drafted` | - | - |
| Custom OpenAI-compatible | [openai-compatible.md](openai-compatible.md) | `contract_verified` (adapter contract); endpoints `operator_asserted` | 2026-07-26 | 2026-10-26 |
| Xiaomi MiMo | [xiaomi-mimo.md](xiaomi-mimo.md) | `blocked` | - | - |
| CommandCode Provider API | [commandcode.md](commandcode.md) | `blocked` | - | - |

Mutable provider facts such as model availability, model-specific context limits, quotas, and pricing are captured as versioned discovery or pricing snapshots. A profile verifies how those facts are discovered and interpreted, not a permanent copy of their current values.

Xiaomi MiMo and CommandCode remain release-decision blockers while they are both listed in the V1 baseline and `blocked`. Before scope freeze, each must be verified, explicitly redefined as a bounded deployment profile, or removed/deferred through the requirements and ledger process in design 14.
