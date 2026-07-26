# CommandCode Provider API Research

## Verification Record

| Field | Value |
| --- | --- |
| Provider type | `commandcode` |
| Intended adapter | Undecided; likely a bounded OpenAI-compatible Chat profile |
| Status | `blocked` |
| Researched at | 2026-07-26 |

## Authoritative Evidence

- [Command Code Provider API](https://commandcode.ai/docs/provider), retrieved 2026-07-26.
- [Pricing and limits](https://commandcode.ai/docs/resources/pricing-limits), retrieved 2026-07-26.

The first-party documentation establishes:

- Base endpoints `https://api.commandcode.ai/provider/v1/chat/completions`, `/messages`, and `/models`.
- Bearer authentication for all routes and `x-api-key` compatibility for Anthropic Messages.
- OpenAI Chat and Anthropic Messages body shapes, text/image input, streaming, final stream usage, model listing, optional `x-cmd-zdr: 1`, and common error envelopes/statuses.

## Blocking Gaps

The published contract does not establish a Responses endpoint, Embeddings endpoint, exact Chat SSE chunk/terminator and mid-stream error grammar, complete supported-parameter/capability metadata, rate-limit headers/quotas, authoritative per-request cost fields, failed/cancelled charge behavior, cancellation semantics, or a cheap authenticated probe classification. It also states that vision capability is not pre-gated per model and that 5xx bodies carry upstream messages, which requires explicit sanitization behavior.

These are material fields in `docs/design/14-provider-verification.md`; marketing-level OpenAI compatibility cannot fill them. Keep implementation blocked until first-party documentation or a versioned first-party schema supplies the missing contract. Before V1 scope freeze, record one design-14 disposition: verify and include it, explicitly redefine it through requirements/design as a narrower bounded provider profile, or remove/defer it from the V1 baseline. No decision means V1 cannot claim this baseline provider complete. This provider research is separate from the CommandCode coding-agent exporter profile.
