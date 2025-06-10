package manifest

import (
	"bytes"
	"io"

	"gopkg.in/yaml.v3"
)

type Meta struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

// Match reports whether any document in data matches the requested
// Group / Version / Kind filters (* is wildcard).
func Match(data []byte, wantGroup, wantVersion, wantKind string) bool {
	// Cheap pre-filter
	if !bytes.Contains(bytes.ToLower(data), []byte("kind:")) {
		return false
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var m Meta
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			continue // tolerate malformed docs
		}

		if !equalsFold(m.Kind, wantKind) {
			continue
		}
		if wantGroup == "*" && wantVersion == "*" {
			return true
		}

		group, ver := splitAPIVersion(m.APIVersion)
		if wantGroup != "*" && wantGroup != group {
			continue
		}
		if wantVersion != "*" && wantVersion != ver {
			continue
		}
		return true
	}
	return false
}

// --- helpers -----------------------------------------------------------------

func splitAPIVersion(av string) (group, version string) {
	for i := range av {
		if av[i] == '/' {
			return av[:i], av[i+1:]
		}
	}
	return "", av // core group
}

func equalsFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ai, bi := a[i], b[i]
		if 'A' <= ai && ai <= 'Z' {
			ai += 'a' - 'A'
		}
		if 'A' <= bi && bi <= 'Z' {
			bi += 'a' - 'A'
		}
		if ai != bi {
			return false
		}
	}
	return true
}
