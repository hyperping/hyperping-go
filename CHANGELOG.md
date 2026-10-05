# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.0] - 2026-10-05

The client is now maintained by Hyperping.

### Changed

- **Module path is now `github.com/hyperping/hyperping-go`** (was
  `github.com/develeap/hyperping-go`). The API is unchanged: update the
  import path and run `go get github.com/hyperping/hyperping-go@v0.8.0`.
  The Develeap copyright headers (MIT) are kept; a `LICENSE` file now
  credits Develeap and Hyperping.

### Added

- **`Healthcheck.PublicUUID`** (`publicUuid`): the public `hc_…` id a
  status page references. Unlike `UUID` (the `tok_…` ping token), it is not
  a secret. `nil` when the API does not return it.
- **`Monitor.IPVersion`** (`ip_version`, `4` or `6`) on `Monitor`,
  `CreateMonitorRequest` and `UpdateMonitorRequest`, plus
  `AllowedIPVersions`.
- **TLS certificate and domain expiry alerting** on `Monitor`,
  `CreateMonitorRequest` and `UpdateMonitorRequest`: `SSLAlertDays`,
  `SSLReminders`, `SSLNotifyOnChange`, `DomainAlertDays`, and the read-only
  `Monitor.DomainExpiration`. All pointers, so an absent field stays `nil`.
  `AllowedSSLAlertDays` and `AllowedDomainAlertDays` list the accepted
  values.
- **`StatusPageService.Type`** (read-only): `monitor`, `healthcheck`,
  `server` or `component`; empty for a group header.

### Fixed

- **Healthcheck timezone lost on read.** `GET` and `PUT /v2/healthchecks`
  return the timezone as `tz`, `POST` returns it as `timezone`. A consumer
  reading `Healthcheck.Timezone` after a `GET` saw an empty string (in the
  Terraform provider: "timezone was Europe/Berlin, but now null" on a cron
  healthcheck). `Healthcheck` now decodes both keys into both fields (`tz`
  wins when both are present), and `GetTimezone()` prefers `tz`.

## [0.7.1] - 2026-06-09

### Fixed

- **MCP tools/call with nil args produced a malformed JSON-RPC request
  (CRITICAL).** `McpTransport.callToolOnce` omitted `params.arguments`
  entirely when the caller passed `nil`. The Hyperping MCP server's
  input validator rejected such requests with JSON-RPC `-32602
  Input validation` and returned the error as `content[0].text`
  starting with the literal string `MCP error -32602: ...`. That
  literal-text body then failed `json.Unmarshal` further down the
  decode path, surfacing to callers as
  `failed to parse MCP tool response: invalid character 'M' looking
  for beginning of value`. The fix normalizes nil args to
  `map[string]any{}` before serialization so the marshalled request
  always contains `"arguments":{}`. Probed against
  `https://api.hyperping.io/v1/mcp` on 2026-06-09 (sending `arguments:
  omitted` reproduced the failure; sending `arguments: {}` produced a
  valid response). The six affected `MCPClient` methods are:
  `GetStatusSummary`, `ListRecentAlerts`, `ListOnCallSchedules`,
  `ListEscalationPolicies`, `ListTeamMembers`, `ListIntegrations`.
- **`AlertHistory` decoded all-zero values silently (CRITICAL).** Even
  with the nil-args fix above, `ListRecentAlerts` populated all
  zero-value fields because the v0.7.0 `AlertHistory{Alerts, Total}`
  declaration did not match the server's actual response shape. The
  server returns `{timeGroups[], totalAlerts, downAlerts, upAlerts,
  rawAlerts[]}`. Downstream consumers reading
  `hyperping_total_alerts` got 0 fleet-wide regardless of true alert
  activity.
- **`TeamMember` dropped four live-server fields at decode time.**
  The v0.7.0 type declared `Role` and `Status`; the server returns
  `accountRole` (no `role`), no `status` at all, plus `phone`,
  `profilePictureUrl`, and `ssoPictureUrl`. Every previous decode
  produced an empty `Role` and `Status`.
- **`StatusSummary` dropped four live-server fields at decode time.**
  The v0.7.0 type declared `total/up/down`; the server also returns
  `paused`, `unknown`, `down_monitors[]`, and `paused_monitors[]`.
  Adding them does not invalidate existing fields, but consumers who
  needed the per-state breakdown were getting nothing.

### Changed (BREAKING)

