package shadows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	slashpath "path"
	"runtime"

	git "github.com/libgit2/git2go"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

type ListMetaTreeFunc func(info pb.PathMetadata) error

func (fs *Filesystem) ListMetaTree(
	ctx context.Context,
	shadowPath string,
	gitCommit []byte,
	callback ListMetaTreeFunc,
) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	repo, err := git.OpenRepository(shadowPath)
	if err != nil {
		return err
	}

	coId := git.NewOidFromBytes(gitCommit)
	if coId == nil {
		return errors.New("invalid commit id")
	}

	co, err := repo.LookupCommit(coId)
	if err != nil {
		return err
	}

	tree, err := co.Tree()
	if err != nil {
		return err
	}

	var walkErr error
	setWalkErr := func(path string, err error) {
		walkErr = fmt.Errorf("walk error at `%s`: %s", path, err)
	}

	// <https://libgit2.github.com/libgit2/#HEAD/group/tree/git_tree_walk>
	// Return codes:
	const (
		WalkContinue = 0
		WalkSkip     = 1
		WalkStop     = -1
	)
	// `path` is the tree path with trailing slash, empty string for root.
	walkFn := func(path string, ent *git.TreeEntry) int {
		if isIgnoredTreeEntry(ent) {
			return WalkSkip
		}

		if ent.Type == git.ObjectTree {
			// `callback()` is called when visiting the subtree
			// `.nogtree`.
			return WalkContinue
		}

		fullPath := slashpath.Join(path, ent.Name)

		pmd := pb.PathMetadata{}
		if ent.Name == ".nogtree" {
			if path == "" {
				pmd.Path = "."
			} else {
				pmd.Path = path
			}
		} else {
			switch ent.Filemode {
			case git.FilemodeBlobExecutable:
				fallthrough
			case git.FilemodeBlob:
				pmd.Path = fullPath

			case git.FilemodeLink:
				fallthrough
			case git.FilemodeCommit:
				fallthrough
			default:
				err = fmt.Errorf(
					"unexpected git object type `%s`",
					ent.Type,
				)
				setWalkErr(fullPath, err)
				return WalkStop
			}
		}

		metaJson, err := recodeMetaBlobYamlToJson(repo, ent.Id)
		if err != nil {
			setWalkErr(fullPath, err)
			return WalkStop
		}
		pmd.MetadataJson = metaJson

		if err := callback(pmd); err != nil {
			setWalkErr(fullPath, err)
			return WalkStop
		}
		return WalkContinue
	}

	// The `KeepAlive(tree)` below avoids potential segfaults.  See
	// `./tree.go` for details.
	tree.Walk(walkFn)
	runtime.KeepAlive(tree)
	return walkErr
}

func recodeMetaBlobYamlToJson(
	repo *git.Repository, oid *git.Oid,
) ([]byte, error) {
	blob, err := repo.LookupBlob(oid)
	if err != nil {
		return nil, err
	}
	return recodeMetaYamlToJson(blob.Contents())
}

// `recodeMetaYamlToJson()` recodes YAML to JSON.  The YAML must use the
// restricted format explained in NOE-13: `<key>: <json-val>` lines.
//
// The implementation explicitly splits lines and parses `<key>: <json-val>` to
// avoid the package `yaml`, which incorrectly unmarshals nested objects as
// `map[interface{}]interface{}`, see
// <https://github.com/go-yaml/yaml/issues/286>, which package `json` refuses
// to marshal.
func recodeMetaYamlToJson(yamlBytes []byte) ([]byte, error) {
	meta := make(map[string]interface{})
	lines := bytes.Split(bytes.TrimSpace(yamlBytes), []byte("\n"))
	for i, line := range lines {
		kv := bytes.SplitN(line, []byte(":"), 2)
		if len(kv) != 2 {
			err := fmt.Errorf(
				"failed to decode metadata line %d: "+
					"failed to split key-value", i+1,
			)
			return nil, err
		}

		var val interface{}
		if err := json.Unmarshal(kv[1], &val); err != nil {
			err := fmt.Errorf(
				"failed to decode metadata line %d: "+
					"failed to decode JSON value: %v",
				i+1, err,
			)
			return nil, err
		}

		meta[string(kv[0])] = val
	}

	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false) // Don't escape `<` and such.
	if err := e.Encode(meta); err != nil {
		err := fmt.Errorf("failed to encode meta JSON: %v", err)
		return nil, err
	}

	buf.Truncate(buf.Len() - 1) // Remove trailing newline.
	return buf.Bytes(), nil
}
