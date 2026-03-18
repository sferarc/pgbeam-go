# pgbeam-go

Go client library for the [PgBeam](https://pgbeam.com) API — globally
distributed PostgreSQL proxy platform with connection pooling and query caching.

## Install

```bash
go get github.com/pgbeam/pgbeam-go
```

## Usage

```go
package main

import (
	"context"
	"fmt"

	pgbeam "github.com/pgbeam/pgbeam-go"
)

func main() {
	client := pgbeam.NewClient("your-api-token")

	projects, err := client.ListProjects(context.Background(), "org_123")
	if err != nil {
		panic(err)
	}

	for _, p := range projects {
		fmt.Printf("Project: %s (%s)\n", p.Name, p.ID)
	}
}
```

## Resources

The client covers all PgBeam API resources:

- **Projects** — create, read, update, delete
- **Databases** — manage PostgreSQL database connections
- **Replicas** — configure read replicas with region selection
- **Custom Domains** — bring your own domain for connection strings
- **Cache Rules** — fine-grained query caching configuration
- **Spend Limits** — budget controls and alerts

## Documentation

Full API reference at [docs.pgbeam.com/go-sdk](https://docs.pgbeam.com/go-sdk).

## License

Apache 2.0 — see [LICENSE](LICENSE).