- `AlertHistory` shape replaced:
  - Old: `{Alerts []Alert, Total int}` (neither key returned by server).
  - New: `{TimeGroups []AlertTimeGroup, TotalAlerts int, DownAlerts int,
    UpAlerts int, RawAlerts []Alert}` with a `Total() int` accessor that
    returns `TotalAlerts` for nil-safe migration of callers that used
    the old `.Total` field. Consumers that read `.Alerts` must migrate
    to `.RawAlerts`; the rename is intentional because the server's
    payload is a flat list, not a paginated alert page.
- `TeamMember` shape replaced:
  - Old: `{UUID, Email, Name, Role, Status}`.
  - New: `{UUID, Email, Name, Phone, ProfilePictureURL, SsoPictureURL,
    AccountRole}`. `Role` is renamed to `AccountRole` (matches the
    server field). `Status` is removed because the server never
    returned it. `SsoPictureURL` is `*string` so the JSON `null`
    returned for members who never signed in via SSO decodes as nil
    pointer, distinct from `""`.
- `StatusSummary` extended with `Paused int`, `Unknown int`,
  `DownMonitors []string`, `PausedMonitors []string`. Existing
  `Total/Up/Down` semantics are unchanged.
- SDK `clientInfo.version` reported during MCP initialize bumped from
  `0.7.0` to `0.7.1`.

### Added

- **Output-shape pinning in the schema-contract test.**
  `testdata/mcp_responses/*.json` snapshots the live `content[0].text`
  payload of every affected nil-args tool captured during this
  release. `schema_contract_test.go` round-trips each fixture through
  the corresponding `MCPClient` method's typed struct and asserts
  every top-level key in the fixture survives the decode. v0.7.0
  pinned only INPUT schemas; this catches RESPONSE-shape regressions
  that previously decoded silently to zero values.
- **Live integration tests for the six nil-args methods.**
  `integration_test.go` adds decode-success assertions for
  `GetStatusSummary`, `ListRecentAlerts`, `ListOnCallSchedules`,
  `ListEscalationPolicies`, `ListTeamMembers`, `ListIntegrations`.
  The integration suite now shares one MCP session across all
  tests (single `McpTransport` singleton) so the server's
  `initialize 5/min` rate-limit does not throttle a growing
  test surface.
- Unit-test pins for the new `AlertHistory`, `TeamMember`, and
  `StatusSummary` decode shapes, plus a regression test asserting
  `CallTool` with `nil` args produces a JSON-RPC body containing
  `"arguments":{}` and never `null` or absent.

## [0.7.0] - 2026-06-08

### Fixed

- **MCP windowed reporting tools were silently returning all-zero values
  (CRITICAL).** Four `MCPClient` methods sent the wrong request-arg name
  to the Hyperping MCP server AND decoded the response into structs
  whose fields the server never returned. Every existing caller saw a
  populated `*Response` with all-zero fields and no error. Affected
  methods, with the wire-level mismatch:

  | Method | v0.6.x request arg | server expects | v0.6.x response struct | server response shape |
  |--------|--------------------|----------------|------------------------|-----------------------|
  | `GetMonitorMtta` | `uuid` (string) | `monitor_uuids` ([]string), `from`, `to` | `MttaReport{AvgWait,MinWait,MaxWait,TotalAlerts,Acknowledged}` | `{monitors[], totalAcknowledged, mtta}` |
  | `GetMonitorMttr` | `uuid` (string) | `monitor_uuids` ([]string), `from`, `to` | `MttrReport{AvgResolve,MinResolve,MaxResolve,TotalOutages,Resolved}` | `{monitors[], totalOutages, totalOutagesLength, mttr, mtta}` |
  | `GetMonitorResponseTime` | `uuid` (string) | `monitor_uuids` ([]string), `from`, `to` | `ResponseTimeReport{UUID,Avg,Min,Max}` | `{timeGroups[], avgResponseTime, p95ResponseTime, monitors[]}` |
  | `GetMonitorUptime` | `monitor_uuid` (string) | `monitor_uuids` ([]string), `from`, `to` | `UptimeReport{Uptime,TotalDays,UptimeDays}` | `{monitors[], periodAverages[], totalOutages, totalOutagesLength, MTTR, averageUptime}` |

  Discovered during hyperping-exporter incident triage when the
  `hyperping_monitor_mtta_seconds` Prometheus series stayed at 0 across
  all 47 monitor labels for months. Probed against
  `https://api.hyperping.io/v1/mcp` on 2026-06-08 to confirm the actual
  wire formats above.

