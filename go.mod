module github.com/ipfs/go-ipld-cbor

require (
	github.com/ipfs/boxo v0.13.1
	github.com/ipfs/go-block-format v0.1.2
	github.com/ipfs/go-cid v0.4.1
	github.com/ipfs/go-datastore v0.6.0
	github.com/ipfs/go-ipfs-util v0.0.2
	github.com/ipfs/go-ipld-format v0.5.0
	github.com/multiformats/go-multihash v0.2.3
	github.com/polydawn/refmt v0.89.0
	github.com/whyrusleeping/cbor-gen v0.0.0-20230818171029-f91ae536ca25
)

require (
	github.com/google/uuid v1.3.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.5 // indirect
	github.com/ipfs/bbloom v0.0.4 // indirect
	github.com/ipfs/go-detect-race v0.0.1 // indirect
	github.com/ipfs/go-log/v2 v2.5.1 // indirect
	github.com/ipfs/go-metrics-interface v0.0.1 // indirect
	github.com/jbenet/goprocess v0.1.4 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multibase v0.2.0 // indirect
	github.com/multiformats/go-varint v0.0.7 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.25.0 // indirect
	golang.org/x/crypto v0.12.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	lukechampine.com/blake3 v1.2.1 // indirect
)

go 1.20

replace github.com/ipfs/go-datastore => github.com/vulcanize/go-datastore v0.6.1-internal-0.0.1

replace github.com/ipfs/boxo => github.com/vulcanize/boxo v0.13.2-internal-0.0.2
