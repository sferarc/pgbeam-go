package pgbeam

// sdkVersion is kept equal to the version in package.json, which changesets
// bumps; `pnpm run script sync-go-version` copies it here.
const sdkVersion = "0.6.0"

const userAgent = "pgbeam-go/" + sdkVersion
