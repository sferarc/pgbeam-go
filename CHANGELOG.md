# @pgbeam/go-sdk

## 0.8.2

### Patch Changes

- 1d62792: Add `grantAgentCredentialForOAuth`, the dashboard's call to mint or reuse the agent credential a database MCP OAuth grant runs as.

## 0.8.1

### Patch Changes

- 2981e78: Declare `Idempotency-Key` on budget top-ups, organization invitations, batch and cancellation feedback. A top-up's key is scoped to its agent token, and a `402` is no longer stored, so a paid retry under the key that met the challenge runs instead of replaying it.
- 61fbe14: Drop the internal Slack support event operation, its `SlackEventPayload` schema and the `InternalSecret` scheme from the public spec and SDKs. It is a dashboard-to-API call no customer can make.

## 0.8.0

### Minor Changes

- c85e41f: Policy profiles carry `function_denylist`, a list of functions an agent on the profile may not call. It adds to the proxy's built-in dangerous-function floor and is refused with decision rule `function_denied`. Entries are a bare name (`recompute`, matched in every schema) or one schema-qualified name (`billing.recompute`, which also matches an unqualified call of that name). On update, an omitted `function_denylist` keeps the stored list; send `[]` to clear it. While the organization has a live self-hosted data plane enrollment, a non-empty list is refused with 409, and a new enrollment is refused while any profile has one, because an older self-hosted proxy would ignore the list. In Terraform, removing `function_denylist` from a config plans an empty list.

## 0.7.0

### Minor Changes

- cf9b283: Add `overage_per_1k` and `overage_per_gb` to `PlanLimits`: the organization's own overage prices in USD per 1,000 queries over the daily quota and per GB over the monthly quota. A value of 0 means the quota is a hard cap and no overage is billed.

## 0.6.0

### Minor Changes

- adb5b67: Add the `last4` and `email_domain` partial mask kinds to masking rules. `last4` returns `****` plus the value's last four characters, and `email_domain` returns `****@` plus the domain. A value too short for `last4`, or one whose text after its last `@` is not a hostname or bracketed IP address under `email_domain`, is redacted instead.

## 0.5.1

### Patch Changes

- 0cf6810: refactor(policy): serve statement-kind verdicts from the engine to the CLI card

## 0.5.0

### Minor Changes

- 796f4a7: Policy profiles carry `query_mode` (`sql` or `templates`). A profile set to `templates` admits only the project's enabled query templates, and every other rule on the profile still applies to what a template runs. Setting `templates` is refused when the project has no enabled template, and while the organization has a live self-hosted enrollment. On update, an omitted `query_mode` keeps the stored mode. In Terraform, Crossplane and Pulumi, removing `query_mode` from a config plans `sql`, and the plan shows that change.

## 0.4.1

### Patch Changes

- 5dbe64b: `DryEvalResult` gains an optional `template` field: the name of the project query template a statement matched, present only when the policy is templates-only.
- d651ca6: Query templates can be declared as code: `pgbeam_query_template`, the Crossplane `QueryTemplate` and the Pulumi `QueryTemplate`. Removing `description`, `include_sql`, `enabled` or `params` from a config resets it to the API default (empty, false, true, none) rather than keeping the last value. The query template input's `description` now declares its default (empty) in the contract, which is what the API already stored for an omitted one. Nothing restricts a credential to templates yet: the policy profile setting that does arrives in a later release.

## 0.4.0

### Minor Changes

- 307db3d: Path parameters are now escaped with `url.PathEscape`, so an ID containing `/`, `?`, `#` or `%` can no longer rewrite the request path or query.

  Breaking signature changes:

  - `AgentsService.ExportAuditLogs` now returns `([]byte, error)` with the CSV body, and sends its `params` filters. It previously returned only `error`, ignored `params` and discarded the export.
  - `WebhooksService.TestWebhookEndpoint` now returns `(*WebhookTestResponse, error)` instead of discarding the `202` response.

  `SchemaannotationsService.DeleteSchemaAnnotation` now sends its `params` (`table_name`, `schema_name`, `column_name`) instead of dropping them.

