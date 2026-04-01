package model

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode"
)

func buildVersionAwareURL(base, endpoint string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid base URL: %q", base)
	}

	baseSegments := splitPathSegments(u.Path)
	endpointSegments := splitPathSegments(endpoint)

	if len(baseSegments) > 0 && len(endpointSegments) > 0 && isVersionSegment(baseSegments[len(baseSegments)-1]) && isVersionSegment(endpointSegments[0]) {
		endpointSegments = endpointSegments[1:]
	}

	allSegments := append(append([]string{}, baseSegments...), endpointSegments...)
	u.Path = path.Join("/", path.Join(allSegments...))
	u.RawPath = ""
	return u.String(), nil
}

func splitPathSegments(value string) []string {
	raw := strings.Trim(value, "/")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "/")
}

func isVersionSegment(value string) bool {
	if len(value) < 2 {
		return false
	}
	if value[0] != 'v' && value[0] != 'V' {
		return false
	}
	hasDigit := false
	for _, r := range value[1:] {
		if unicode.IsDigit(r) {
			hasDigit = true
			continue
		}
		if r == '.' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return hasDigit
}
