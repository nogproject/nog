// vim: sw=8

// Command `test-git2go` tests that package git2go works.
package main

import (
	"fmt"
	"log"

	docopt "github.com/docopt/docopt-go"
	git "github.com/libgit2/git2go"
)

const version = "0.0.0"

var usage = `Usage:
  test-git2go

List Git refs in the current working directory using libgit2/git2go.
`

func main() {
	const autoHelp = true
	const noOptionFirst = false
	args, err := docopt.Parse(usage, nil, autoHelp, version, noOptionFirst)
	must(err)
	_ = args

	repo, err := git.OpenRepository(".")
	must(err)

	iter, err := repo.NewReferenceIterator()
	must(err)

	names := iter.Names()
	for {
		name, err := names.Next()
		if git.IsErrorCode(err, git.ErrIterOver) {
			break
		}
		must(err)
		fmt.Println(name)
	}
}

func must(err error) {
	if err == nil {
		return
	}
	log.Fatal(err)
}
