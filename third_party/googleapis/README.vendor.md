This directory vendors the upstream `google/api` subtree from the official
`googleapis/googleapis` repository.

Source repository:
- https://github.com/googleapis/googleapis

Vendored upstream commit:
- `208f19890d8e0a4a5bc772584246c973ff57f6c1`

Vendored scope:
- `google/api/**`
- upstream `LICENSE`

Why this exists:
- keep `google/api/annotations.proto` imports working without a Buf registry dependency
- make `buf generate` work in environments that cannot access `buf.build/googleapis/googleapis`

Refresh process:
1. Sparse-clone the upstream `googleapis/googleapis` repository.
2. Copy `google/api/` into `third_party/googleapis/google/api/`.
3. Copy the upstream `LICENSE` into `third_party/googleapis/LICENSE`.
4. Re-run this repo's Buf and Go verification commands.
