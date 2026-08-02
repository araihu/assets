package proof

import (
	"net/url"
	"os"
	"strings"
)

const jsDelivrProofBase = "https://cdn.jsdelivr.net/gh/araihu/assets@"

// AssetURL returns a proof-local path unless production CDN delivery is configured.
func AssetURL(assetPath string) string {
	if os.Getenv("APP_ENV") != "production" {
		return assetPath
	}
	version := os.Getenv("CDN_VERSION")
	if version == "" {
		return assetPath
	}
	return jsDelivrProofBase + url.PathEscape(version) + "/dist/proof/" + strings.TrimLeft(assetPath, "/")
}
