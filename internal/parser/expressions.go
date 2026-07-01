package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type ExprType string

const (
	ExprDirectRef    ExprType = "direct_ref"
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

func classifyExpr(expr hclsyntax.Expression) ExprType {
	switch e := expr.(type) {
	case *hclsyntax.ScopeTraversalExpr:
		if isVarTraversal(e) {
			return ExprVarRef
		}
		return ExprDirectRef
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

func isVarTraversal(e *hclsyntax.ScopeTraversalExpr) bool {
	if len(e.Traversal) == 0 {
		return false
	}
	root, ok := e.Traversal[0].(hcl.TraverseRoot)
	if !ok {
		return false
	}
	return root.Name == "var"
}

func isPureLiteralTemplate(e *hclsyntax.TemplateExpr) bool {
	for _, part := range e.Parts {
		if _, ok := part.(*hclsyntax.LiteralValueExpr); !ok {
			return false
		}
	}
	return true
}