- f17bb94: Bound every call the way the TypeScript and Python SDKs do. Each attempt now times out after `ClientOptions.Timeout` (default 30s, negative disables), and `RetryConfig.TotalBudget` (default 120s) stops retrying once the next wait would land past it, so a `Retry-After: 3600` returns the error instead of sleeping an hour. A truncated response body is retried like a network error, error bodies are read up to 1 MiB, a negative `MaxRetries` makes one attempt, and the User-Agent carries the SDK version.
- f363bd7: Add the query templates API: create, list, get, update and delete named, parameterized statements on a project (`/v1/projects/{project_id}/query-templates`, under the Policies tag). A template is checked when it is written and refused with 400 if it is not exactly one SELECT, INSERT, UPDATE or DELETE whose `$n` placeholders match its declared parameters. Nothing restricts a credential to templates yet: the policy profile setting that does arrives in a later release.
- 94466c2: Remove the internal Slack support webhook (`POST /v1/internal/support/slack-event`) from the public SDKs and their docs. It only accepts calls from the dashboard's Slack handler, so `api.internal.handleSlackSupportEvent` (TypeScript), `client.Internal.HandleSlackSupportEvent` and the `SlackEventPayload` type (Go), and `client.internal.handle_slack_support_event` (Python) are gone. This removes public methods, so it would be a breaking change after 1.0; while the SDKs are pre-1.0 it ships as a minor release.

### Patch Changes

- 86e67c7: Query parameters now encode timestamps as RFC 3339, so `start`, `end` and `before` filters on audit logs reach the API in a form it accepts. Header parameters such as `If-None-Match` and `Idempotency-Key` are no longer leaked into the query string, and an explicitly set pointer to `0` or `false` is sent instead of dropped. Slice parameters are encoded as repeated keys.
- ba058a7: Describe the management MCP endpoint as it behaves: six registered tools, with API operations reached through `call_endpoint`, and the protocol version negotiated at `initialize` rather than pinned.
- 421ec09: Parse RFC 9457 problem documents into `ApiError`. The message now comes from `detail` (then `title`) instead of falling back to the bare status text, and `code`, `type`, `title`, `detail`, `instance`, `request_id` and `errors` are attributes, so callers can tell a 403 `FORBIDDEN` from a 403 `PLAN_LIMIT_REACHED` without reading `body`.
- 6cd15fe: Stop sending an automatic `Idempotency-Key` on `POST /v1/scan/text`, which the API rejects with 400. Retries stay on for that call, since a scan has no side effects.

## 0.3.7

### Patch Changes

- 14df323: Approval rules can be authored: five operations under `/v1/projects/{project_id}/approval-rules` and `pgbeam approvals rules {list,create,show,update,delete}`.

## 0.3.6

### Patch Changes

- eb77c5f: `listAnomalyAlerts` takes an optional `kind` filter, one of the new `AnomalyKind` enum values.
- 9d6807d: The audit session summary carries `anomalies`: the alerts raised from the session's own entries, and separately the rate and shape alerts on its credentials whose window overlaps it. The field is absent when the caller's role does not hold `anomaly:read`. `pgbeam audit session` prints both lists.
- d92dcdb: Reword five API descriptions that carried em dashes into every generated reference.

## 0.3.5

### Patch Changes

- b821f90: Approval requests carry `approval_rule_name`, the approval rule that held the statement.

## 0.3.4

### Patch Changes

- 16ed7f3: feat(proxy): content_scan_mode=block now blocks, and it does it out loud (roadmap item 7, rung 3)

## 0.3.3

### Patch Changes

- cb41f54: Publish the Go SDK surface for anomaly rules, which #2357 generated into `providers/pgbeam-go/` without moving the version sentinel. `go.pgbeam.com/sdk v0.3.2` predates `AnomaliesService.GetAnomalyRule`, `AnomalyRuleInput` and `AnomalyMetric`, so a provider that consumes them cannot build against the published module.

## 0.3.2

### Patch Changes

- 3342c65: feat(policy): content_scan_mode and content_scan_max_bytes on the policy profile

## 0.3.1

### Patch Changes

