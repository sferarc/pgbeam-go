# @pgbeam/go-sdk

## 0.2.1

### Patch Changes

- 31cb990: feat(byoc): self-host enrollment hardening, optional `expires_at` on enrollment create/list and a rotate operation that mints a new `pbh_` token once and atomically invalidates the old one

## 0.2.0

### Minor Changes

- d70bf02: Publish and document the Go SDK (`go.pgbeam.com/sdk`). The release pipeline now tags the public mirror at `v{version}` on a sentinel bump so `go get go.pgbeam.com/sdk@vX.Y.Z` resolves through the Go module proxy, and the docs now ship a full quickstart plus examples across the agent-gateway surface (agent credentials, policy profiles, approvals, webhooks, audit logs). Merging this changeset's release PR cuts the first tagged SDK version.
