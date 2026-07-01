package parser

import (
	"path/filepath"
	"strings"
)

type EnvDetectionPattern string

const (
	EnvPatternDirectory EnvDetectionPattern = "directory"
	EnvPatternFilename  EnvDetectionPattern = "filename"
	EnvPatternWorkspace EnvDetectionPattern = "workspace"
	EnvPatternUnknown   EnvDetectionPattern = "unknown"
)

type EnvDetectionConfidence string

const (
	ConfidenceHigh   EnvDetectionConfidence = "high"
	ConfidenceMedium EnvDetectionConfidence = "medium"
	ConfidenceLow    EnvDetectionConfidence = "low"
)

type EnvDetectionResult struct {
	Pattern           EnvDetectionPattern
	Confidence        EnvDetectionConfidence
	EnvironmentsFound []string
	FileEnvironments  map[string]string
}

// Single source of truth for recognized environment tokens.
func normalizeEnvName(raw string) (string, bool) {
	switch strings.ToLower(raw) {
	case "dev", "development":
		return "dev", true
	case "staging", "stage":
		return "staging", true
	case "prod", "production":
		return "prod", true
	case "shared":
		return "shared", true
	default:
		return "", false
	}
}

// DetectEnvironments implements Pattern A (directory-based) only.
// Pattern B/C are out of scope for Phase 1 — see stubs below.
func DetectEnvironments(repoRoot string, files []string) EnvDetectionResult {
	fileEnvs := make(map[string]string, len(files))
	envSet := make(map[string]bool)
	matchedAny := false

	for _, f := range files {
		rel, err := filepath.Rel(repoRoot, f)
		if err != nil {
			rel = f
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")

		env, found := findEnvironmentSegment(parts)
		if found {
			fileEnvs[f] = env
			envSet[env] = true
			matchedAny = true
		} else {
			fileEnvs[f] = ""
		}
	}

	if !matchedAny {
		return EnvDetectionResult{
			Pattern:          EnvPatternUnknown,
			Confidence:       ConfidenceLow,
			FileEnvironments: fileEnvs,
		}
	}

	envs := make([]string, 0, len(envSet))
	for e := range envSet {
		envs = append(envs, e)
	}

	return EnvDetectionResult{
		Pattern:           EnvPatternDirectory,
		Confidence:        ConfidenceHigh,
		EnvironmentsFound: envs,
		FileEnvironments:  fileEnvs,
	}
}

func findEnvironmentSegment(pathParts []string) (string, bool) {
	if len(pathParts) == 0 {
		return "", false
	}
	dirParts := pathParts[:len(pathParts)-1]
	for _, part := range dirParts {
		if env, ok := normalizeEnvName(part); ok {
			return env, true
		}
	}
	return "", false
}

// Not implemented in Phase 1 — see build memo Section 7.
func DetectEnvironmentsB(repoRoot string, files []string) EnvDetectionResult {
	panic("DetectEnvironmentsB: Pattern B is out of scope for Phase 1")
}

// Not implemented in Phase 1 — see build memo Section 7.
func DetectEnvironmentsC(repoRoot string, files []string) EnvDetectionResult {
	panic("DetectEnvironmentsC: Pattern C is out of scope for Phase 1")
}
