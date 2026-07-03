package parser

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestClassifyExpr_DirectRefVsDataRefVsLocalRef(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want ExprType
	}{
		{"resource ref", `x = aws_ecs_cluster.main.id`, ExprDirectRef},
		{"resource ref no attr", `x = aws_ecs_cluster.main`, ExprDirectRef},
		{"data source ref", `x = data.aws_ami.example.id`, ExprDataRef},
		{"local ref", `x = local.foo`, ExprLocalRef},
		{"var ref", `x = var.foo`, ExprVarRef},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			attr := parseSingleAttrForTest(t, c.src)
			got := classifyExpr(attr.Expr)
			if got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestExtractResourceReference_ResourceWithAttribute(t *testing.T) {
	attr := parseSingleAttrForTest(t, `x = aws_ecs_cluster.main.id`)
	ref, ok := extractResourceReference(attr.Expr)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	want := ResourceReference{TargetType: "aws_ecs_cluster", TargetName: "main", TargetAttribute: "id"}
	if ref != want {
		t.Errorf("got %+v, want %+v", ref, want)
	}
}

func TestExtractResourceReference_ResourceWholeReference(t *testing.T) {
	attr := parseSingleAttrForTest(t, `x = aws_ecs_cluster.main`)
	ref, ok := extractResourceReference(attr.Expr)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	want := ResourceReference{TargetType: "aws_ecs_cluster", TargetName: "main", TargetAttribute: ""}
	if ref != want {
		t.Errorf("got %+v, want %+v", ref, want)
	}
}

func TestExtractResourceReference_DataSource(t *testing.T) {
	attr := parseSingleAttrForTest(t, `x = data.aws_ami.example.id`)
	ref, ok := extractResourceReference(attr.Expr)
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	want := ResourceReference{IsDataSource: true, TargetType: "aws_ami", TargetName: "example", TargetAttribute: "id"}
	if ref != want {
		t.Errorf("got %+v, want %+v", ref, want)
	}
}

func TestExtractResourceReference_IndexTraversal_NotExtracted(t *testing.T) {
	attr := parseSingleAttrForTest(t, `x = aws_ecs_cluster.main.tags["env"]`)
	_, ok := extractResourceReference(attr.Expr)
	if ok {
		t.Error("expected extraction to decline on an index traversal, not guess at a name")
	}
}

// parseSingleAttrForTest parses a one-line HCL attribute assignment
// and returns its *hclsyntax.Attribute, for testing classifyExpr and
// extractResourceReference in isolation without needing a full
// resource block or fixture file.
func parseSingleAttrForTest(t *testing.T, line string) *hclsyntax.Attribute {
	t.Helper()
	src := []byte(line + "\n")
	f, diags := hclsyntax.ParseConfig(src, "test.tf", hclPos1())
	if diags.HasErrors() {
		t.Fatalf("failed to parse test snippet %q: %v", line, diags)
	}
	body := f.Body.(*hclsyntax.Body)
	attr, ok := body.Attributes["x"]
	if !ok {
		t.Fatalf("no attribute 'x' found in snippet %q", line)
	}
	return attr
}

func hclPos1() hclPosAlias {
	return hclPosAlias{Line: 1, Column: 1}
}
