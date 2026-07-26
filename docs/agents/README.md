# Agent Exporter Profiles

Agent configuration formats are independent external contracts. A named NexusRelay exporter is enabled only when its profile in this directory reaches `contract_verified` under `docs/design/15-agent-config-export.md`.

Profiles record authoritative sources, schema/grammar version and hash, retrieval and review dates, key-reference syntax, OpenAI-compatible base-URL behavior, model declaration shape, merge/collision semantics, output location guidance, deterministic fixtures, and optional smoke-test instructions.

Support applies only to the installation and invocation method recorded by the profile. A schema-valid fragment is not sufficient when a lower-precedence installation can be overridden so that a trusted environment-variable reference is sent to another base URL.

Current targets:

| Agent | State |
| --- | --- |
| OpenCode | `contract_verified` |
| Kilo | `blocked` |
| CommandCode | `blocked` |

No state below `contract_verified` is a support claim.

Repository-pinned contract fixtures live under `docs/agents/fixtures/<agent>/`. Runtime export and validation must not fetch mutable schemas or package metadata.
