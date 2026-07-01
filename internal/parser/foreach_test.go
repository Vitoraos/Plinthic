package parser

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestParseFile_PanicRecoveryWiring(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic escaped a function that should have recovered it: %v", r)
		}
	}()

	result := panicTestHarness()
	if result.Error == nil {
		t.Fatal("expected a recovered-panic ParseError, got nil")
	}
	if result.Error.Code != ErrPanicRecovered {
		t.Errorf("got error code %s, want %s", result.Error.Code, ErrPanicRecovered)
	}
}

func panicTestHarness() (result ParseFileResult) {
	defer func() {
		if r := recover(); r != nil {
			result = ParseFileResult{
				Error: NewParseError(ErrPanicRecovered, "synthetic", 0, "forced panic for test"),
			}
		}
	}()
	panic("forced panic for test")
}

func TestResolveCollectionInstances_NotACollection(t *testing.T) {
	rb := ResourceBlock{Iterator: ""}
	res := ResolveCollectionInstances(rb, nil, "")
	if res.IsCollection {
		t.Error("expected IsCollection=false when Iterator is empty")
	}
}

func TestResolveCollectionInstances_LiteralCount(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "dev", "main.tf")
	body := parseBodyForTest(t, path)

	var taskRolesBlock *hclsyntax.Block
	for _, b := range body.Blocks {
		if b.Type == "resource" && len(b.Labels) == 2 && b.Labels[1] == "task_roles" {
			taskRolesBlock = b
		}
	}
	if taskRolesBlock == nil {
		t.Fatal("aws_iam_role.task_roles block not found in fixture")
	}

	rb := ResourceBlock{Iterator: "count", File: path, Line: taskRolesBlock.OpenBraceRange.Start.Line}
	res := ResolveCollectionInstances(rb, taskRolesBlock.Body, "")

	if !res.IsCollection {
		t.Fatal("expected IsCollection=true")
	}
	if res.Source != SourceStaticLiteral {
		t.Errorf("got source %s, want %s", res.Source, SourceStaticLiteral)
	}
	want := []string{"0", "1", "2"}
	if len(res.Instances) != len(want) {
		t.Fatalf("got %d instances, want %d: %v", len(res.Instances), len(want), res.Instances)
	}
	for i, w := range want {
		if res.Instances[i] != w {
			t.Errorf("instance[%d]: got %s, want %s", i, res.Instances[i], w)
		}
	}
}

func TestResolveCollectionInstances_LiteralForEachObject(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "dev", "main.tf")
	body := parseBodyForTest(t, path)

	var workersBlock *hclsyntax.Block
	for _, b := range body.Blocks {
		if b.Type == "resource" && len(b.Labels) == 2 && b.Labels[1] == "workers" {
			workersBlock = b
		}
	}
	if workersBlock == nil {
		t.Fatal("aws_ecs_service.workers block not found in fixture")
	}

	rb := ResourceBlock{Iterator: "for_each", File: path, Line: workersBlock.OpenBraceRange.Start.Line}
	res := ResolveCollectionInstances(rb, workersBlock.Body, "")

	if !res.IsCollection || res.Source != SourceStaticLiteral {
		t.Fatalf("got IsCollection=%v Source=%s, want true/%s", res.IsCollection, res.Source, SourceStaticLiteral)
	}

	gotSet := map[string]bool{}
	for _, i := range res.Instances {
		gotSet[i] = true
	}
	for _, want := range []string{"api", "batch"} {
		if !gotSet[want] {
			t.Errorf("expected instance key %q in resolved for_each instances, got %v", want, res.Instances)
		}
	}
}

func TestResolveCollectionInstances_UnknownCardinality_NoState(t *testing.T) {
	rb := ResourceBlock{Iterator: "for_each", File: "fake.tf", Line: 1}
	res := ResolveCollectionInstances(rb, &hclsyntax.Body{Attributes: hclsyntax.Attributes{
		"for_each": {Expr: &hclsyntax.ScopeTraversalExpr{}},
	}}, "")

	if !res.IsCollection {
		t.Fatal("expected IsCollection=true even when instances are unresolvable")
	}
	if res.Error == nil {
		t.Fatal("expected a non-nil Error for unresolvable for_each")
	}
	if res.Error.Code != ErrUnknownCardinality {
		t.Errorf("got error code %s, want %s", res.Error.Code, ErrUnknownCardinality)
	}
	if len(res.Instances) != 0 {
		t.Errorf("expected zero instances on unknown cardinality, got %v", res.Instances)
	}
}

func parseBodyForTest(t *testing.T, path string) *hclsyntax.Body {
	t.Helper()
	src := mustReadFile(t, path)
	file, diags := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		t.Fatalf("fixture file failed to parse: %v", diags)
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		t.Fatal("parsed body was not *hclsyntax.Body")
	}
	return body
}
