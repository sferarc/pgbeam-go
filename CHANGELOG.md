# @pgbeam/go-sdk

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
