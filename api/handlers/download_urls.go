package handlers

import "strings"

// deriveDownloadURLs constructs platform-specific download URLs from the stored linux-amd64 URL.
// GitHub release asset URLs follow a predictable pattern:
//
//	https://github.com/{org}/{repo}/releases/download/{tag}/plugin-{os}-{arch}[.exe]
//
// We derive other-platform URLs by recognising and replacing the platform suffix.
// Platforms are inferred from the keys already present in the checksums map.
func deriveDownloadURLs(baseURL string, checksums map[string]string) map[string]string {
	if baseURL == "" || len(checksums) == 0 {
		return nil
	}

	cleanBase := strings.TrimSuffix(baseURL, ".exe")

	type platform struct{ goos, goarch string }
	known := []platform{
		{"linux", "amd64"}, {"linux", "arm64"},
		{"darwin", "amd64"}, {"darwin", "arm64"},
		{"windows", "amd64"}, {"windows", "arm64"},
	}

	var urlPrefix string
	for _, p := range known {
		suffix := "plugin-" + p.goos + "-" + p.goarch
		if idx := strings.LastIndex(cleanBase, suffix); idx >= 0 {
			urlPrefix = cleanBase[:idx]
			break
		}
	}
	if urlPrefix == "" {
		return nil
	}

	urls := make(map[string]string, len(checksums))
	for key := range checksums {
		parts := strings.SplitN(key, "_", 2)
		if len(parts) != 2 {
			continue
		}
		goos, goarch := parts[0], parts[1]
		assetName := "plugin-" + goos + "-" + goarch
		if goos == "windows" {
			assetName += ".exe"
		}
		urls[key] = urlPrefix + assetName
	}
	return urls
}
