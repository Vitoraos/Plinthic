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

		if d.Name() == ".git" || d.Name() == "node_modules" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// d.IsDir() reports false for a symlink, even one pointing at a
		// directory — WalkDir classifies entries by their own Lstat
		// type, not the resolved target's. So the symlink check must
		// happen before/independent of d.IsDir(), or a symlinked
		// directory silently skips cycle detection and falls through
		// to the plain-file branch below, which is exactly what caused
		// this bug: no error, no recursion, no detection.
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			resolved, evalErr := filepath.EvalSymlinks(path)
			if evalErr != nil {
				result.Errors = append(result.Errors, NewParseError(ErrSkippedSymlinkCycle, path, 0, "broken symlink"))
				return nil
			}
			targetInfo, statErr := os.Stat(resolved)
			if statErr != nil {
				return nil
			}
			if !targetInfo.IsDir() {
				return nil // symlink to a file: not a cycle risk, and not a dir to recurse into
			}
			if visited.isVisited(resolved, targetInfo) {
				result.Errors = append(result.Errors, NewParseError(ErrSkippedSymlinkCycle, path, 0, "cycle detected: "+resolved))
				return nil
			}
			visited.markVisited(resolved, targetInfo)
			// Recurse manually into the resolved target. filepath.WalkDir
			// does not follow symlinks on its own, by design, so we walk
			// the resolved path ourselves and merge results in.
			sub := WalkRepo(resolved, WalkOptions{MaxFiles: maxFiles - len(result.Files)})
			result.Files = append(result.Files, sub.Files...)
			result.Errors = append(result.Errors, sub.Errors...)
			if sub.TooLarge {
				result.TooLarge = true
				return filepath.SkipAll
			}
			return nil
		}

		if d.IsDir() {
			visited.markVisited(path, info)
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
