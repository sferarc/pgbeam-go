# pgbeam-go

Official Go client for the [PgBeam](https://pgbeam.com) control-plane API —
safe, scoped PostgreSQL access for AI agents on a globally distributed proxy
with connection pooling and query caching.

The module is imported via the vanity path `go.pgbeam.com/sdk`; the package name
is `pgbeam`.

## Install

```bash
go get go.pgbeam.com/sdk
```

Requires Go 1.24 or newer.

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	pgbeam "go.pgbeam.com/sdk"
)

func main() {
	client := pgbeam.NewClient(&pgbeam.ClientOptions{
		APIKey: os.Getenv("PGBEAM_API_KEY"), // prefix: pgb_
		// BaseURL: "https://api.pgbeam.com", // optional
	})

	ctx := context.Background()
	resp, err := client.Projects.ListProjects(ctx, &pgbeam.ListProjectsParams{OrgId: "org_123"})
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range resp.Projects {
		fmt.Printf("Project: %s (%s)\n", p.Name, p.Id)
	}
}
```

Operations are grouped into services on the client (`client.Projects`,
`client.Agents`, …), mirroring the API's tags.

## Resources

The client covers the full PgBeam API:

- **Projects, Databases, Replicas, Custom Domains, Cache Rules** — the proxy and
  pooling primitives.
- **Agent gateway** — agent credentials (issue, rotate, revoke), policy profiles
  (read-only, allowlists, masking, budgets), approval requests, webhook
  endpoints, anomaly alerts, and audit logs.
- **Analytics & Account** — plans, usage, spend limits, insights, account export.

## Documentation

Full reference, quickstart, and per-method examples at
[pgbeam.com/docs/go-sdk](https://pgbeam.com/docs/go-sdk).

## Contributing

Issues and pull requests are welcome here. An issue is the right place to start
for a bug, a wrong doc, or a missing capability; say what you ran, what
happened, what you expected, and which version you were on.

To build and test it locally:

```bash
go build ./...
go test ./...
```

Do not open a public issue for a suspected security vulnerability. Email
security@pgbeam.com, or report it privately from this repository's Security
tab.

## License

Apache 2.0 — see [LICENSE](LICENSE).
