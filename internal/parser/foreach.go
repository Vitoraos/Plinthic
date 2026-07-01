package parser

import "github.com/hashicorp/hcl/v2/hclsyntax"

type ResolutionSource string

const (
	SourceStaticLiteral ResolutionSource = "static_literal"
	SourceStateFile     ResolutionSource = "state_file"
	SourceUnknown       ResolutionSource = "unknown"
)

type CollectionResolution struct {
	IsCollection bool
	Instances    []string
	Source       ResolutionSource
	Confidence   EnvDetectionConfidence
	Error        *ParseError
}

// Reviewer path: no state access, ever. Tier 1 -> Tier 3, non-blocking.
func ResolveCollectionInstancesForReview(rb ResourceBlock, attrs *hclsyntax.Body) CollectionResolution {
	if rb.Iterator == "" {
		return CollectionResolution{IsCollection: false}
	}

	if literal, ok := tryResolveLiteralIterator(rb, attrs); ok {
		return CollectionResolution{
			IsCollection: true,
			Instances:    literal,
			Source:       SourceStaticLiteral,
			Confidence:   ConfidenceHigh,
		}
	}

	return CollectionResolution{
		IsCollection: true,
		Source:       SourceUnknown,
		Confidence:   ConfidenceLow,
		Error: NewParseError(ErrUnknownCardinality, rb.File, rb.Line,
			"instances cannot be statically determined; no state file in Reviewer path"),
	}
}

// Agent path: state access expected. Tier 1 -> Tier 2. No Tier 3 —
// state failure is a hard error the caller must not continue past.
func ResolveCollectionInstancesForAgent(rb ResourceBlock, attrs *hclsyntax.Body, statePath string) CollectionResolution {
	if rb.Iterator == "" {
		return CollectionResolution{IsCollection: false}
	}

	if literal, ok := tryResolveLiteralIterator(rb, attrs); ok {
		return CollectionResolution{
			IsCollection: true,
			Instances:    literal,
			Source:       SourceStaticLiteral,
			Confidence:   ConfidenceHigh,
		}
	}

	instances, stateErr := LoadStateInstances(statePath, rb.Type, rb.Name)
	if stateErr != nil {
		return CollectionResolution{
			IsCollection: true,
			Source:       SourceUnknown,
			Confidence:   ConfidenceLow,
			Error:        stateErr,
		}
	}

	return CollectionResolution{
		IsCollection: true,
		Instances:    instances,
		Source:       SourceStateFile,
		Confidence:   ConfidenceHigh,
	}
}

func tryResolveLiteralIterator(rb ResourceBlock, attrs *hclsyntax.Body) ([]string, bool) {
	if attrs == nil {
		return nil, false
	}

	switch rb.Iterator {
	case "count":
		attr, ok := attrs.Attributes["count"]
		if !ok {
			return nil, false
		}
		lit, ok := attr.Expr.(*hclsyntax.LiteralValueExpr)
		if !ok {
			return nil, false
		}
		n, ok := literalToInt(lit)
		if !ok || n < 0 {
			return nil, false
		}
		instances := make([]string, n)
		for i := 0; i < n; i++ {
			instances[i] = intToString(i)
		}
		return instances, true

	case "for_each":
		attr, ok := attrs.Attributes["for_each"]
		if !ok {
			return nil, false
		}
		switch expr := attr.Expr.(type) {
		case *hclsyntax.TupleConsExpr:
			return tupleLiteralStrings(expr)
		case *hclsyntax.ObjectConsExpr:
			return objectLiteralKeys(expr)
		default:
			return nil, false
		}
	}

	return nil, false
}

func literalToInt(lit *hclsyntax.LiteralValueExpr) (int, bool) {
	bf := lit.Val.AsBigFloat()
	if bf == nil {
		return 0, false
	}
	f, _ := bf.Float64()
	n := int(f)
	if float64(n) != f {
		return 0, false
	}
	return n, true
}

func tupleLiteralStrings(expr *hclsyntax.TupleConsExpr) ([]string, bool) {
	out := make([]string, 0, len(expr.Exprs))
	for _, e := range expr.Exprs {
		s, ok := literalExprToString(e)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func objectLiteralKeys(expr *hclsyntax.ObjectConsExpr) ([]string, bool) {
	out := make([]string, 0, len(expr.Items))
	for _, item := range expr.Items {
		s, ok := literalExprToString(item.KeyExpr)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func literalExprToString(e hclsyntax.Expression) (string, bool) {
	switch v := e.(type) {
	case *hclsyntax.LiteralValueExpr:
		if v.Val.Type().FriendlyName() == "string" {
			return v.Val.AsString(), true
		}
		return "", false
	case *hclsyntax.TemplateExpr:
		if !isPureLiteralTemplate(v) {
			return "", false
		}
		var s string
		for _, part := range v.Parts {
			lit, ok := part.(*hclsyntax.LiteralValueExpr)
			if !ok {
				return "", false
			}
			s += lit.Val.AsString()
		}
		return s, true
	default:
		return "", false
	}
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
