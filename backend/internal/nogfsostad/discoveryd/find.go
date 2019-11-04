package discoveryd

import (
	"errors"
	"fmt"

	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd/rules"
	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd/rulesdefault"
	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd/rulespatterns"
	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd/rulesstdtools"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrStdtoolsDisabled = errors.New("Rule Stdtools2017 disabled")

func (srv *Server) newFinder(
	rule string, cfg map[string]interface{},
) (rules.Finder, error) {
	switch rule {
	case "":
		return &rulesdefault.DirectSubdirFinder{}, nil

	case "SubdirLevel":
		level, err := getLevel(cfg)
		if err != nil {
			return nil, err
		}

		ignorePatterns, err := getStringList(cfg, "ignore")
		if err != nil {
			return nil, err
		}

		return &rulesdefault.SubdirLevelFinder{
			Level:          level,
			IgnorePatterns: ignorePatterns,
		}, nil

	case "PathPatterns":
		patterns, err := getStringList(cfg, "patterns")
		if err != nil {
			return nil, err
		}
		enabledPaths, err := getStringList(cfg, "enabledPaths")
		if err != nil {
			return nil, err
		}
		return rulespatterns.NewFinder(rulespatterns.Config{
			Patterns:     patterns,
			EnabledPaths: enabledPaths,
		})

	case "Stdtools2017":
		ignorePatterns, err := getStringList(cfg, "ignore")
		if err != nil {
			return nil, err
		}
		if srv.stdtoolsProjectsRoot == "" {
			return nil, ErrStdtoolsDisabled
		}
		return rulesstdtools.NewFinder(
			srv.stdtoolsProjectsRoot,
			ignorePatterns,
		)

	default:
		err := status.Errorf(
			codes.FailedPrecondition,
			"unknown repo naming rule `%s`", rule,
		)
		return nil, err
	}
}

func getStringList(cfg map[string]interface{}, key string) ([]string, error) {
	v, ok := cfg[key]
	if !ok {
		return nil, nil
	}

	slist, ok := v.([]string)
	if !ok {
		err := fmt.Errorf(
			"naming rule config field `%s` has wrong type", key,
		)
		return nil, err
	}

	return slist, nil
}

func getLevel(cfg map[string]interface{}) (int, error) {
	v, ok := cfg["level"]
	if !ok {
		err := errors.New("missing `level`")
		return 0, err
	}

	levelF, ok := v.(float64)
	if !ok {
		err := errors.New("`level` is not a number")
		return 0, err
	}

	level := int(levelF)
	if float64(level) != levelF {
		err := errors.New("`level` is not an integer")
		return 0, err
	}

	if level < 1 || level > 4 {
		err := errors.New("`level` out of range")
		return 0, err
	}

	return level, nil
}
