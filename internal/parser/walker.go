package parser

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const DefaultMaxFilesPerIndex = 5000

type WalkResult struct {
	Files    []string
	Errors   []*ParseError
	TooLarge bool
}

type WalkOptions struct {
	MaxFiles int
}

// WalkRepo finds .tf files, skipping permission errors and symlink cycles.
func WalkRepo(root string, opts WalkOptions) WalkResult {
	maxFiles := opts.MaxFiles
	if maxFiles == 0 {
		maxFiles = DefaultMaxFilesPerIndex
	}

	result := WalkResult{}
	visited := newInodeSet()

	if info, err := os.Lstat(root); err == nil {
		visited.markVisited(root, info)
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				result.Errors = append(result.Errors, NewParseError(ErrSkippedPermission, path, 0, "permission denied"))
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			result.Errors = append(result.Errors, NewParseError(ErrSkippedPermission, path, 0, err.Error()))
			return nil
		}

		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}

			info, statErr := d.Info()
			if statErr != nil {
				return nil
			}

			if info.Mode()&os.ModeSymlink != 0 {
				resolved, evalErr := filepath.EvalSymlinks(path)
				if evalErr != nil {
					result.Errors = append(result.Errors, NewParseError(ErrSkippedSymlinkCycle, path, 0, "broken symlink"))
					return filepath.SkipDir
				}
				targetInfo, lstatErr := os.Lstat(resolved)
				if lstatErr != nil {
					return filepath.SkipDir
				}
				if visited.isVisited(resolved, targetInfo) {
					result.Errors = append(result.Errors, NewParseError(ErrSkippedSymlinkCycle, path, 0, "cycle detected: "+resolved))
					return filepath.SkipDir
				}
				visited.markVisited(resolved, targetInfo)
			} else {
				visited.markVisited(path, info)
			}
			return nil
		}

		if !strings.HasSuffix(d.Name(), ".tf") {
			return nil
		}

		if len(result.Files) >= maxFiles {
			result.TooLarge = true
			return filepath.SkipAll
		}

		result.Files = append(result.Files, path)
		return nil
	})

	if walkErr != nil && !errors.Is(walkErr, filepath.SkipAll) {
		result.Errors = append(result.Errors, NewParseError(ErrSkippedPermission, root, 0, "walk aborted: "+walkErr.Error()))
	}

	return result
}
