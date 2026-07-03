package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type ExprType string

const (
	ExprDirectRef    ExprType = "direct_ref"    // aws_ecs_cluster.main.id — a managed resource
	ExprDataRef      ExprType = "data_ref"       // data.aws_ami.example.id — a data source
	ExprLocalRef     ExprType = "local_ref"       // local.foo — a Terraform local value
	ExprLiteral      ExprType = "literal"
	ExprTemplate     ExprType = "template"
	ExprVarRef       ExprType = "var_ref"
	ExprRelativeTrav ExprType = "relative_traversal"
	ExprFunctionCall ExprType = "function_call"
	ExprConditional  ExprType = "conditional"
	ExprObjectCons   ExprType = "object_construction"
	ExprTupleCons    ExprType = "tuple_construction"
	ExprForExpr      ExprType = "for_expression"
	ExprIndex        ExprType = "index_access"
	ExprUnknown      ExprType = "unknown"
)

// ResourceReference is the structured target of a direct_ref or
// data_ref expression — what CONSUMES_ATTRIBUTE edges in the graph
// are keyed on. TargetAttribute is "" when the whole resource/data
// source is referenced (e.g. `aws_ecs_cluster.main`, no trailing
// attribute) rather than one specific attribute of it.
type ResourceReference struct {
	IsDataSource    bool
	TargetType      string
	TargetName      string
	TargetAttribute string
}

func classifyExpr(expr hclsyntax.Expression) ExprType {
	switch e := expr.(type) {
	case *hclsyntax.ScopeTraversalExpr:
		return classifyTraversal(e)
	case *hclsyntax.RelativeTraversalExpr:
		return ExprRelativeTrav
	case *hclsyntax.LiteralValueExpr:
		return ExprLiteral
	case *hclsyntax.TemplateExpr:
		if isPureLiteralTemplate(e) {
			return ExprLiteral
		}
		return ExprTemplate
	case *hclsyntax.TemplateWrapExpr:
		return classifyExpr(e.Wrapped)
	case *hclsyntax.FunctionCallExpr:
		return ExprFunctionCall
	case *hclsyntax.ConditionalExpr:
		return ExprConditional
	case *hclsyntax.ObjectConsExpr:
		return ExprObjectCons
	case *hclsyntax.TupleConsExpr:
		return ExprTupleCons
	case *hclsyntax.ForExpr:
		return ExprForExpr
	case *hclsyntax.IndexExpr:
		return ExprIndex
	default:
		return ExprUnknown
	}
}

// classifyTraversal distinguishes the four root-identifier meanings a
// ScopeTraversalExpr can have in Terraform: a variable, a local value,
// a data source, or a managed resource. This split matters because
// each produces a structurally different graph edge (or none at all,
// for var_ref) — collapsing them into one "direct_ref" bucket would
// make it impossible to correctly build CONSUMES_ATTRIBUTE edges in
// Phase 3, since `local.foo` and `data.aws_ami.x` are not resource
// addresses even though they're syntactically identical in shape.
func classifyTraversal(e *hclsyntax.ScopeTraversalExpr) ExprType {
	root, ok := rootName(e)
	if !ok {
		return ExprUnknown
	}
	switch root {
	case "var":
		return ExprVarRef
	case "local":
		return ExprLocalRef
	case "data":
		return ExprDataRef
	default:
		return ExprDirectRef
	}
}

func rootName(e *hclsyntax.ScopeTraversalExpr) (string, bool) {
	if len(e.Traversal) == 0 {
		return "", false
	}
	root, ok := e.Traversal[0].(hcl.TraverseRoot)
	if !ok {
		return "", false
	}
	return root.Name, true
}

// extractResourceReference builds the structured target of a
// direct_ref or data_ref expression. Only meaningful when classifyExpr
// returned one of those two kinds for the same expression — callers
// must check ExprType first; this function does not re-classify.
//
// Traversal shapes handled:
//   - direct_ref, whole resource:    TYPE.NAME              (2 segments)
//   - direct_ref, one attribute:     TYPE.NAME.ATTR          (3+ segments)
//   - data_ref,   whole data source: data.TYPE.NAME          (3 segments)
//   - data_ref,   one attribute:     data.TYPE.NAME.ATTR     (4+ segments)
func extractResourceReference(expr hclsyntax.Expression) (ResourceReference, bool) {
	e, ok := expr.(*hclsyntax.ScopeTraversalExpr)
	if !ok {
		return ResourceReference{}, false
	}

	names, ok := traversalNames(e)
	if !ok {
		return ResourceReference{}, false
	}

	if len(names) > 0 && names[0] == "data" {
		names = names[1:]
		if len(names) < 2 {
			return ResourceReference{}, false
		}
		ref := ResourceReference{IsDataSource: true, TargetType: names[0], TargetName: names[1]}
		if len(names) >= 3 {
			ref.TargetAttribute = names[2]
		}
		return ref, true
	}

	if len(names) < 2 {
		return ResourceReference{}, false
	}
	ref := ResourceReference{TargetType: names[0], TargetName: names[1]}
	if len(names) >= 3 {
		ref.TargetAttribute = names[2]
	}
	return ref, true
}

// traversalNames converts a Traversal into its plain identifier
// segments (TraverseRoot / TraverseAttr only). Returns false if any
// segment is an index traversal (e.g. `foo.bar[0].baz`) — index
// traversals into a reference are a Phase-3-and-later concern, not
// silently mishandled here as a plain name.
func traversalNames(e *hclsyntax.ScopeTraversalExpr) ([]string, bool) {
	names := make([]string, 0, len(e.Traversal))
	for _, step := range e.Traversal {
		switch t := step.(type) {
		case hcl.TraverseRoot:
			names = append(names, t.Name)
		case hcl.TraverseAttr:
			names = append(names, t.Name)
		default:
			return nil, false
		}
	}
	return names, true
}

func isPureLiteralTemplate(e *hclsyntax.TemplateExpr) bool {
	for _, part := range e.Parts {
		if _, ok := part.(*hclsyntax.LiteralValueExpr); !ok {
			return false
		}
	}
	return true
}
