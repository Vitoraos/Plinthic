package parser

import (
	"path/filepath"
	"testing"
)

func stateFixturesDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(fixturesDir(t), "state")
}

func TestLoadStateInstances_CountResource(t *testing.T) {
	path := filepath.Join(stateFixturesDir(t), "valid.tfstate.json")
	instances, err := LoadStateInstances(path, "aws_iam_role", "task_roles")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"0", "1", "2"}
	if len(instances) != len(want) {
		t.Fatalf("got %v, want %v", instances, want)
	}
	for i, w := range want {
		if instances[i] != w {
			t.Errorf("instance[%d]: got %s, want %s", i, instances[i], w)
		}
	}
}

func TestLoadStateInstances_ForEachResource(t *testing.T) {
	path := filepath.Join(stateFixturesDir(t), "valid.tfstate.json")
	instances, err := LoadStateInstances(path, "aws_ecs_service", "dynamic_workers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotSet := map[string]bool{}
	for _, i := range instances {
		gotSet[i] = true
	}
	for _, want := range []string{"web", "worker"} {
		if !gotSet[want] {
			t.Errorf("expected instance key %q, got %v", want, instances)
		}
	}
}

func TestLoadStateInstances_Unreadable(t *testing.T) {
	_, err := LoadStateInstances("/nonexistent/terraform.tfstate", "aws_iam_role", "x")
	if err == nil || err.Code != ErrStateFileUnreadable {
		t.Fatalf("got %v, want code %s", err, ErrStateFileUnreadable)
	}
}

func TestLoadStateInstances_Malformed(t *testing.T) {
	path := filepath.Join(stateFixturesDir(t), "malformed.tfstate.json")
	_, err := LoadStateInstances(path, "aws_iam_role", "x")
	if err == nil || err.Code != ErrStateFileMalformed {
		t.Fatalf("got %v, want code %s", err, ErrStateFileMalformed)
	}
}

func TestLoadStateInstances_WrongVersion(t *testing.T) {
	path := filepath.Join(stateFixturesDir(t), "wrong-version.tfstate.json")
	_, err := LoadStateInstances(path, "aws_iam_role", "x")
	if err == nil || err.Code != ErrStateFileMalformed {
		t.Fatalf("got %v, want code %s", err, ErrStateFileMalformed)
	}
}

func TestLoadStateInstances_ResourceNotFound(t *testing.T) {
	path := filepath.Join(stateFixturesDir(t), "missing-resource.tfstate.json")
	_, err := LoadStateInstances(path, "aws_iam_role", "task_roles")
	if err == nil || err.Code != ErrResourceNotInState {
		t.Fatalf("got %v, want code %s", err, ErrResourceNotInState)
	}
}
