package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type ModuleSourceCategory string

const (
	ModuleCategoryGit      ModuleSourceCategory = "git"
	ModuleCategoryRegistry ModuleSourceCategory = "registry"
	ModuleCategoryLocal    ModuleSourceCategory = "local"
	ModuleCategoryArchive  ModuleSourceCategory = "archive"
	ModuleCategoryUnknown  ModuleSourceCategory = "unknown"
)

// unpinnedSentinel makes "no ref/version specified" distinguishable in
// a graph key from a coincidentally-empty string, rather than letting
// two unrelated unpinned modules silently collide on "".
const unpinnedSentinel = "UNPINNED"

var bareGitHosts = []string{"github.com/", "gitlab.com/", "bitbucket.org/"}

var archiveExtensions = []string{".zip", ".tar.gz", ".tar.bz2", ".tar.xz", ".tgz"}

// ModuleSourceInfo is the result of classifying a raw `source` string
// alone, with no HCL block context — kept separate from ModuleBlock so
// string-parsing logic can be tested independently of block extraction.
type ModuleSourceInfo struct {
	Category   ModuleSourceCategory
	BaseSource string
	Subdir     string
	Ref        string // git only; "" if unpinned or not applicable
}

type ModuleBlock struct {
	Name       string
	File       string
	Line       int
	RawSource  string
	RawVersion string // literal `version` attribute, "" if absent
	Source     ModuleSourceInfo

	// GraphKey uniquely identifies this module+version/ref for
	// blast-radius purposes. "" for local, archive, and unknown —
	// none of these have a meaningful, safely-parseable pin concept.
	GraphKey string

	// Pinned is true only when a git ref or registry version was
	// explicitly specified. Meaningless (left false) for local/archive/unknown.
	Pinned bool

	// Error is set only when Category is Unknown. Non-blocking.
	Error *ParseError
}

// ParseModuleBlock extracts and classifies a `module` block's source,
// building its graph key according to category.
func ParseModuleBlock(block *hclsyntax.Block, file string) ModuleBlock {
	mb := ModuleBlock{
		File: file,
		Line: block.OpenBraceRange.Start.Line,
	}
	if len(block.Labels) >= 1 {
		mb.Name = block.Labels[0]
	}

	sourceAttr, ok := block.Body.Attributes["source"]
	if !ok {
		mb.Source = ModuleSourceInfo{Category: ModuleCategoryUnknown}
		mb.Error = NewParseError(ErrUnknownSourceType, file, mb.Line, "module block has no source attribute")
		return mb
	}

	rawSource, ok := literalExprToString(sourceAttr.Expr)
	if !ok {
		mb.Source = ModuleSourceInfo{Category: ModuleCategoryUnknown}
		mb.Error = NewParseError(ErrUnknownSourceType, file, mb.Line, "source is not a literal string; cannot classify")
		return mb
	}
	mb.RawSource = rawSource
	mb.Source = ParseModuleSource(rawSource)

	if versionAttr, ok := block.Body.Attributes["version"]; ok {
		if v, ok := literalExprToString(versionAttr.Expr); ok {
			mb.RawVersion = v
		}
	}

	switch mb.Source.Category {
	case ModuleCategoryGit:
		mb.Pinned = mb.Source.Ref != ""
		ref := mb.Source.Ref
		if ref == "" {
			ref = unpinnedSentinel
		}
		mb.GraphKey = mb.Source.BaseSource + "@" + ref

	case ModuleCategoryRegistry:
		mb.Pinned = mb.RawVersion != ""
		version := mb.RawVersion
		if version == "" {
			version = unpinnedSentinel
		}
		mb.GraphKey = mb.Source.BaseSource + "@" + version

	case ModuleCategoryLocal, ModuleCategoryArchive:
		// No graph key: local always tracks its caller's version;
		// archive sources have no parseable pin concept in Phase 2.

	case ModuleCategoryUnknown:
		mb.Error = NewParseError(ErrUnknownSourceType, file, mb.Line,
			fmt.Sprintf("unrecognized module source type: %q", rawSource))
	}

	return mb
}

// ParseModuleSource classifies a raw source string in isolation.
func ParseModuleSource(raw string) ModuleSourceInfo {
	pathPart, queryPart := splitQuery(raw)
	category := classifyCategory(pathPart)

	info := ModuleSourceInfo{Category: category}

	switch category {
	case ModuleCategoryGit:
		base, subdir := splitSubdir(pathPart)
		info.BaseSource = strings.TrimPrefix(base, "git::")
		info.Subdir = subdir
		info.Ref = extractQueryParam(queryPart, "ref")

	case ModuleCategoryRegistry:
		base, subdir := splitSubdir(pathPart)
		info.BaseSource = base
		info.Subdir = subdir

	default: // local, archive, unknown
		info.BaseSource = pathPart
	}

	return info
}

func classifyCategory(pathPart string) ModuleSourceCategory {
	switch {
	case isLocalPath(pathPart):
		return ModuleCategoryLocal
	case isGitSource(pathPart):
		return ModuleCategoryGit
	case isArchiveSource(pathPart):
		return ModuleCategoryArchive
	case isRegistrySource(pathPart):
		return ModuleCategoryRegistry
	default:
		return ModuleCategoryUnknown
	}
}

func isLocalPath(p string) bool {
	return strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/")
}

func isGitSource(p string) bool {
	if strings.HasPrefix(p, "git::") || strings.HasPrefix(p, "git@") {
		return true
	}
	for _, host := range bareGitHosts {
		if strings.HasPrefix(p, host) {
			return true
		}
	}
	return false
}

func isArchiveSource(p string) bool {
	if strings.HasPrefix(p, "s3::") || strings.HasPrefix(p, "gcs::") {
		return true
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		lower := strings.ToLower(p)
		for _, ext := range archiveExtensions {
			if strings.HasSuffix(lower, ext) {
				return true
			}
		}
	}
	return false
}

// isRegistrySource matches NAMESPACE/NAME/PROVIDER (public) or
// HOST/NAMESPACE/NAME/PROVIDER (private, host segment contains a dot).
func isRegistrySource(pathPart string) bool {
	if strings.Contains(pathPart, "://") {
		return false
	}
	base, _ := splitSubdir(pathPart)
	segments := strings.Split(base, "/")
	switch len(segments) {
	case 3:
		return true
	case 4:
		return strings.Contains(segments[0], ".")
	default:
		return false
	}
}

func splitQuery(raw string) (path string, query string) {
	if idx := strings.Index(raw, "?"); idx != -1 {
		return raw[:idx], raw[idx+1:]
	}
	return raw, ""
}

// splitSubdir separates a base address from an optional "//subdir"
// suffix. The search starts after any "://" protocol separator so it
// isn't confused by the "//" inside "https://" or "ssh://".
func splitSubdir(raw string) (base string, subdir string) {
	searchFrom := 0
	if idx := strings.Index(raw, "://"); idx != -1 {
		searchFrom = idx + 3
	}
	if idx := strings.Index(raw[searchFrom:], "//"); idx != -1 {
		splitAt := searchFrom + idx
		return raw[:splitAt], raw[splitAt+2:]
	}
	return raw, ""
}

func extractQueryParam(query string, key string) string {
	values, err := url.ParseQuery(query)
	if err != nil {
		return ""
	}
	return values.Get(key)
}
