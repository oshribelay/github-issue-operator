package utils

import (
	"fmt"
	"strings"
)

func ParseRepoUrl(repoUrl string) (string, string, error) {
	repoUrl = strings.TrimPrefix(repoUrl, "https://github.com/")
	parts := strings.Split(repoUrl, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid repo url: %s", repoUrl)
	}

	return parts[0], parts[1], nil
}
