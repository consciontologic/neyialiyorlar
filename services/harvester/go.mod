module github.com/neyialiyorlar/services/harvester

go 1.25.0

require (
	github.com/ledongthuc/pdf v0.0.0-20250511090121-5959a4027728
	github.com/lib/pq v1.10.9
	github.com/neyialiyorlar/services/shared v0.0.0
	golang.org/x/net v0.56.0
	golang.org/x/text v0.38.0
)

require gopkg.in/yaml.v3 v3.0.1 // indirect

replace github.com/neyialiyorlar/services/shared => ../shared
