# Kilo Exporter Profile

- State: `blocked`
- Owner/reviewer: NexusRelay maintainers
- Retrieved: 2026-07-26
- Verified package: `@kilocode/cli@7.4.16`

## Authoritative Sources

- Configuration schema: `https://app.kilo.ai/config.json`

The package had npm integrity `sha512-sOvq0HW6CZebCvGyUn0fTFj/1mX4nplpuoCJuqe6QjDUliLNYRC6ky9Rjl8kdT/nk0OGNO1kRaygEyc7+Htr2Q==` and npm SHA-1 `c39f0f94f1cae2aeed28b4a5b5a952a5efab2b1d`.

## Research Result

The authoritative schema includes an OpenCode-derived custom-provider shape with `npm`, `options.baseURL`, `options.apiKey`, and explicit `models`. That establishes syntax compatibility at the schema level, but it does not by itself establish a safe NexusRelay export workflow.

Available evidence supports trusted user/global configuration. It does not authoritatively document a Kilo-specific, highest-precedence run-scoped configuration mechanism that binds the NexusRelay base URL and environment-backed key reference against untrusted project overrides. NexusRelay does not infer that OpenCode environment variables or precedence controls are supported by Kilo merely because the schemas share definitions.

## Blocker

The exporter remains blocked until authoritative Kilo documentation or verified source behavior establishes all of:

1. Custom OpenAI-compatible provider behavior for the pinned CLI release.
2. Environment-variable secret-reference syntax without plaintext fallback.
3. Configuration search paths, merge order, and project-versus-global precedence.
4. A run-scoped highest-precedence placement, or another fail-closed mechanism, that binds `baseURL` and the key reference together.
5. Model declaration and supported capability/limit fields.
6. Repository-pinned golden fixtures and deterministic validation for that supported workflow.

No Kilo artifact, installation instruction, or support claim may be emitted while this profile is blocked.
