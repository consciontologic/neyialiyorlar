module github.com/neyialiyorlar/services/api

go 1.23

require (
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.12.3
	github.com/neyialiyorlar/services/shared v0.0.0
	github.com/redis/go-redis/v9 v9.6.1
	gopkg.in/yaml.v3 v3.0.1
	nhooyr.io/websocket v1.8.11
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace github.com/neyialiyorlar/services/shared => ../shared