### Changed (BREAKING)

- `GetMonitorMtta`, `GetMonitorMttr`, `GetMonitorResponseTime`, and
  `GetMonitorUptime` migrated to a canonical signature:

  ```go
  func (c *MCPClient) GetMonitorMtta(ctx context.Context, from, to time.Time, uuids ...string) (*MonitorMttaResponse, error)
  ```

  Semantics:
  - Zero-value `from`/`to` omits the param so the server uses its declared
    default window (30 days at the time of release).
  - Empty `uuids` omits `monitor_uuids` entirely, which the server treats
    as "all monitors in the project."
  - ISO-8601 formatting for `from`/`to` is handled internally; callers
    always pass `time.Time`.
- Response types renamed and rewritten to match the server's actual shape:
  `MttaReport` → `MonitorMttaResponse`, `MttrReport` → `MonitorMttrResponse`,
  `ResponseTimeReport` → `MonitorResponseTimeResponse`,
  `UptimeReport` → `MonitorUptimeResponse`. Each adds typed per-monitor
  entry structs (`MonitorMttrEntry`, `MonitorResponseTimeEntry`,
  `MonitorUptimeEntry`, etc.).
- `MonitorResponseTimeEntry.AvgResponseTimeByRegion` uses `*float64` to
  preserve the server's null-vs-zero distinction; regions with no probes
  in the window decode as nil pointers rather than 0.
- `MonitorUptimeResponse.AverageUptime` is a string (the server returns
  `"100%"` at the top level) while per-monitor `AverageUptime` is a float.
  The typed asymmetry mirrors the server.
- SDK `clientInfo.version` reported during MCP initialize bumped from
  `0.6.2` to `0.7.0`.

### Added

- **Schema snapshot test (no live calls).** `testdata/mcp_tools_list.json`
  pins the live MCP server's `tools/list` response captured during this
  release. `schema_contract_test.go` iterates the wrapped tools and
  asserts every request-arg name the client sends is declared in the
  snapshot's `inputSchema.properties`. A targeted assertion guards the
  v0.6.3 bug specifically: each of the four windowed tools must declare
  `monitor_uuids: array`. Re-capturing the snapshot is a deliberate
  maintenance step so drift surfaces in code review, not silently.
- **Live-server integration tests.** `integration_test.go` (build tag
  `integration`) issues real `CallTool` requests for the four windowed
  tools and asserts decode-into-the-v0.7.0-models succeeds. Skips
  cleanly with `t.Skip` when `HYPERPING_TEST_API_KEY` is unset.
- CI workflow adds a separate `integration` job, guarded by the same
  env var, that PRs from forks skip transparently.

### Fixed (carried over from Unreleased)

- **HTTP/2 ALPN advertised without handler (CRITICAL).** v0.6.2's
  transport-bypass audit fix introduced `httpTransport.Clone()` in
  `enforceTLS`. The clone's `TLSClientConfig` inherits
  `NextProtos: ["h2", "http/1.1"]` from the source's lazy auto-init, but the
  clone's own auto-init early-returns when `TLSClientConfig` is custom,
  leaving an empty `TLSNextProto` map. Result: all HTTPS calls to
  non-localhost servers failed with `malformed HTTP response \x00\x00...`
  or `Unsolicited response received on idle HTTP channel`. Affects both
  REST (`hyperping.NewClient`) and MCP (`hyperping.NewMcpTransport`) paths.
  Localhost was unaffected (early-return path), which is why the SDK's
  own test suite did not catch it. Fixed by setting
  `cloned.ForceAttemptHTTP2 = true` after `Clone`, instructing Go's stdlib
  to auto-init h2 even when `TLSConfig` is custom. The existing TLS
  hardening (MinVersion TLS 1.2, AEAD cipher suites) is preserved.

### Migration notes for downstream consumers

The primary consumer is `develeap/hyperping-exporter`. Migration is
mechanical:

```go
// before (v0.6.x):
report, err := mcp.GetMonitorMtta(ctx, monitorUUID)
//   uses: report.AvgWait, report.TotalAlerts, ...

// after (v0.7.0):
now := time.Now().UTC()
resp, err := mcp.GetMonitorMtta(ctx, now.Add(-24*time.Hour), now, monitorUUID)
//   uses: resp.Mtta, resp.TotalAcknowledged, resp.Monitors[*].Mtta
```

The exporter's existing 47-series `hyperping_monitor_mtta_seconds` will
begin emitting real (non-zero) values after the migration.

