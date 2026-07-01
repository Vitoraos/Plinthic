package parser

import (
	"path/filepath"
	"runtime"
	"testing"
)

func fixturesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve caller for fixtures path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "test", "fixtures")
}

func TestParseFile_EKSLikeDev_ResourceCount(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "dev", "main.tf")
	result := ParseFile(path, "dev")

	if result.Error != nil {
		t.Fatalf("unexpected parse error: %v", result.Error)
	}

	want := 4
	if len(result.Resources) != want {
		t.Fatalf("got %d resources, want %d: %+v", len(result.Resources), want, result.Resources)
	}
}

func TestParseFile_EKSLikeDev_DirectRefVsLiteral(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "dev", "main.tf")
	result := ParseFile(path, "dev")
	if result.Error != nil {
		t.Fatalf("unexpected parse error: %v", result.Error)
	}

	var apiWorker *ResourceBlock
	for i := range result.Resources {
		if result.Resources[i].Type == "aws_ecs_service" && result.Resources[i].Name == "api_worker" {
			apiWorker = &result.Resources[i]
		}
	}
	if apiWorker == nil {
		t.Fatal("aws_ecs_service.api_worker not found")
	}

	clusterAttr, ok := apiWorker.Attributes["cluster"]
	if !ok {
		t.Fatal("expected `cluster` attribute on api_worker")
	}
	if clusterAttr.ExprType != ExprDirectRef {
		t.Errorf("dev api_worker.cluster: got %s, want %s", clusterAttr.ExprType, ExprDirectRef)
	}

	nameAttr, ok := apiWorker.Attributes["name"]
	if !ok {
		t.Fatal("expected `name` attribute on api_worker")
	}
	if nameAttr.ExprType != ExprLiteral {
		t.Errorf("dev api_worker.name: got %s, want %s", nameAttr.ExprType, ExprLiteral)
	}
}

func TestParseFile_EKSLikeStaging_HardcodedClusterString(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "staging", "main.tf")
	result := ParseFile(path, "staging")
	if result.Error != nil {
		t.Fatalf("unexpected parse error: %v", result.Error)
	}

	var apiWorker *ResourceBlock
	for i := range result.Resources {
		if result.Resources[i].Type == "aws_ecs_service" && result.Resources[i].Name == "api_worker" {
			apiWorker = &result.Resources[i]
		}
	}
	if apiWorker == nil {
		t.Fatal("aws_ecs_service.api_worker not found in staging fixture")
	}

	clusterAttr, ok := apiWorker.Attributes["cluster"]
	if !ok {
		t.Fatal("expected `cluster` attribute on staging api_worker")
	}
	if clusterAttr.ExprType != ExprLiteral {
		t.Errorf("staging api_worker.cluster: got %s, want %s — this is the hardcoded-string case the product exists to catch",
			clusterAttr.ExprType, ExprLiteral)
	}
}

func TestParseFile_ForEachAndCountDetection(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "dev", "main.tf")
	result := ParseFile(path, "dev")
	if result.Error != nil {
		t.Fatalf("unexpected parse error: %v", result.Error)
	}

	var workers, taskRoles *ResourceBlock
	for i := range result.Resources {
		r := &result.Resources[i]
		if r.Type == "aws_ecs_service" && r.Name == "workers" {
			workers = r
		}
		if r.Type == "aws_iam_role" && r.Name == "task_roles" {
			taskRoles = r
		}
	}

	if workers == nil {
		t.Fatal("aws_ecs_service.workers not found")
	}
	if !workers.IsCollection || workers.Iterator != "for_each" {
		t.Errorf("workers: got IsCollection=%v Iterator=%q, want true/for_each", workers.IsCollection, workers.Iterator)
	}

	if taskRoles == nil {
		t.Fatal("aws_iam_role.task_roles not found")
	}
	if !taskRoles.IsCollection || taskRoles.Iterator != "count" {
		t.Errorf("task_roles: got IsCollection=%v Iterator=%q, want true/count", taskRoles.IsCollection, taskRoles.Iterator)
	}
}

func TestParseFile_NonexistentFile(t *testing.T) {
	result := ParseFile("/nonexistent/path/does-not-exist.tf", "dev")
	if result.Error == nil {
		t.Fatal("expected an error for a nonexistent file, got none")
	}
	if result.Error.Code != ErrParseFailed {
		t.Errorf("got error code %s, want %s", result.Error.Code, ErrParseFailed)
	}
}
