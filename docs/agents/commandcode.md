# CommandCode Exporter Profile

- State: `blocked`
- Owner/reviewer: NexusRelay maintainers
- Retrieved: 2026-07-26

## Research Result

Authoritative documentation has not established CommandCode's bring-your-own-provider configuration grammar for a custom OpenAI-compatible endpoint. In particular, NexusRelay has no authoritative contract for the base-URL field, bearer-key representation, environment-variable secret reference, explicit model declarations, merge precedence, or safe output placement.

## Blocker

The exporter remains blocked until authoritative CommandCode documentation or verified source artifacts establish:

1. The exact custom-provider schema or grammar and supported release version.
2. OpenAI-compatible base-URL semantics, including whether `/v1` is expected.
3. A non-plaintext secret reference and its failure behavior when the variable is absent.
4. Model declaration and capability/limit syntax.
5. Configuration precedence and a fail-closed placement that prevents an untrusted project from redirecting a trusted key reference.
6. Repository-pinned schema or golden fixtures with provenance and hashes.

NexusRelay must not infer CommandCode syntax from OpenCode, Kilo, provider API documentation, or generic OpenAI compatibility. No CommandCode artifact or support claim may be emitted while this profile is blocked.
