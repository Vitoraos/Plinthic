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
	ForceNew bool // populated later, not by the parser itself
}

type ParseFileResult struct {
	Resources []ResourceBlock
	Error     *ParseError
}

// ParseFile never panics out and never hands non-UTF8 input to hclsyntax.
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
	for _, block := range body.Blocks {
		if block.Type != "resource" {
			continue
		}
		resources = append(resources, parseResourceBlock(block, path, environment, src))
	}

	return ParseFileResult{Resources: resources}
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
		rb.Attributes[name] = AttributeInfo{
			Line:     attr.Range().Start.Line,
			ExprType: classifyExpr(attr.Expr),
			RawExpr:  extractRawExpr(src, attr),
		}
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
