// Run with: go run ./test/fixtures/gen
// Creates the non-UTF8 and symlink-cycle fixtures, which can't be
// created by pasting text through a web UI.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root, err := os.Getwd()
	must(err)

	genNonUTF8(root)
	genSymlinkCycle(root)

	fmt.Println("fixtures generated")
}

func genNonUTF8(root string) {
	dir := filepath.Join(root, "test", "fixtures", "non-utf8")
	must(os.MkdirAll(dir, 0o755))

	content := append([]byte(`resource "aws_ecs_cluster" "main" {
  name = "bad-`), 0xff, 0xfe)
	content = append(content, []byte(`-encoding"
}
`)...)

	path := filepath.Join(dir, "invalid_encoding.tf")
	must(os.WriteFile(path, content, 0o644))
	fmt.Println("wrote", path)
}

func genSymlinkCycle(root string) {
	base := filepath.Join(root, "test", "fixtures", "symlink-test")
	realDir := filepath.Join(base, "real_dir")
	must(os.MkdirAll(realDir, 0o755))

	mainTf := filepath.Join(realDir, "main.tf")
	must(os.WriteFile(mainTf, []byte(`resource "aws_ecs_cluster" "main" {
  name = "symlink-fixture-cluster"
}
`), 0o644))

	loopLink := filepath.Join(base, "loop_dir")
	_ = os.Remove(loopLink) // safe to ignore if it doesn't exist yet
	must(os.Symlink(".", loopLink))

	fmt.Println("wrote", mainTf)
	fmt.Println("created symlink", loopLink, "-> .")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