## [0.6.2] - 2026-05-31

### Security

- MCP transport now caps decoded response bodies at `maxResponseBodyBytes` (10 MB, matching the REST path). `decodeJSONRPCResponse` performs a two-stage check: a `Content-Length` pre-flight that rejects oversize payloads without reading any body bytes, plus a post-read length check for chunked / unknown-length servers. A malicious or compromised MCP server can no longer stream multi-gigabyte JSON through the decoder and exhaust client memory.
- `WithMCPHTTPClient` now re-wraps the caller's transport through the auth + TLS chain after options run. Without the rewrap a caller's default `*http.Client` would lose `defaultTLSConfig` (TLS 1.2+ floor and AEAD cipher restrictions) and the `authTransport` wrapper, opening a downgrade-to-cleartext path that could leak the manually-attached Bearer token.
- `sanitizeMessage` Bearer redaction now preserves RFC 6750 challenge parameters. Previously `Bearer realm="api"`, `Bearer error="invalid_token"`, and `Bearer scope="read"` were all over-redacted to `Bearer ***REDACTED***`, hiding legitimate `WWW-Authenticate` diagnostics. The matcher now captures the post-Bearer token via `ReplaceAllStringFunc` and short-circuits when the token starts with `realm=`, `scope=`, `error=`, `error_description=`, or `error_uri=`. Ambiguous short opaque tokens (e.g. literal "Cookie") remain redacted by design.
- Bearer redaction floor stays at 6 non-whitespace characters so letters-only session ids and short opaque tokens still redact. Five-character placeholders pass through unchanged for documentation / example use.
- MCP `Retry-After` parsing now routes through `parseRetryAfter`, which clamps the returned wait at `maxRetryAfterSeconds`. A hostile server can no longer return `Retry-After: 86400` and force a 24-hour wait on callers that respect the value.
- `WithBaseURL` and `NewMcpTransport` reject base URLs that embed userinfo (`https://user:pass@host`). Embedded userinfo previously bypassed the Bearer auth contract: the URL credential would be sent in addition to the API key, possibly to the wrong tenant.
- `initializeAttempt` publishes the `(sessionID, initialized)` pair under `initMu` so the 404-clear path in `callToolOnce` cannot interleave between the two writes and leave the transport in a torn state.
- `enforceTLS` now clones the caller-supplied `*http.Transport` before applying TLS minimums, instead of overwriting the caller's `TLSClientConfig` in place. Custom `RootCAs`, `ServerName`, and client-cert configuration on the caller's transport are preserved on the clone; the original is untouched.

### Changed

- `Initialize` (public method on `*McpTransport`) now runs under the same stampede guard as `ensureInitialized`. Concurrent direct callers see exactly one initialize handshake hit the server; the remaining callers short-circuit on the published `initialized` flag. No public API change, but worker pools that previously pre-initialized in parallel will now observe a single server-side handshake.
- `WithBaseURL` (REST) and `NewMcpTransport` (MCP) reject base URLs containing userinfo. Callers that relied on `https://user@host` must move credentials into headers (the Bearer API key path is the intended channel). Breaking for any caller that used embedded userinfo.
- `WithMCPHTTPClient` rewraps the caller's `*http.Transport` through `buildTransportChain` after options run. Callers that supplied a custom `*http.Transport` hoping to disable our TLS minimums or auth injection will observe their transport wrapped, not bypassed. The caller's original `*http.Transport` is not mutated; the rewrap operates on a clone (silent override of caller-supplied behavior, by design).
- Initialize publish step (`sessionID` + `initialized`) is atomic against the 404-clear path.

### Notes

- Two stdlib vulnerabilities surfaced by `govulncheck` remain unpatched in `go.mod`: GO-2026-4971 (net dial NUL-byte panic on Windows) and GO-2026-4918 (HTTP/2 SETTINGS_MAX_FRAME_SIZE infinite loop). Both fix in Go 1.26.3; a toolchain bump is tracked separately.

## [0.6.1] - 2026-05-31

### Security

- `sanitizeMessage` (the redactor invoked by `APIError.Error()`) now covers credential-bearing headers beyond Bearer. `Authorization:` is redacted for Bearer, Basic, Digest, AWS SigV4, and any custom scheme; the terminator was widened from "stop at comma" to "stop at line break" so multi-field schemes (e.g. Digest `response="<hash>"`, SigV4 `Signature=...`) no longer leak the credential past the first comma. Added explicit patterns for `Cookie`, `Set-Cookie`, `Proxy-Authorization`, `X-Api-Key`, `X-Auth-Token`, and `X-Access-Token`. Tradeoff: a single log line packing multiple headers separated by commas is over-redacted (only the first matching header survives), which is the intended bias for a security primitive.

