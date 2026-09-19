package protosets

import (
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/zeromicro/go-zero/gateway"
)

//go:embed pb/*.pb
var protoFS embed.FS

// ExtractProtoSets extracts embedded .pb files to a temporary directory
func ExtractProtoSets() (string, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "protosets")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}

	// Create a pb subdirectory
	pbDir := filepath.Join(tempDir, "pb")
	if err := os.MkdirAll(pbDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create pb dir: %v", err)
	}

	// Read and write all .pb files
	entries, err := protoFS.ReadDir("pb")
	if err != nil {
		return "", fmt.Errorf("failed to read embed dir: %v", err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".pb" {
			continue
		}

		// Read the embedded file
		content, err := protoFS.ReadFile(path.Join("pb", entry.Name()))
		if err != nil {
			return "", fmt.Errorf("failed to read embed file %s: %v", entry.Name(), err)
		}

		// Write to the temporary directory
		outPath := filepath.Join(pbDir, entry.Name())
		if err := os.WriteFile(outPath, content, 0644); err != nil {
			return "", fmt.Errorf("failed to write file %s: %v", outPath, err)
		}
	}

	return tempDir, nil
}

// UpdateProtoSetsPaths updates the ProtoSets configuration paths for each Grpc service
func UpdateProtoSetsPaths(c *gateway.GatewayConf, tempDir string) {
	// Iterate over each Grpc service
	for i := range c.Upstreams {
		var protoPaths []string

		// Set the corresponding .pb file path for each Grpc service
		if len(c.Upstreams[i].ProtoSets) > 0 {
			switch c.Upstreams[i].ProtoSets[0] {
			case "apps/gateway/internal/embed/pb/account.pb":
				protoPaths = append(protoPaths, filepath.Join(tempDir, "pb", "account.pb"))
			case "apps/gateway/internal/embed/pb/market.pb":
				protoPaths = append(protoPaths, filepath.Join(tempDir, "pb", "market.pb"))
			case "apps/gateway/internal/embed/pb/trade.pb":
				protoPaths = append(protoPaths, filepath.Join(tempDir, "pb", "trade.pb"))
			case "apps/gateway/internal/embed/pb/dcmsg.pb":
				protoPaths = append(protoPaths, filepath.Join(tempDir, "pb", "dcmsg.pb"))
			case "apps/gateway/internal/embed/pb/rebate.pb":
				protoPaths = append(protoPaths, filepath.Join(tempDir, "pb", "rebate.pb"))
			case "apps/gateway/internal/embed/pb/twmsg.pb":
				protoPaths = append(protoPaths, filepath.Join(tempDir, "pb", "twmsg.pb"))
			case "apps/gateway/internal/embed/pb/adminpanel.pb":
				protoPaths = append(protoPaths, filepath.Join(tempDir, "pb", "adminpanel.pb"))
			case "apps/gateway/internal/embed/pb/campaign.pb":
				protoPaths = append(protoPaths, filepath.Join(tempDir, "pb", "campaign.pb"))
			case "apps/gateway/internal/embed/pb/api_push.pb":
				protoPaths = append(protoPaths, filepath.Join(tempDir, "pb", "api_push.pb"))
			}
		}

		// Update the ProtoSets paths for the Grpc service
		c.Upstreams[i].ProtoSets = protoPaths
	}
}
