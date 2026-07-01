package parser

type IndexResult struct {
	EnvDetection EnvDetectionResult
	Resources    []ResourceBlock
	Errors       []*ParseError
	TooLarge     bool
}

// IndexRepo walks a repo, parses every .tf file, detects environments.
// No single file's failure aborts the rest of the repo.
func IndexRepo(repoRoot string) IndexResult {
	walkResult := WalkRepo(repoRoot, WalkOptions{})

	result := IndexResult{
		Errors:   append([]*ParseError{}, walkResult.Errors...),
		TooLarge: walkResult.TooLarge,
	}

	envDetection := DetectEnvironments(repoRoot, walkResult.Files)
	result.EnvDetection = envDetection

	for _, f := range walkResult.Files {
		env := envDetection.FileEnvironments[f]
		if env == "" {
			env = "UNKNOWN"
		}

		parseResult := ParseFile(f, env)
		if parseResult.Error != nil {
			result.Errors = append(result.Errors, parseResult.Error)
			continue
		}
		result.Resources = append(result.Resources, parseResult.Resources...)
	}

	return result
}
