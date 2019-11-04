package main

import (
	"errors"
	slashpath "path"
)

var SkipTree = errors.New("skip tree")

type LifeCycle int

const (
	LcUnspecified LifeCycle = iota
	LcActive
	LcFrozen
	LcRetired // future
)

func (lc LifeCycle) Letter() string {
	switch lc {
	case LcActive:
		return "a"
	case LcFrozen:
		return "f"
	case LcRetired:
		return "r"
	case LcUnspecified:
		panic("unspecified LifeCycle")
	default:
		panic("invalid LifeCycle")
	}
}

type TreeInfo struct {
	Node      TreeNode
	Path      string
	LifeCycle LifeCycle
}

type TreeWalkFunc func(TreeInfo) error

func (s *Store) WalkTree(tree *Tree, cb TreeWalkFunc) error {
	return tree.walk(cb, s.levels)
}

func (t *Tree) walk(cb TreeWalkFunc, levels []*Level) error {
	if t.Root == nil {
		return nil
	}

	path := ""
	err := cb(TreeInfo{
		Node:      t.Root,
		Path:      path,
		LifeCycle: LcActive,
	})
	switch err {
	case nil: // ok, continue
	case SkipTree:
		return nil
	default:
		return err
	}

	return t.Root.walk(cb, levels, path, LcActive)
}

func (t *LevelTree) walk(
	cb TreeWalkFunc,
	levels []*Level,
	path string,
	lifeCycle LifeCycle,
) error {
	if t.level != levels[0] {
		err := errors.New("level mismatch")
		return err
	}

	for i, child := range t.Times {
		isLast := i == len(t.Times)-1
		childPath := slashpath.Join(path, child.Name())

		childLc := lifeCycle
		if !isLast {
			childLc = LcFrozen
		}
		err := cb(TreeInfo{
			Node:      child,
			Path:      childPath,
			LifeCycle: childLc,
		})
		switch err {
		case nil: // ok, enter child.
		case SkipTree:
			continue
		default:
			return err
		}

		if err := child.walk(
			cb, levels, childPath, childLc,
		); err != nil {
			return err
		}
	}

	return nil
}

func (t *TimeTree) walk(
	cb TreeWalkFunc,
	levels []*Level,
	path string,
	lifeCycle LifeCycle,
) error {
	// `WhereAppend()`, see `./tree.go`, always enters the first child when
	// looking for an archive parent.  Therefore, only the first child can
	// be active.  All other children are frozen.
	var activeChild *LevelTree
	for _, lv := range levels {
		if child, ok := t.SubLevels[lv.Name()]; ok {
			activeChild = child
			break
		}
	}

	// Iterate in reverse order to report highest frequency levels first,
	// which are also the oldest.
	for i := len(levels) - 1; i >= 0; i-- {
		lv := levels[i]
		lvName := lv.Name()
		child, ok := t.SubLevels[lvName]
		if !ok {
			continue
		}

		childPath := slashpath.Join(path, child.Name())
		childLc := lifeCycle
		if child != activeChild {
			childLc = LcFrozen
		}
		err := cb(TreeInfo{
			Node:      child,
			Path:      childPath,
			LifeCycle: childLc,
		})
		switch err {
		case nil: // ok, enter child.
		case SkipTree:
			continue
		default:
			return err
		}

		if err := child.walk(
			cb, levels[i:], childPath, childLc,
		); err != nil {
			return err
		}
	}
	return nil
}
