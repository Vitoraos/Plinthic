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

func TestResolveCollectionInstancesForReview_NotACollection(t *testing.T) {
	rb := ResourceBlock{Iterator: ""}
	res := ResolveCollectionInstancesForReview(rb, nil)
	if res.IsCollection {
		t.Error("expected IsCollection=false when Iterator is empty")
	}
}

func TestResolveCollectionInstancesForReview_LiteralCount(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "dev", "main.tf")
	body := parseBodyForTest(t, path)
	block := findResourceBlock(t, body, "task_roles")

	rb := ResourceBlock{Iterator: "count", File: path, Line: block.OpenBraceRange.Start.Line}
	res := ResolveCollectionInstancesForReview(rb, block.Body)

	if !res.IsCollection || res.Source != SourceStaticLiteral {
		t.Fatalf("got IsCollection=%v Source=%s, want true/%s", res.IsCollection, res.Source, SourceStaticLiteral)
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

func TestResolveCollectionInstancesForReview_LiteralForEachObject(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "dev", "main.tf")
	body := parseBodyForTest(t, path)
	block := findResourceBlock(t, body, "workers")

	rb := ResourceBlock{Iterator: "for_each", File: path, Line: block.OpenBraceRange.Start.Line}
	res := ResolveCollectionInstancesForReview(rb, block.Body)

	if !res.IsCollection || res.Source != SourceStaticLiteral {
		t.Fatalf("got IsCollection=%v Source=%s, want true/%s", res.IsCollection, res.Source, SourceStaticLiteral)
	}

	gotSet := map[string]bool{}
	for _, i := range res.Instances {
		gotSet[i] = true
	}
	for _, want := range []string{"api", "batch"} {
		if !gotSet[want] {
			t.Errorf("expected instance key %q, got %v", want, res.Instances)
		}
	}
}

func TestResolveCollectionInstancesForReview_UnknownCardinality(t *testing.T) {
	rb := ResourceBlock{Iterator: "for_each", File: "fake.tf", Line: 1}
	attrs := &hclsyntax.Body{Attributes: hclsyntax.Attributes{
		"for_each": {Expr: &hclsyntax.ScopeTraversalExpr{}},
	}}
	res := ResolveCollectionInstancesForReview(rb, attrs)

	if !res.IsCollection {
		t.Fatal("expected IsCollection=true even when instances are unresolvable")
	}
	if res.Error == nil || res.Error.Code != ErrUnknownCardinality {
		t.Fatalf("got error %v, want code %s", res.Error, ErrUnknownCardinality)
	}
	if len(res.Instances) != 0 {
		t.Errorf("expected zero instances, got %v", res.Instances)
	}
}

func TestResolveCollectionInstancesForAgent_LiteralStillWorks(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "dev", "main.tf")
	body := parseBodyForTest(t, path)
	block := findResourceBlock(t, body, "task_roles")
	rb := ResourceBlock{Iterator: "count", File: path, Line: block.OpenBraceRange.Start.Line}

	res := ResolveCollectionInstancesForAgent(rb, block.Body, "")
	if res.Source != SourceStaticLiteral || res.Error != nil {
		t.Fatalf("got Source=%s Error=%v, want static_literal/nil", res.Source, res.Error)
	}
}

func TestResolveCollectionInstancesForAgent_FallsBackToState_Success(t *testing.T) {
	statePath := filepath.Join(fixturesDir(t), "state", "valid.tfstate.json")
	rb := ResourceBlock{Type: "aws_ecs_service", Name: "dynamic_workers", Iterator: "for_each", File: "fake.tf", Line: 1}
	attrs := &hclsyntax.Body{Attributes: hclsyntax.Attributes{
		"for_each": {Expr: &hclsyntax.ScopeTraversalExpr{}}, // non-literal, forces Tier 2
	}}

	res := ResolveCollectionInstancesForAgent(rb, attrs, statePath)
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if res.Source != SourceStateFile {
		t.Errorf("got Source=%s, want %s", res.Source, SourceStateFile)
	}
	gotSet := map[string]bool{}
	for _, i := range res.Instances {
		gotSet[i] = true
	}
	for _, want := range []string{"web", "worker"} {
		if !gotSet[want] {
			t.Errorf("expected instance key %q, got %v", want, res.Instances)
		}
	}
}

func TestResolveCollectionInstancesForAgent_StateUnreadable_HardError(t *testing.T) {
	rb := ResourceBlock{Type: "aws_ecs_service", Name: "dynamic_workers", Iterator: "for_each", File: "fake.tf", Line: 1}
	attrs := &hclsyntax.Body{Attributes: hclsyntax.Attributes{
		"for_each": {Expr: &hclsyntax.ScopeTraversalExpr{}},
	}}
	res := ResolveCollectionInstancesForAgent(rb, attrs, "/nonexistent/terraform.tfstate")
	assertAgentHardError(t, res, ErrStateFileUnreadable)
}

func TestResolveCollectionInstancesForAgent_StateMalformed_HardError(t *testing.T) {
	statePath := filepath.Join(fixturesDir(t), "state", "malformed.tfstate.json")
	rb := ResourceBlock{Type: "aws_ecs_service", Name: "dynamic_workers", Iterator: "for_each", File: "fake.tf", Line: 1}
	attrs := &hclsyntax.Body{Attributes: hclsyntax.Attributes{
		"for_each": {Expr: &hclsyntax.ScopeTraversalExpr{}},
	}}
	res := ResolveCollectionInstancesForAgent(rb, attrs, statePath)
	assertAgentHardError(t, res, ErrStateFileMalformed)
}

func TestResolveCollectionInstancesForAgent_ResourceNotInState_HardError(t *testing.T) {
	statePath := filepath.Join(fixturesDir(t), "state", "missing-resource.tfstate.json")
	rb := ResourceBlock{Type: "aws_ecs_service", Name: "dynamic_workers", Iterator: "for_each", File: "fake.tf", Line: 1}
	attrs := &hclsyntax.Body{Attributes: hclsyntax.Attributes{
		"for_each": {Expr: &hclsyntax.ScopeTraversalExpr{}},
	}}
	res := ResolveCollectionInstancesForAgent(rb, attrs, statePath)
	assertAgentHardError(t, res, ErrResourceNotInState)
}

// Enforces the Phase 1.5 rule: Agent state failures must never surface
// as ErrUnknownCardinality — that would misrepresent a blocking
// failure as a soft, continuable one.
func assertAgentHardError(t *testing.T, res CollectionResolution, wantCode ErrorCode) {
	t.Helper()
	if res.Error == nil {
		t.Fatal("expected a hard error, got nil")
	}
	if res.Error.Code != wantCode {
		t.Errorf("got error code %s, want %s", res.Error.Code, wantCode)
	}
	if res.Error.Code == ErrUnknownCardinality {
		t.Fatal("Agent path must never silently degrade to ErrUnknownCardinality")
	}
	if len(res.Instances) != 0 {
		t.Errorf("expected zero instances on hard error, got %v", res.Instances)
	}
}

func findResourceBlock(t *testing.T, body *hclsyntax.Body, name string) *hclsyntax.Block {
	t.Helper()
	for _, b := range body.Blocks {
		if b.Type == "resource" && len(b.Labels) == 2 && b.Labels[1] == name {
			return b
		}
	}
	t.Fatalf("resource block %q not found", name)
	return nil
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
