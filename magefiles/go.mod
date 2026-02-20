module github.com/petar-djukic/go-unix-utils/magefiles

go 1.25.7

replace github.com/mesh-intelligence/mage-claude-orchestrator => /Users/petardjukic/WORKSPACE/mage-claude-orchestrator

require (
	github.com/magefile/mage v1.15.0
	github.com/mesh-intelligence/mage-claude-orchestrator v0.0.0-00010101000000-000000000000
)

require gopkg.in/yaml.v3 v3.0.1 // indirect
