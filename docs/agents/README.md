# Agent Exporter Profiles

Agent configuration formats are independent external contracts. A named NexusRelay exporter is enabled only when its profile in this directory reaches `contract_verified` under `docs/design/15-agent-config-export.md`.

Profiles record authoritative sources, schema/grammar version and hash, retrieval and review dates, key-reference syntax, OpenAI-compatible base-URL behavior, model declaration shape, merge/collision semantics, output location guidance, deterministic fixtures, and optional smoke-test instructions.

Current targets:

| Agent | State |
| --- | --- |
| OpenCode | `profile_drafted` |
| Kilo | `not_researched` |
| CommandCode | `not_researched` |

No state below `contract_verified` is a support claim.
