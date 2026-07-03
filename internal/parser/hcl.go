package parser

import (
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type ResourceBlock struct {
	Type         string
	Name         string
	File         string
	Line         int
	Environment  string
	IsCollection bool
	Iterator     string
	Attributes   map[string]AttributeInfo
}

func (rb ResourceBlock) Address() string {
	return fmt.Sprintf("%s.%s", rb.Type, rb.Name)
}

type AttributeInfo struct {
	Line     int
	ExprType ExprType
	RawExpr  string
	ForceNew bool
	// Reference is non-nil only when ExprType is direct_ref or
	// data_ref — the structured target this attribute points to,
	// used to build CONSUMES_ATTRIBUTE graph edges in Phase 3.
	Reference *ResourceReference
}

type ParseFileResult struct {
	Resources []ResourceBlock
	Modules   []ModuleBlock
	Error     *ParseError
}

func ParseFile(path string, environment string) (result ParseFileResult) {
	defer func() {
		if r := recover(); r != nil {
			result = ParseFileResult{
				Error: NewParseError(ErrPanicRecovered, path, 0, fmt.Sprintf("recovered panic: %v", r)),
			}
		}
	}()

	src, err := os.ReadFile(path)
	if err != nil {
		return ParseFileResult{Error: NewParseError(ErrParseFailed, path, 0, err.Error())}
	}

	if !utf8.Valid(src) {
		return ParseFileResult{Error: NewParseError(ErrInvalidEncoding, path, 0, "file is not valid UTF-8")}
	}

	file, diags := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return ParseFileResult{Error: NewParseError(ErrParseFailed, path, firstErrorLine(diags), diags.Error())}
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return ParseFileResult{Error: NewParseError(ErrParseFailed, path, 0, "internal: body was not *hclsyntax.Body")}
	}

	var resources []ResourceBlock
	var modules []ModuleBlock
	for _, block := range body.Blocks {
		switch block.Type {
		case "resource":
			resources = append(resources, parseResourceBlock(block, path, environment, src))
		case "module":
			modules = append(modules, ParseModuleBlock(block, path))
		}
	}

	return ParseFileResult{Resources: resources, Modules: modules}
}

func parseResourceBlock(block *hclsyntax.Block, path string, environment string, src []byte) ResourceBlock {
	rb := ResourceBlock{
		File:        path,
		Line:        block.OpenBraceRange.Start.Line,
		Environment: environment,
		Attributes:  make(map[string]AttributeInfo),
	}

	if len(block.Labels) >= 1 {
		rb.Type = block.Labels[0]
	}
	if len(block.Labels) >= 2 {
		rb.Name = block.Labels[1]
	}

	if _, ok := block.Body.Attributes["for_each"]; ok {
		rb.IsCollection = true
		rb.Iterator = "for_each"
	} else if _, ok := block.Body.Attributes["count"]; ok {
		rb.IsCollection = true
		rb.Iterator = "count"
	}

	for name, attr := range block.Body.Attributes {
		exprType := classifyExpr(attr.Expr)
		info := AttributeInfo{
			Line:     attr.Range().Start.Line,
			ExprType: exprType,
			RawExpr:  extractRawExpr(src, attr),
		}
		if exprType == ExprDirectRef || exprType == ExprDataRef {
			if ref, ok := extractResourceReference(attr.Expr); ok {
				info.Reference = &ref
			}
		}
		rb.Attributes[name] = info
	}

	return rb
}

func extractRawExpr(src []byte, attr *hclsyntax.Attribute) string {
	start := attr.Expr.StartRange().Start.Byte
	end := attr.Expr.StartRange().End.Byte
	if start < 0 || end > len(src) || start > end {
		return ""
	}
	return string(src[start:end])
}

func firstErrorLine(diags hcl.Diagnostics) int {
	for _, d := range diags {
		if d.Severity == hcl.DiagError && d.Subject != nil {
			return d.Subject.Start.Line
		}
	}
	return 0
}
