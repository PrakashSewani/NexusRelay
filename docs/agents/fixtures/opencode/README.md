# OpenCode Contract Fixtures

These repository-pinned fixtures define the deterministic NexusRelay export contract for `opencode-ai@1.18.5`. They are test inputs and expected artifacts, not user configuration to install in a project or global config file.

## Provenance

| Item | Value |
| --- | --- |
| Package | `opencode-ai@1.18.5` |
| Release commit | `2b2aacc93975330f9fd045d4306f698b0c6a8f8f` |
| npm integrity | `sha512-Q0jlX4ihn7veMeYsLX3c4PYFAKIURU3GIpXt1FnhNxNn3v8+RpIZ8z9umG5D0r8g8Smp9fZLGjgLe/9mJ4NyYw==` |
| npm SHA-1 | `91dcee1ca87ac6f445b4fbf7a3375de170acbfe6` |
| Schema URL | `https://opencode.ai/config.json` |
| Schema retrieved | `2026-07-26` |
| Retrieved schema SHA-256 | `8ffffc8622f2bbee5e9b1e57bf2509910f2a6dfc237458766bfaa5e295787a2e` |

Runtime generation and blocking tests must not fetch the schema URL. These fixtures pin the accepted provider subset and security behavior. A separate drift check may retrieve the current schema for review.

## Fixture Set

| File | Purpose |
| --- | --- |
| `minimal.json` | Smallest supported provider with one selected model. |
| `capabilities.json` | Guaranteed limits and capabilities that the verified schema can represent. |
| `omissions.json` | Unknown metadata omitted rather than guessed or emitted as false/zero. |
| `project-override-threat.json` | Lower-precedence malicious project entry used to prove static/global installation is unsafe. |

Validation must parse strict JSON, compare semantic shape and deterministic serialization, assert the exact `{env:NEXUSRELAY_API_KEY}` reference, and reject plaintext-key sentinels. The threat fixture is not a valid generated artifact; tests combine it only to demonstrate why the supported invocation sets `OPENCODE_DISABLE_PROJECT_CONFIG=1` and supplies the generated fragment through `OPENCODE_CONFIG_CONTENT`, the highest user/project-controlled merge in the pinned source. Resolved-configuration smoke tests separately reject later trusted administrative overrides that alter the generated provider entry.

Fixture SHA-256 values are recorded in `SHA256SUMS` and must be updated deliberately with profile review.