- 022577d: feat(api): the audit log handed out a cursor with nowhere to put it, so a generated client could not page it at all
- 3f33063: fix(sdk,cli): read the API's RFC 9457 problem documents

  `ApiError` now exposes `code`, `type`, `title`, `detail`, `instance`, `requestId` and `errors`, and its `message` comes from the document's `detail` rather than falling through to the status text. Branch on `code`: two conditions can share a status, and a 403 is either a permissions problem or a billing one. The CLI puts the code on the error line, lists field errors under it, and carries both in `--json` output.

## 0.3.0

### Minor Changes

- 43acb8e: fix(api): the member API contract claimed two things the server refuses

  `UpdateOrgMemberRoleRequest.role` and `CreateOrgInvitationRequest.role` now use a new `AssignableOrgRole` enum, which is `OrgRole` without `owner`. The server has always rejected `owner` on both paths with a 400, so every generated client, the CLI help and the reference pages were advertising a call that never works. Responses keep the full `OrgRole`, because a member really can be an owner. `role` is also no longer required on an invitation, matching the server, which defaults an absent role to `member`.

  `listOrgMembers` and `listOrgInvitations` now enforce the `page_size` range they declare. They read the query string directly and clamped anything outside 1 to 100 back to the default of 20, so `?page_size=500` was a documented 400 everywhere else in the API and a silent 20 here.

  `listOrgInvitations` now enforces the `status` enum it declares. Anything outside `pending`, `accepted`, `rejected` and `canceled` went to the database as a literal filter and came back as an empty page with a 200, so a caller who mistyped the status was told the organization has no invitations. It is a 400 now, as the contract has always said. `?status=` with no value is also a 400 rather than the unfiltered list; omit the parameter to list everything.

  `removeOrgMember`'s published description said "An owner cannot be removed; demote them first", and the server does neither half of that. It refuses only when the organization is down to its last owner, so removing any other owner returns 204, and demoting the last owner hits the same guard and returns the same 409. The description now says what the guard does: the last remaining owner cannot be removed or demoted, so transfer ownership first, which is the advice the 409 itself gives.

  Go SDK callers: nothing to migrate. `OrgRole` and `AssignableOrgRole` are both new types in `go.pgbeam.com/sdk` as of this release, which is what the minor bump is for.

## 0.2.2

### Patch Changes

- 06f9609: feat(cli): first-run golden path. New public `GET /v1/organizations` lists the organizations visible to the caller's credential (an org-scoped `pbo_` key sees exactly its org, a user credential sees memberships with roles). `pgbeam auth login` now verifies the key against the API before storing it (a rejected key fails the login and stores nothing) and resolves the organization automatically, auto-selecting a single org and prompting a pick among several. `orgs list` shows live organizations with the active one marked (falling back to saved profiles offline) and `orgs switch` with no argument lists and picks interactively. `auth status`/`whoami` verify the credential live when online and print the masked key, method, email, and org, degrading gracefully offline; `whoami --help` now shows its own name. Top-level `pgbeam link` and `pgbeam unlink` aliases are registered so every hint that references them works, and the project link is discovered by walking ancestor directories like git. `policies create` gains the write-safety flags `update` already had (`--write-mode`, `--approval-mode`, `--approval-timeout-seconds`, `--approval-auto-max-rows`, `--migration-safety`, `--table-allowlist`, `--table-denylist`). The "No organization set" error now names the exact dashboard location to copy an org ID, the `mcp --help` example shows the real `.mcp.json` stanza, and `agents mcp-config` explains all three ways to supply credentials when input is missing.

## 0.2.1

### Patch Changes

- 31cb990: feat(byoc): self-host enrollment hardening, optional `expires_at` on enrollment create/list and a rotate operation that mints a new `pbh_` token once and atomically invalidates the old one

## 0.2.0

### Minor Changes

- d70bf02: Publish and document the Go SDK (`go.pgbeam.com/sdk`). The release pipeline now tags the public mirror at `v{version}` on a sentinel bump so `go get go.pgbeam.com/sdk@vX.Y.Z` resolves through the Go module proxy, and the docs now ship a full quickstart plus examples across the agent-gateway surface (agent credentials, policy profiles, approvals, webhooks, audit logs). Merging this changeset's release PR cuts the first tagged SDK version.