### Notes

- Backward compatible. Callers that pattern-match on the previous `Authorization: Bearer <token>` redacted form should check for the broader `Authorization: ***REDACTED***` shape, which now applies regardless of scheme.

## [0.6.0] - 2026-05-24

### Added

- `ListOutages` now accepts functional options. `WithStatus("all"|"ongoing"|"resolved")` adds the corresponding `status` query parameter so callers can let the Hyperping API filter outages server-side instead of paginating through the full history. The no-option call is unchanged and omits the parameter entirely.

## [0.5.0] - 2026-05-21

### Fixed

- MCP transport now captures `Mcp-Session-Id` from the `initialize` response and echoes it on every subsequent JSON-RPC request, per MCP 2025-03-26 Streamable HTTP spec. Servers that enforce session ownership (Hyperping among them) were previously treating every `tools/call` as a sessionless fresh attempt and rate-limiting them as `initialize`-bucket violations. Fixes hyperping-exporter#60.

### Added

- One-shot session-loss recovery: when the server returns HTTP 404 to a request that carried a session id, the transport clears its local session state, re-initializes once, and retries the failing call. The existing init mutex serializes concurrent recovery so a worker pool does not stampede the server with parallel initialize attempts.
- `ErrSessionLost` sentinel error so callers can match the recovered-too-late case via `errors.Is`.

### Notes

- Backward compatible: servers that do not issue `Mcp-Session-Id` continue to work without sending the header.

## [0.4.0] - 2026-04-25

### Breaking Changes

- `NewMcpTransport` now returns `(*McpTransport, error)` instead of `*McpTransport`. Callers must handle the returned error. The constructor validates the base URL (must be HTTPS or `http://localhost`) before returning.

### Changed

- MCP transport now uses `buildTransportChain` to enforce TLS 1.2+, AEAD cipher suites, and connection pooling (matches the REST client).
- `Initialize` uses mutex-based double-checked locking to prevent concurrent goroutines from racing into the handshake.
- All JSON unmarshal errors in `mcp_client.go` are now propagated to callers instead of being silently discarded.
- Unchecked type assertions in MCP response parsing replaced with comma-ok checks that return descriptive errors.

### Fixed

- HTTP response bodies are now drained before close on error paths, preventing connection pool exhaustion.
- `context.Done()` is respected in retry sleep loops, allowing in-flight MCP calls to cancel promptly.

### Dependency

- Bumped `github.com/stretchr/testify` from 1.3.0 to 1.11.1.

## [0.3.0] - 2026-04-22

### Added

- MCP transport (`McpTransport`): JSON-RPC 2.0 client for the Hyperping MCP server with retry logic on 5xx errors and error mapping to typed error types.
- MCP client (`MCPClient`): high-level typed wrapper exposing all 30 MCP tools (25 read, 5 write).
- `mcp_models.go`: typed structs for all MCP responses (`StatusSummary`, `ResponseTimeReport`, `MttaReport`, `MttrReport`, `MonitorAnomaly`, `ProbeLogResponse`, `AlertHistory`, `OnCallSchedule`, `EscalationPolicy`, `Integration`, and more).

## [0.2.1] - 2026-04-14

### Fixed

- `parseOutageListResponse` and `parseMaintenanceListResponse` now preserve `HasNextPage` from the API response instead of always returning `false` on empty pages.
- Circuit breaker timeout increased from 5s to 30s to match realistic API response times.

## [0.2.0] - 2026-04-10

### Added

- Circuit breaker via `gobreaker` to protect against cascading failures on repeated API errors.
- Retry with exponential backoff on 5xx responses across all REST client methods.
- Pagination helpers: `HasNextPage` field propagated correctly through list responses.

## [0.1.0] - 2026-04-01

### Added

- Initial release with full REST API client for Hyperping.
- Resources: monitors, incidents, outages, maintenance windows, escalation policies, on-call schedules, integrations, status pages.
- Error types: `HyperpingError`, `NotFoundError`, `AuthError`, `RateLimitError`, `ValidationError`.
- `validateBaseURL`, `IsNotFound`, `IsRateLimit` helpers.
