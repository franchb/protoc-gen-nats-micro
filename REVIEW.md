# Code Review — Findings & Open Items

A full-repo multi-agent review (2026-06-12) of the generator core, all four
template trees, and examples. ~30 correctness bugs and ~15 cleanups were
confirmed (most reproduced empirically with protoc + go build / tsc / py_compile)
and **fixed in the `code-review` branch**. This file tracks what was deliberately
NOT fixed, so future sessions can pick items up with full context.

## Already fixed (summary, for orientation)

- TS streaming server was entirely nonfunctional (`(msg as any)._nc` /
  `msg.respondWith?.nc` don't exist on ServiceMsg) → Handlers now receive `nc`.
- Duplicate client-streaming handler blocks in `ts/service.ts.tmpl` and
  `python/service.py.tmpl` (double `add_endpoint` in Python) → first block deleted.
- TS client streaming receive never woke (`recvResolve` closure vs field) and
  `recv()` poisoned the nats.js iterator (`already yielding`) → shared resolver
  holder + persistent iterator.
- Python streaming dead on arrival (sync lambda `cb=`, lowercase `false`/`true`
  from `{{UseJSON}}`, bidi ack published to the wrong subject, missing
  `_WithJetStream`/`logging` imports) → all repaired.
- Unary errors from TS/Python services were decoded as SUCCESS by every client
  (`Status`/`Description` vs `Nats-Service-Error-Code`) → services now emit the
  standard NATS micro headers; `docs/guide/error-handling.md` corrected.
- Timeout literal corruption (`0000` octal in TS, `0.5.0` in Python,
  `0.5 * time.Second` Go compile error) → `ToGoDuration`/`ToMillis`/`ToPySeconds`
  FuncMap helpers.
- Go header/skip mismatches (unused `os`/`strconv`/`sync`/`io` imports,
  header-only files, undefined chunked stream types) → skip-aware import scan +
  `$needsOS`, whole-file skip, skip-aware chunked templates.
- Root-level protos placed the shared file in a spurious subdirectory
  (`lastSlash > 0` bug) → `path.Dir`/`path.Join`; `shared.proto` collisions now
  fail with a clear message.
- Multi-service protos generated duplicate TS imports/classes/consts → file-scope
  declarations moved to header templates.
- Go client-streaming subscribed to the reply inbox only inside `CloseAndRecv`
  (server errors/fast responses silently lost) → subscribe before handshake.
- Python chunked downloads committed truncated files on timeout
  (`except Exception: raise StopAsyncIteration`) → errors propagate, temp file
  unlinked.
- TS chunked receive hardcoded the `data` field; Python chunked helpers were
  last-method-wins → chunk field threaded through receiver construction.
- `json=true` for TS targets and non-UPPER_SNAKE_CASE `error_codes` now fail at
  generation time instead of generating broken/non-interoperable code; key
  templates reject characters that corrupted generated expressions; Go key
  getters resolved from descriptors (`id_2` → `GetId_2`).
- Streaming handlers no longer serialize an endpoint (Go: `go func`, Python:
  `asyncio.create_task` + retained task set).
- KV: bucket handles cached at registration (was one STREAM.INFO RTT per
  request); LWW writes use plain `Put` when no key-TTL; Python CAS catches
  `KeyWrongLastSequenceError` instead of blanket `Exception`;
  `(natsmicro.stream).ordered` wired (was parsed-but-ignored).
- Duplicated `options.pb.go` under `tools/` deleted (latent proto
  double-registration panic + drift channel); generator imports
  `gen/nats/micro`; Taskfile `cp` sync removed.
- Stale checked-in `examples/streaming-go/gen` regenerated (shipped a known
  subscription leak); module tidied.
- Dead code removed (`ToKebabCase`, `GetInputFields`,
  `IsKVWriteModeLastWriteWins`, dead FuncMap entries, unreachable `-lang` flag);
  `GetEndpointOptions` memoized; path derivation deduplicated
  (`OutputFilenamePrefix`/`ImportPathFor`); unknown plugin parameters now error.

## Open items (not fixed — future sessions)

### 1. TS targets: incompatible nats.js type imports (pre-existing, surfaced by tsc)

`tsc --noEmit` against real `nats@2.29.3` + `@nats-io/services` shows 16 (ts) /
4 (web-ts) pre-existing errors in generated output, none introduced by the fixes:

- `Service`/`ServiceConfig`/`ServiceError` are imported from `@nats-io/services`
  (v3 ecosystem) but used with nats v2's `nc.services.add(...)` — the two
  libraries declare incompatible twin types (`ts/header.ts.tmpl` line ~4,
  service register + wrapper in `ts/service.ts.tmpl`).
- Wrapper `isStopped(): boolean` returns `service.stopped`, which is
  `Promise<null | Error>` in both libraries; `stop(): Promise<void>` actually
  returns `Promise<null | Error>`.
- Client factories read `nc.options.inboxPrefix`, which isn't on the
  `NatsConnection` interface (currently cast via `(nc as any)`); should use
  `createInbox()` from the library instead.

**Decision needed:** target ONE services API — either import service types from
`nats` v2 and drop the `@nats-io/services` import, or migrate generated code to
the v3 ecosystem (`@nats-io/transport-node` + `@nats-io/services`). Then fix
`isStopped`/`stop` signatures and inbox creation.

### 2. web-ts is a 95% copy of ts (and silently overwrites it)

- `templates/web-ts/errors.web-ts.tmpl` is byte-identical to
  `templates/ts/errors.ts.tmpl` except one comment line; client overlaps ~93%,
  shared ~97%. Every TS fix must be hand-mirrored — this review already had to
  apply several fixes twice.
- Both languages use FileExtension `_nats.pb.ts`, so generating ts and web-ts
  into the same out dir silently overwrites whichever ran first.

**Suggested fix:** model web-ts as a parameterized variant of the TS language
(codec/runtime imports + an emit-server flag; `newBaseLanguage` already supports
per-language template lists), or share blocks via `{{define}}` across both
trees. Give web-ts a distinct extension (e.g. `_nats.pb.web.ts`) or make the
collision an error.

### 3. Editions protos unsupported

`main.go` declares only `FEATURE_PROTO3_OPTIONAL`; protoc aborts on
`edition = "2023"` files with "plugin does not support editions". The generator
only consumes services/methods/field names, so support is likely cheap:
declare `FEATURE_SUPPORTS_EDITIONS` + `SupportedEditionsMinimum/Maximum`
(mirror protoc-gen-go) and add an editions proto to the test corpus.

### 4. Per-key TTL parity impossible for Python/TS KV

Go honors `(natsmicro.kv_store).key_ttl/purge_ttl` via `jetstream.KeyTTL`/
`PurgeTTL`; nats-py ≥2.7 and nats.js v2 KV APIs expose no per-key TTL, so
Py/TS-written keys never expire while Go-written ones do.
**Suggested fix:** emit a generation-time warning (or error) when
`key_ttl`/`purge_ttl` is set for python/ts/web-ts targets, and document the gap.
Revisit if/when client libraries gain per-key TTL.

### 5. `(natsmicro.stream).max_inflight` still a silent no-op

`ordered` is now wired through to the stream receivers, but `max_inflight` is
parsed into `StreamOpts.MaxInflight` and read by nothing. Either implement
backpressure in the stream helpers (natural pairing: the now-concurrent stream
handlers — goroutine/task per stream — currently have unbounded per-endpoint
concurrency) or reject/document the option.

### 6. `x_chunked.proto` filename collision

`shared.proto` collisions now fail with a clear message, but a proto literally
named `x_chunked.proto` next to `x.proto` (with a chunked client-streaming
method) still collides with the generated `x_chunked_nats.pb.go` and dies with
protoc's cryptic "Tried to write the same file twice" (reproduced). Add the
same pre-flight detection for the chunked suffixes in
`GoLanguage.GenerateExtraFiles` or `main.go`.

### 7. CI never regenerates or builds examples / generated-output syntax checks

The drift that shipped a known subscription leak in
`examples/streaming-go/gen` was invisible because CI only builds the root
module and runs generator tests. Add CI steps:

1. Build the plugin, `buf generate` the streaming example, fail on `git diff`.
2. `go build ./...` in `examples/streaming-go`.
3. Generate TS/Python outputs for a rich test proto and run `tsc --noEmit` /
   `python3 -m py_compile` — this class of check would have caught most of the
   30 bugs in this review at introduction time.

Also resolve the contradiction that `.gitignore` lists
`examples/streaming-go/gen/` while the three files there are tracked
(`git ls-files` shows them): either drop the ignore line or untrack the files
and rely on CI regeneration.

### 8. Skip filtering still re-implemented at 3 altitudes

`GenerateFile` skips services (and now whole files), `hasClientStreamingChunkedIO`
re-checks, and ~28 `{{if not $opts.Skip}}` guards across 8 templates re-filter
methods. One forgotten guard silently emits code for a skipped endpoint.
**Suggested fix:** pass a pre-filtered method list (with pre-computed options)
through `TemplateData` and delete the per-template guards.

### 9. Per-language resolvers live in the global FuncMap

`ResolveKeyTemplateGo/TS/Py` are registered for all four languages' template
sets — a ts template can call the Go resolver and it compiles. Move language-
specific funcs to a per-language overlay (extra `Funcs` arg to
`newBaseLanguage`), so adding a language stops requiring edits to shared infra.

### 10. Optional: defensive error-header parsing in unary clients

Services now emit the standard `Nats-Service-Error-Code`/`Nats-Service-Error`
headers, but services generated by ANY pre-fix version of this plugin still
emit `Status`/`Description`. The streaming receivers already accept both
conventions; unary clients accept only the standard one. Decide whether a
compatibility window matters; if so, have unary clients fall back to
`Status`/`Description` like the stream paths do.

### 11. Protoopaque chunked-helper caveat (document)

`*_chunked_protoopaque_nats.pb.go` compiles only when message code is generated
with the opaque/hybrid API (`SetData` setters); with default open-struct
messages only the `!protoopaque` twin compiles. Inherent to the build-tag pair
design — document it in API.md.

### 12. Docs sweep for stale behavior claims

`docs/guide/error-handling.md`'s wire-format table was corrected, but other
docs (README, API.md, PYTHON.md, TYPESCRIPT.md, docs/) were not audited against
the new behavior: timeout option semantics (now rendered via duration helpers),
the removed `-lang` flag, key-template character restrictions, `json` option
unsupported for TS, error-code UPPER_SNAKE_CASE requirement, unknown plugin
parameters now erroring. Note: `options_test.go` asserts specific API.md
wording about KV semantics — keep that test in sync when editing API.md.

## Open items from the PR #10 bot review (triaged 2026-06-14)

CodeRabbit/Gemini reviewed PR #10. Eight confirmed bugs were fixed in this
branch (Go `ToMillis` sub-ms truncation; error-code identifier collisions like
`FOO_BAR`/`FOO__BAR`; Python client-streaming upload iterator swallowing
timeouts as clean EOF; TS `recvToFile` double-close masking the real error; TS
unary `options?.timeout || …` dropping an explicit `0`; TS bidi server-inbox
collision under `inboxPrefix`; Go streaming goroutines crashing the process on a
handler panic; TS bidi `recv` leaking a stale resolver on timeout). The three
below were verified as real but deliberately deferred.

### 13. Streaming goroutines are not tied to service shutdown

The per-stream goroutines now recover from panics (no more process crash), but
they still derive `ctx` from `context.Background()` (overridden only by an
explicit timeout) and are not tracked/drained on shutdown — in-flight streams
can outlive `service.Stop()`. The handler struct has no shutdown context or
`WaitGroup`, so this needs a service-lifecycle addition (shutdown `ctx` +
goroutine tracking), not a local template tweak. (CodeRabbit, go/service.go.tmpl)

### 14. Server-streaming / bidi handler failures aren't surfaced to the client

When a server-streaming or bidi handler raises (non-panic) in Python, the outer
`except` only `logging.error`s — no end-of-stream/error frame is sent, so the
client hangs until its own timeout (server-streaming also skips `sender.close()`
on the error path). The client-streaming handler already publishes an error to
`Reply-To`; the gap is server-streaming + bidi (Python, and the TS equivalents).
Fix needs an error-frame on close for those paths. (CodeRabbit,
python/service.py.tmpl, ts/service.ts.tmpl)

### 15. Duplicated per-service KV `put` helper in generated Go

`put{{Service}}KVValue` (service.go.tmpl) and `put{{Service}}ClientKVValue`
(client.go.tmpl) are byte-identical but differently named, so both land in the
generated file when client+server are emitted. No collision/compile error — pure
duplication. Dedup would mean emitting one shared helper (e.g. in
`shared_nats.pb.go`), a generation-model change. (Gemini)
