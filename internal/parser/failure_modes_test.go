package parser

import (
	"path/filepath"
	"testing"
)

func TestParseFile_MalformedHCL(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "malformed", "broken.tf")
	result := ParseFile(path, "dev")

	if result.Error == nil {
		t.Fatal("expected PARSE_FAILED for malformed HCL, got no error")
	}
	if result.Error.Code != ErrParseFailed {
		t.Errorf("got error code %s, want %s", result.Error.Code, ErrParseFailed)
	}
	if result.Resources != nil {
		t.Errorf("expected nil Resources on parse failure, got %d resources", len(result.Resources))
	}
}

func TestParseFile_MalformedHCL_DoesNotAffectSibling(t *testing.T) {
	root := filepath.Join(fixturesDir(t), "malformed")
	result := IndexRepo(root)

	foundError := false
	for _, e := range result.Errors {
		if e.Code == ErrParseFailed {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected a PARSE_FAILED error from broken.tf, found none")
	}

	foundValidCluster := false
	for _, r := range result.Resources {
		if r.Type == "aws_ecs_cluster" && r.Name == "main" {
			foundValidCluster = true
		}
	}
	if !foundValidCluster {
		t.Error("valid_sibling.tf's resource should still be present despite broken.tf failing to parse")
	}
}

func TestParseFile_InvalidUTF8(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "non-utf8", "invalid_encoding.tf")
	result := ParseFile(path, "dev")

	if result.Error == nil {
		t.Fatal("expected an error for non-UTF8 file, got none")
	}
	if result.Error.Code != ErrInvalidEncoding {
		t.Errorf("got error code %s, want %s", result.Error.Code, ErrInvalidEncoding)
	}
}

func TestWalkRepo_SymlinkCycle(t *testing.T) {
	root := filepath.Join(fixturesDir(t), "symlink-test")

	done := make(chan WalkResult, 1)
	go func() {
		done <- WalkRepo(root, WalkOptions{})
	}()

	select {
	case result := <-done:
		foundCycleError := false
		for _, e := range result.Errors {
			if e.Code == ErrSkippedSymlinkCycle {
				foundCycleError = true
			}
		}
		if !foundCycleError {
			t.Error("expected a SKIPPED_SYMLINK_CYCLE error, found none")
		}

		foundRealFile := false
		for _, f := range result.Files {
			if filepath.Base(f) == "main.tf" {
				foundRealFile = true
			}
		}
		if !foundRealFile {
			t.Error("expected real_dir/main.tf to be discovered despite the symlink cycle elsewhere")
		}

	case <-timeoutAfterSeconds(10):
		t.Fatal("WalkRepo did not return within 10s — likely stuck in an infinite symlink loop")
	}
}

func TestWalkRepo_MaxFilesCeiling(t *testing.T) {
	root := filepath.Join(fixturesDir(t), "eks-like")
	result := WalkRepo(root, WalkOptions{MaxFiles: 1})

	if !result.TooLarge {
		t.Error("expected TooLarge=true when MaxFiles=1 is exceeded")
	}
	if len(result.Files) != 1 {
		t.Errorf("expected exactly 1 file when capped at MaxFiles=1, got %d", len(result.Files))
	}
}

func TestIndexRepo_MultiEnvironment(t *testing.T) {
	root := filepath.Join(fixturesDir(t), "multi-env")
	result := IndexRepo(root)

	if result.EnvDetection.Pattern != EnvPatternDirectory {
		t.Errorf("got pattern %s, want %s", result.EnvDetection.Pattern, EnvPatternDirectory)
	}
	if result.EnvDetection.Confidence != ConfidenceHigh {
		t.Errorf("got confidence %s, want %s", result.EnvDetection.Confidence, ConfidenceHigh)
	}

	envCounts := map[string]int{}
	for _, r := range result.Resources {
		envCounts[r.Environment]++
	}

	for _, env := range []string{"dev", "staging", "prod"} {
		if envCounts[env] != 1 {
			t.Errorf("environment %s: got %d resources, want 1", env, envCounts[env])
		}
	}
}
