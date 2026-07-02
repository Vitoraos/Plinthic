package parser

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

import (
	"path/filepath"
	"testing"
)

func TestParseModuleSource_GitWithRef(t *testing.T) {
	info := ParseModuleSource("git::https://github.com/myorg/network-modules.git//vpc?ref=v1.0.0")
	if info.Category != ModuleCategoryGit {
		t.Fatalf("got category %s, want %s", info.Category, ModuleCategoryGit)
	}
	if info.BaseSource != "https://github.com/myorg/network-modules.git" {
		t.Errorf("got BaseSource %q", info.BaseSource)
	}
	if info.Subdir != "vpc" {
		t.Errorf("got Subdir %q, want vpc", info.Subdir)
	}
	if info.Ref != "v1.0.0" {
		t.Errorf("got Ref %q, want v1.0.0", info.Ref)
	}
}

func TestParseModuleSource_GitUnpinned(t *testing.T) {
	info := ParseModuleSource("git::https://github.com/myorg/network-modules.git//vpc")
	if info.Category != ModuleCategoryGit {
		t.Fatalf("got category %s, want %s", info.Category, ModuleCategoryGit)
	}
	if info.Ref != "" {
		t.Errorf("got Ref %q, want empty (unpinned)", info.Ref)
	}
}

func TestParseModuleSource_BareGitHost(t *testing.T) {
	info := ParseModuleSource("github.com/hashicorp/example")
	if info.Category != ModuleCategoryGit {
		t.Fatalf("got category %s, want %s (bare git host should be detected)", info.Category, ModuleCategoryGit)
	}
}

func TestParseModuleSource_RegistryWithSubmodule(t *testing.T) {
	info := ParseModuleSource("terraform-aws-modules/iam/aws//modules/iam-assumable-role")
	if info.Category != ModuleCategoryRegistry {
		t.Fatalf("got category %s, want %s", info.Category, ModuleCategoryRegistry)
	}
	if info.BaseSource != "terraform-aws-modules/iam/aws" {
		t.Errorf("got BaseSource %q", info.BaseSource)
	}
	if info.Subdir != "modules/iam-assumable-role" {
		t.Errorf("got Subdir %q", info.Subdir)
	}
}

func TestParseModuleSource_PrivateRegistry(t *testing.T) {
	info := ParseModuleSource("app.terraform.io/myorg/vpc/aws")
	if info.Category != ModuleCategoryRegistry {
		t.Fatalf("got category %s, want %s (host-prefixed private registry)", info.Category, ModuleCategoryRegistry)
	}
}

func TestParseModuleSource_LocalPath(t *testing.T) {
	info := ParseModuleSource("../shared-modules/logging")
	if info.Category != ModuleCategoryLocal {
		t.Fatalf("got category %s, want %s", info.Category, ModuleCategoryLocal)
	}
}

func TestParseModuleSource_ArchiveS3(t *testing.T) {
	info := ParseModuleSource("s3::https://s3-eu-west-1.amazonaws.com/consulbucket/monitoring.zip")
	if info.Category != ModuleCategoryArchive {
		t.Fatalf("got category %s, want %s", info.Category, ModuleCategoryArchive)
	}
}

func TestParseModuleSource_ArchiveHTTPSZip(t *testing.T) {
	info := ParseModuleSource("https://example.com/modules/legacy.zip")
	if info.Category != ModuleCategoryArchive {
		t.Fatalf("got category %s, want %s", info.Category, ModuleCategoryArchive)
	}
}

func TestParseModuleSource_UnknownMercurial(t *testing.T) {
	info := ParseModuleSource("hg::http://example.com/repo.hg")
	if info.Category != ModuleCategoryUnknown {
		t.Fatalf("got category %s, want %s", info.Category, ModuleCategoryUnknown)
	}
}

func TestParseModuleBlock_CollisionCase_NoKeyCollision(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "modules", "main.tf")
	body := parseBodyForTest(t, path)

	a := ParseModuleBlock(findModuleBlock(t, body, "collision_a"), path)
	b := ParseModuleBlock(findModuleBlock(t, body, "collision_b"), path)

	if a.GraphKey == "" || b.GraphKey == "" {
		t.Fatalf("expected non-empty graph keys, got a=%q b=%q", a.GraphKey, b.GraphKey)
	}
	if a.GraphKey == b.GraphKey {
		t.Fatalf("graph keys collided despite different sources: both %q — keying on ref alone would cause this", a.GraphKey)
	}
}

func TestParseModuleBlock_SameRepoDifferentPinState_DistinctKeys(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "modules", "main.tf")
	body := parseBodyForTest(t, path)

	pinned := ParseModuleBlock(findModuleBlock(t, body, "vpc_pinned"), path)
	unpinned := ParseModuleBlock(findModuleBlock(t, body, "vpc_unpinned"), path)

	if !pinned.Pinned {
		t.Error("expected vpc_pinned.Pinned=true")
	}
	if unpinned.Pinned {
		t.Error("expected vpc_unpinned.Pinned=false")
	}
	if pinned.GraphKey == unpinned.GraphKey {
		t.Fatalf("same repo, different pin state, but keys matched: %q", pinned.GraphKey)
	}
}

func TestParseModuleBlock_RegistryVersion(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "modules", "main.tf")
	body := parseBodyForTest(t, path)

	mb := ParseModuleBlock(findModuleBlock(t, body, "iam_role"), path)
	if mb.Source.Category != ModuleCategoryRegistry {
		t.Fatalf("got category %s, want %s", mb.Source.Category, ModuleCategoryRegistry)
	}
	if !mb.Pinned {
		t.Error("expected Pinned=true (version specified)")
	}
	want := "terraform-aws-modules/iam/aws@5.33.0"
	if mb.GraphKey != want {
		t.Errorf("got GraphKey %q, want %q", mb.GraphKey, want)
	}
}

func TestParseModuleBlock_LocalAndArchive_NoGraphKey(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "modules", "main.tf")
	body := parseBodyForTest(t, path)

	local := ParseModuleBlock(findModuleBlock(t, body, "shared_lib"), path)
	if local.GraphKey != "" {
		t.Errorf("local module: expected empty GraphKey, got %q", local.GraphKey)
	}

	archive := ParseModuleBlock(findModuleBlock(t, body, "monitoring"), path)
	if archive.GraphKey != "" {
		t.Errorf("archive module: expected empty GraphKey, got %q", archive.GraphKey)
	}
	if archive.Error != nil {
		t.Errorf("archive module: expected no error (recognized category, just unversioned), got %v", archive.Error)
	}
}

func TestParseModuleBlock_UnknownSourceType_HasError(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "modules", "main.tf")
	body := parseBodyForTest(t, path)

	mb := ParseModuleBlock(findModuleBlock(t, body, "hg_unknown"), path)
	if mb.Error == nil || mb.Error.Code != ErrUnknownSourceType {
		t.Fatalf("got error %v, want code %s", mb.Error, ErrUnknownSourceType)
	}
}

func findModuleBlock(t *testing.T, body *hclsyntax.Body, name string) *hclsyntax.Block {
	t.Helper()
	for _, b := range body.Blocks {
		if b.Type == "module" && len(b.Labels) == 1 && b.Labels[0] == name {
			return b
		}
	}
	t.Fatalf("module block %q not found", name)
	return nil
}
