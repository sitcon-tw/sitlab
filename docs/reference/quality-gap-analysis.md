# Quality Gap Analysis

Project Template treats Netstamp's architecture as evidence, not as an instruction to copy every implementation choice. The following safeguards make the retained philosophy executable.

| Risk                                                 | Template safeguard                                                                                      |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| Layer names without enforced direction               | Go architecture tests reject forbidden imports.                                                         |
| Speculative repository abstractions                  | Ports are consumer-owned and introduced by a real use case.                                             |
| Transactions hidden in infrastructure                | Application commands declare atomic boundaries.                                                         |
| Generic errors losing client behavior                | TypeSpec defines status-specific problem unions and stable codes.                                       |
| Generated files silently drifting                    | CI emits into a temporary directory and byte-compares three artifacts.                                  |
| React server state copied into contexts              | State ownership is documented and tested by feature.                                                    |
| Domain workflows leaking into shared UI              | UI package is restricted to domain-neutral primitives.                                                  |
| Tokens bypassed by local CSS                         | A repository style check rejects raw colors and unsafe focus resets.                                    |
| Docs and Storybook omitted from aggregate validation | Both have explicit type/build jobs and participate in `just build`.                                     |
| Docker starts against an old schema                  | App startup depends on a successful one-shot migration job.                                             |
| Trace export becomes a runtime dependency            | The OTLP endpoint is optional; Prometheus metrics remain locally scrapeable.                            |
| Template renaming becomes global search/replace      | The one-time initializer validates identifiers and traverses only explicit source roots and extensions. |

Accepted tradeoffs are deliberate: PostgreSQL is the only database, the backend remains one deployable application, and the reference slice is small. Those constraints keep the template understandable while still exercising authentication, roles, transactions, list/detail UI, contract generation, and production deployment.
