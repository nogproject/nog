// vim: sw=8

// Command `nogfsoctl` to control FSO registries and repos via `nogfsoregd` and
// `nogfsostad`.
package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/cmdeventsbroadcast"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/cmdeventsephreg"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/cmdeventsrepo"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/cmdeventsrepoworkflow"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/cmdeventsuxdom"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/cmdgitnog"
	"github.com/nogproject/nog/backend/pkg/gpg"
	"github.com/nogproject/nog/backend/pkg/mulog"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("nogfsoctl-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

const defaultCert = "~/.nogfso/certs/nogfsoctl/combined.pem"
const defaultCa = "~/.nogfso/certs/nogfsoctl/ca.pem"

var usage = qqBackticks(`Usage:
  nogfsoctl [options] init registry (--vid=<vid>|--no-vid) <registry>
  nogfsoctl [options] registry <registry> (--vid=<vid>|--no-vid) enable-ephemeral-workflows
  nogfsoctl [options] registry <registry> (--vid=<vid>|--no-vid) enable-propagate-root-acls
  nogfsoctl [options] init root --host=<host> [--gitlab-namespace=<path>] <registry> (--vid=<vid>|--no-vid) <root> [<host-root>]
  nogfsoctl [options] remove root <registry> (--vid=<vid>|--no-vid) <root>
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> enable-gitlab <gitlab-namespace>
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> disable-gitlab
  nogfsoctl [options] root <registry> <root> find-untracked
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> set-repo-naming <rule> [<configmap>]
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> add-repo-naming-ignore <rule> <patterns>...
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> enable-discovery-paths <depth-paths>...
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> set-init-policy subdir-tracking-globlist <subdir-tracking-globs>...
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> enable-archive-encryption <gpg-keys>...
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> disable-archive-encryption
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> enable-shadow-backup-encryption <gpg-keys>...
  nogfsoctl [options] root <registry> (--vid=<vid>|--no-vid) <root> disable-shadow-backup-encryption
  nogfsoctl [options] init repo --author=<user> [--uuid=<uuid>] <registry> (--vid=<vid>|--no-vid) <repo>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> enable-gitlab <gitlab-namespace>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> begin-move-repo --workflow=<uuid> [--unchanged-global-path] <new-global-path>
  nogfsoctl [options] repo <repoid> (--vid=<vid>|--no-vid) enable-archive-encryption <gpg-keys>...
  nogfsoctl [options] repo <repoid> (--vid=<vid>|--no-vid) disable-archive-encryption
  nogfsoctl [options] repo <repoid> (--vid=<vid>|--no-vid) enable-shadow-backup-encryption <gpg-keys>...
  nogfsoctl [options] repo <repoid> (--vid=<vid>|--no-vid) disable-shadow-backup-encryption
  nogfsoctl [options] repo <repoid> (--vid=<vid>|--no-vid) begin-move-shadow --workflow=<uuid> <new-shadow-path>
  nogfsoctl [options] repo <repoid> commit-move-shadow --workflow=<uuid> (--vid=<vid>|--no-workflow-vid)
  nogfsoctl [options] repo <repoid> (--vid=<vid>|--no-vid) init-tartt <tartt-url>
  nogfsoctl [options] repo <repoid> (--vid=<vid>|--no-vid) init-shadow-backup <shadow-backup-url>
  nogfsoctl [options] repo <repoid> (--vid=<vid>|--no-vid) move-shadow-backup <shadow-backup-url>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> [--repo-vid=<vid>] freeze [--wait=<duration>] --workflow=<uuid> --author=<user>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> [--repo-vid=<vid>] begin-freeze --workflow=<uuid> --author=<user>
  nogfsoctl [options] repo <registry> <repoid> get-freeze [--wait=<duration>] <workflowid>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> [--repo-vid=<vid>] unfreeze [--wait=<duration>] --workflow=<uuid> --author=<user>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> [--repo-vid=<vid>] begin-unfreeze --workflow=<uuid> --author=<user>
  nogfsoctl [options] repo <registry> <repoid> get-unfreeze [--wait=<duration>] <workflowid>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> [--repo-vid=<vid>] archive [--wait=<duration>] --workflow=<uuid> --author=<user>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> [--repo-vid=<vid>] begin-archive --workflow=<uuid> --author=<user>
  nogfsoctl [options] repo <registry> <repoid> get-archive [--wait=<duration>] <workflowid>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> [--repo-vid=<vid>] unarchive [--wait=<duration>] --workflow=<uuid> --author=<user>
  nogfsoctl [options] repo <registry> (--vid=<vid>|--no-vid) <repoid> [--repo-vid=<vid>] begin-unarchive --workflow=<uuid> --author=<user>
  nogfsoctl [options] repo <registry> <repoid> get-unarchive [--wait=<duration>] <workflowid>
  nogfsoctl [options] clear-error <repoid> <errmsg>
  nogfsoctl [options] reinit repo --reason=<msg> <registry> (--vid=<vid>|--no-vid) <repoid>
  nogfsoctl [options] get registries
  nogfsoctl [options] info <registry>
  nogfsoctl [options] get roots <registry>
  nogfsoctl [options] get root <registry> <root>
  nogfsoctl [options] get repos [--global-path-prefix=<prefix>] <registry>
  nogfsoctl [options] get repo <repoid>
  nogfsoctl [options] events broadcast [--watch] [--after=<vid>] [--after-now]
  nogfsoctl [options] events registry [--watch] [--after=<vid>] <registry>
  nogfsoctl [options] events repo [--watch] [--after=<vid>] <repoid>
  nogfsoctl [options] events repo-workflow [--watch] [--after=<vid>] <repoid> <workflowid>
  nogfsoctl [options] events du [--watch] [--after=<vid>] <registry> <root> <workflowid>
  nogfsoctl [options] events ping-registry [--watch] [--after=<vid>] <registry> <workflowid>
  nogfsoctl [options] events split-root [--watch] [--after=<vid>] <registry> <root> <workflowid>
  nogfsoctl [options] events freeze-repo [--watch] [--after=<vid>] <registry> <repoid> <workflowid>
  nogfsoctl [options] events unfreeze-repo [--watch] [--after=<vid>] <registry> <repoid> <workflowid>
  nogfsoctl [options] events archive-repo [--watch] [--after=<vid>] <registry> <repoid> <workflowid>
  nogfsoctl [options] events unarchive-repo [--watch] [--after=<vid>] <registry> <repoid> <workflowid>
  nogfsoctl [options] events unix-domain [--watch] [--after=<vid>] <domain>
  nogfsoctl [options] stat-status [--stad] <repoid>
  nogfsoctl [options] stat [--stad] [--wait=<duration>] [--mtime-range-only] --author=<user> <repoid>
  nogfsoctl [options] sha [--stad] [--wait=<duration>] --author=<user> <repoid>
  nogfsoctl [options] refresh content [--wait=<duration>] --author=<user> <repoid>
  nogfsoctl [options] reinit-subdir-tracking [--stad] [--wait=<duration>] --author=<user> <repoid> (enter-subdirs|bundle-subdirs|ignore-subdirs|ignore-most)
  nogfsoctl [options] gitnog [--regd|--g2nd] head <repoid>
  nogfsoctl [options] gitnog [--regd|--g2nd] summary <repoid>
  nogfsoctl [options] gitnog [--regd|--g2nd] meta <repoid>
  nogfsoctl [options] gitnog [--regd|--g2nd] putmeta [--old-commit=<id>] --author=<user> --message=<msg> <repoid> <kvs>...
  nogfsoctl [options] gitnog put-path-metadata --author=<user> --message=<msg> [--old-commit=<id>] [--old-meta-git-commit=<id>] <repoid> <path-metadata>...
  nogfsoctl [options] gitnog [--regd|--g2nd] content <repoid> <path>
  nogfsoctl [options] ls-stat-tree <repoid> <git-commit> [<prefix>]
  nogfsoctl [options] ls-meta-tree <repoid> <git-commit>
  nogfsoctl [options] tartt head <repoid>
  nogfsoctl [options] tartt config [--verbose] <repoid> [<git-commit>]
  nogfsoctl [options] tartt ls [--verbose] [--sha] <repoid> [<git-commit>]
  nogfsoctl [options] du begin --workflow=<uuid> root <registry> (--vid=<vid>|--no-vid) <root>
  nogfsoctl [options] du get [--verbose] [--wait=<duration>] --workflow=<uuid> root <registry> <root>
  nogfsoctl [options] ping-registry begin <registry> (--vid=<vid>|--no-vid) --workflow=<uuid>
  nogfsoctl [options] ping-registry commit <registry> --workflow=<uuid> (--vid=<vid>|--no-vid)
  nogfsoctl [options] ping-registry get [--wait=<duration>] <registry> --workflow=<uuid>
  nogfsoctl [options] split-root enable-root <registry> (--vid=<vid>|--no-vid) <root> [--max-depth=<depth>] [--min-du=<size>] [--max-du=<size>]
  nogfsoctl [options] split-root disable-root <registry> (--vid=<vid>|--no-vid) <root>
  nogfsoctl [options] split-root dont-split <registry> (--vid=<vid>|--no-vid) <root> <path>
  nogfsoctl [options] split-root allow-split <registry> (--vid=<vid>|--no-vid) <root> <path>
  nogfsoctl [options] split-root config <registry> <root>
  nogfsoctl [options] split-root begin <registry> (--vid=<vid>|--no-vid) <root> --workflow=<uuid>
  nogfsoctl [options] split-root get [--wait=<duration>] <registry> <root> <workflowid>
  nogfsoctl [options] split-root decide <registry> <root> <workflowid> (--vid=<vid>|--no-vid) [--author=<user>] [--init-repo=<path>...] [--never-split=<path>...] [--ignore-once=<path>...]
  nogfsoctl [options] split-root commit <registry> <root> <workflowid> (--vid=<vid>|--no-vid)
  nogfsoctl [options] split-root abort <registry> <root> <workflowid> (--vid=<vid>|--no-vid)
  nogfsoctl [options] test-udo [--as-user=<user>] <global-path>
  nogfsoctl [options] init unix-domain (--vid=<vid>|--no-vid) <domain>
  nogfsoctl [options] get unix-domain <domain>
  nogfsoctl [options] unix-domain <domain> (--vid=<vid>|--no-vid) create-group <group> <gid>
  nogfsoctl [options] unix-domain <domain> (--vid=<vid>|--no-vid) delete-group <gid>
  nogfsoctl [options] unix-domain <domain> (--vid=<vid>|--no-vid) create-user <user> <uid> <gid>
  nogfsoctl [options] unix-domain <domain> (--vid=<vid>|--no-vid) delete-user <uid>
  nogfsoctl [options] unix-domain <domain> (--vid=<vid>|--no-vid) add-group-user <gid> <uid>
  nogfsoctl [options] unix-domain <domain> (--vid=<vid>|--no-vid) remove-group-user <gid> <uid>

Options:
  --nogfsoregd=<addr>  [default: localhost:7550]
  --nogfsostad=<addr>  [default: localhost:7552]
  --nogfsog2nd=<addr>  [default: localhost:7554]
  --tls-cert=<pem>  [default: ` + defaultCert + `]
        TLS certificate and corresponding private key.  PEM files can be
        concatenated ''cat cert.pem privkey.pem > combined.pem''.
  --tls-ca=<pem>  [default: ` + defaultCa + `]
        TLS certificates that are accepted as CA for client certs.  Multiple
        PEM files can be concatenated.
  --jwt=<path>  [default: /nog/jwt/tokens/admin.jwt]
        Path of the JWT for GRPCs.
  --jwt-auth=<url>  [default: NOG_API_URL]
	URL of the API endpoint to contact to retrieve per-request JWTs for
	operations that may involve ''nogfsostad''.  ''<url>'' is either a URL
	like ''https://nog.zib.de/api/v1/fso/auth'' or one of the following
	special values: ''NOG_API_URL'' to construct the URL from the
	corresponding environment variable; ''no'' to disable per-request JWTs.
  --host=<host>  File host that manages the ''<root>''.
        Example: ''files.example.org''.
  --gitlab-namespace=<path>  GitLab ''<host>/<user>'' or ''<host>/<group>''.
        Repos will be created below.  Example: ''git.example.org/topic''.
  --author=<user>  Git author for commits.
        Example: ''A U Thor <author@example.org>''.
  --mtime-range-only  Run ''git-fso stat --mtime-range-only''.
  --watch  Wait for more events and print them as they arrive.
  --after=<vid>  List events after event version ''<vid>''.
  --after-now  Return only events that arrive after now.  Implies --watch.
  --regd  Contact --nogfsoregd.  This is the default.
  --stad  Contact --nogfsostad directly and not via --nogfsoregd.
  --g2nd  Contact --nogfsog2nd instead of --nogfsoregd.
  --reason=<msg>  Reason for repo reinit.
  --vid=<vid>  Reject command if the aggregate version differs from ''<vid>''.
  --no-vid  Disable aggregate version check.
  --old-commit=<id>  Fail if the current Git Nog commit is differs.
  --old-meta-git-commit=<id>  Fail if the current meta Git commit differs.
  --uuid=<uuid>  Use this UUID instead of generating one.
  --workflow=<uuid>  Workflow ID, used to group related events.
  --unchanged-global-path  Move to unchanged global path, which can be used to
        move the repo to a new host path if the root config has changed.
  -v, --verbose  Print more details.
  --sha          Print file SHAs.

''<kvs>'' are ''key=value'' pairs.  Values are stored as strings.

''<path-metadata>'' are ''<path>=<json>'' pairs.  ''<json>'' is a JSON object
that will be stored as metadata for ''<path>''.  Use an empty JSON object
''<path>={}'' to delete metadata.  Use a trailing slash ''/'' on ''<path>'' to
store metadata for a directory.  Use a single dot ''.'' to store metadata for
the repo root.

''add-repo-naming-ignore'' expects glob ''<patterns>'' that are matched using
Go's ''path.Match()'' against relative paths below the root without leading dot
or slash.

''<configmap>'' is a JSON object that controls details of the repo naming rule.
Its structure depends on the naming rule.

''<subdir-tracking-globs>'' is a list of ''<subdir-tracking>:<glob>'' that
specifies how subdir tracking is configured during repo initialization.
''<subdir-tracking>'' can be ''enter-subdirs'', ''bundle-subdirs',
''ignore-subdirs'', or ''ignore-most''.  The first glob that matches the
root-relative repo path decides.  Use ''.'' to indicate the root itself.  The
default is ''enter-subdirs''.  See ''git-fso --help'' for details.

''<depth-paths>'' is a list of ''<depth>:<path>'' pairs.  Depth 0 enables only
the path, depth 1 enables the path and direct sub-directories, and so on.

''<gpg-keys>'' is a list of GPG key fingerprints, formatted as 40-digit hex
numbers.
`)

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
	Fatalw(msg string, kv ...interface{})
}

var lg Logger = mulog.Printer{}

func main() {
	args := argparse()
	switch {
	case args["events"].(bool) && args["unix-domain"].(bool):
		cmdeventsuxdom.Cmd(lg, args)
	case args["unix-domain"].(bool):
		cmdUnixDomain(args)
	case args["events"].(bool) && args["du"].(bool):
		cmdeventsephreg.Cmd(lg, args, cmdeventsephreg.Du)
	case args["du"].(bool):
		cmdDu(args)
	case args["events"].(bool) && args["ping-registry"].(bool):
		cmdeventsephreg.Cmd(lg, args, cmdeventsephreg.PingRegistry)
	case args["ping-registry"].(bool):
		cmdPingRegistry(args)
	case args["events"].(bool) && args["split-root"].(bool):
		cmdeventsephreg.Cmd(lg, args, cmdeventsephreg.SplitRoot)
	case args["split-root"].(bool):
		cmdSplitRoot(args)
	case args["init"].(bool):
		cmdInit(args)
	case args["remove"].(bool) && args["root"].(bool):
		cmdRemoveRoot(args)
	case args["info"].(bool):
		cmdInfo(args)
	case args["get"].(bool):
		cmdGet(args)
	case args["events"].(bool) && args["broadcast"].(bool):
		cmdeventsbroadcast.Cmd(lg, args)
	case args["events"].(bool) && args["repo"].(bool):
		cmdeventsrepo.Cmd(lg, args)
	case args["events"].(bool) && args["repo-workflow"].(bool):
		cmdeventsrepoworkflow.Cmd(lg, args)
	case args["events"].(bool) && args["registry"].(bool):
		cmdEventsRegistry(args)
	case args["events"].(bool) && args["freeze-repo"].(bool):
		cmdeventsephreg.Cmd(lg, args, cmdeventsephreg.FreezeRepo)
	case args["events"].(bool) && args["unfreeze-repo"].(bool):
		cmdeventsephreg.Cmd(lg, args, cmdeventsephreg.UnfreezeRepo)
	case args["events"].(bool) && args["archive-repo"].(bool):
		cmdeventsephreg.Cmd(lg, args, cmdeventsephreg.ArchiveRepo)
	case args["events"].(bool) && args["unarchive-repo"].(bool):
		cmdeventsephreg.Cmd(lg, args, cmdeventsephreg.UnarchiveRepo)
	case args["stat-status"].(bool):
		cmdStatStatus(args)
	case args["stat"].(bool):
		cmdStat(args)
	case args["sha"].(bool):
		cmdSha(args)
	case args["refresh"].(bool) && args["content"].(bool):
		cmdRefreshContent(args)
	case args["reinit-subdir-tracking"].(bool):
		cmdReinitSubdirTracking(args)
	case args["clear-error"].(bool):
		cmdClearError(args)
	case args["reinit"].(bool):
		cmdReinit(args)
	case args["ls-stat-tree"].(bool):
		cmdLsStatTree(args)
	case args["ls-meta-tree"].(bool):
		cmdLsMetaTree(args)
	case args["put-path-metadata"].(bool):
		cmdPutPathMetadata(args)
	case args["gitnog"].(bool):
		cmdgitnog.Cmd(lg, args)
	case args["registry"].(bool):
		cmdRegistry(args)
	case args["root"].(bool):
		cmdRoot(args)
	case args["repo"].(bool):
		cmdRepo(args)
	case args["tartt"].(bool):
		cmdTartt(args)
	case args["test-udo"].(bool):
		cmdTestUdo(args)
	default:
		panic("unhandled args")
	}
}

func argparse() map[string]interface{} {
	const autoHelp = true
	const noOptionFirst = false
	args, err := docopt.Parse(
		usage, nil, autoHelp, version, noOptionFirst,
	)
	if err != nil {
		lg.Fatalw("docopt failed.", "err", err)
	}

	for _, k := range []string{
		"--after",
		"--vid",
		"--repo-vid",
		"<opid>",
	} {
		if arg, ok := args[k].(string); ok {
			if v, err := ulid.Parse(arg); err != nil {
				msg := fmt.Sprintf("Invalid ULID %s.", k)
				lg.Fatalw(msg, "err", err)
			} else {
				args[k] = v
			}
		}
	}

	if idHex, ok := args["--old-commit"].(string); ok {
		idBytes, err := hex.DecodeString(idHex)
		if err != nil {
			lg.Fatalw(
				"--old-commit must be a hex string",
				"err", err,
			)
		}
		args["--old-commit"] = idBytes
	}

	if idHex, ok := args["--old-meta-git-commit"].(string); ok {
		idBytes, err := hex.DecodeString(idHex)
		if err != nil {
			lg.Fatalw(
				"--old-meta-git-commit must be a hex string",
				"err", err,
			)
		}
		args["--old-meta-git-commit"] = idBytes
	}

	if a, ok := args["<repoid>"].(string); ok {
		id, err := uuid.Parse(a)
		if err != nil {
			lg.Fatalw(
				"<repoid> must be a UUID.",
				"err", err,
			)
		}
		args["<repoid>"] = id
	}

	if a, ok := args["<workflowid>"].(string); ok {
		id, err := uuid.Parse(a)
		if err != nil {
			lg.Fatalw(
				"<workflowid> must be a UUID.",
				"err", err,
			)
		}
		args["<workflowid>"] = id
	}

	if a, ok := args["--uuid"].(string); ok {
		id, err := uuid.Parse(a)
		if err != nil {
			lg.Fatalw(
				"--uuid must be a UUID.",
				"err", err,
			)
		}
		args["--uuid"] = id
	}

	if a, ok := args["--workflow"].(string); ok {
		id, err := uuid.Parse(a)
		if err != nil {
			lg.Fatalw(
				"--workflow must be a UUID.",
				"err", err,
			)
		}
		args["--workflow"] = id
	}

	if args["--tls-cert"].(string) == defaultCert {
		usr, err := user.Current()
		if err != nil {
			lg.Fatalw(
				"Failed to determine current user.",
				"err", err,
			)
		}
		args["--tls-cert"] = filepath.Join(
			usr.HomeDir, defaultCert[2:],
		)
	}

	if args["--tls-ca"].(string) == defaultCa {
		usr, err := user.Current()
		if err != nil {
			lg.Fatalw(
				"Failed to determine current user.",
				"err", err,
			)
		}
		args["--tls-ca"] = filepath.Join(usr.HomeDir, defaultCa[2:])
	}

	if idHex, ok := args["<git-commit>"].(string); ok {
		idBytes, err := hex.DecodeString(idHex)
		if err != nil {
			lg.Fatalw(
				"<git-commit> must be a hex string",
				"err", err,
			)
		}
		args["<git-commit>"] = idBytes
	}

	if !args["--g2nd"].(bool) {
		args["--regd"] = true
	}

	args["<kvs>"] = mustParseKvs(args["<kvs>"])

	args["<subdir-tracking-globs>"] = mustParseSubdirTrackingGlobs(
		args["<subdir-tracking-globs>"],
	)

	if args["--after-now"].(bool) {
		args["--watch"] = true
	}

	if arg, ok := args["--jwt-auth"]; ok {
		switch {
		case arg == "no":
			args["--jwt-auth"] = nil
		case arg == "NOG_API_URL":
			api := os.Getenv("NOG_API_URL")
			if api == "" {
				lg.Fatalw("NOG_API_URL not in environment.")
			}
			args["--jwt-auth"] = fmt.Sprintf(
				"%s/v1/fso/auth", api,
			)
		}
	}

	if w, ok := args["--wait"].(string); ok {
		d, err := time.ParseDuration(w)
		if err != nil {
			lg.Fatalw("Invalid --wait", "err", err)
		}
		args["--wait"] = d
	}

	// Positive int32.
	for _, k := range []string{
		"--max-depth",
		"<uid>",
		"<gid>",
	} {
		if arg, ok := args[k].(string); ok {
			v, err := strconv.ParseInt(arg, 10, 32)
			if err != nil || v <= 0 {
				msg := fmt.Sprintf("Invalid %s.", k)
				lg.Fatalw(msg, "err", err)
			} else {
				args[k] = int32(v)
			}
		}
	}

	// Size with SI unit as int64.
	for _, k := range []string{
		"--min-du",
		"--max-du",
	} {
		if arg, ok := args[k].(string); ok {
			if v, err := parseInt64SiNonNegative(arg); err != nil {
				msg := fmt.Sprintf("Invalid %s.", k)
				lg.Fatalw(msg, "err", err)
			} else {
				args[k] = v
			}
		}
	}

	for _, k := range []string{
		"<gpg-keys>",
	} {
		if arg, ok := args[k].([]string); ok {
			if v, err := gpg.ParseFingerprintsHex(arg...); err != nil {
				msg := fmt.Sprintf("Invalid %s.", k)
				lg.Fatalw(msg, "err", err)
			} else {
				args[k] = v
			}
		}
	}

	return args
}

// `mustParseKvs` converts a list of `<key>=<val>` strings to a list of
// `[<key>, <val>]` pairs.
func mustParseKvs(arg interface{}) [][2]string {
	kvs := [][2]string{}
	for _, a := range arg.([]string) {
		kv := strings.SplitN(a, "=", 2)
		if len(kv) != 2 {
			err := fmt.Errorf("failed to parse `%s`", a)
			lg.Fatalw(
				"Failed to parse <kvs>.",
				"err", err,
			)
		}
		kvs = append(kvs, [2]string{kv[0], kv[1]})
	}
	return kvs
}

// `mustParseSubdirTrackingGlobs()` converts a list of `<a>:<b>` strings to a
// list of `[<a>, <b>]` pairs.
func mustParseSubdirTrackingGlobs(arg interface{}) [][2]string {
	pairs := [][2]string{}
	for _, a := range arg.([]string) {
		pair := strings.SplitN(a, ":", 2)
		if len(pair) != 2 {
			err := fmt.Errorf(
				"failed to parse `%s`: missing colon", a,
			)
			lg.Fatalw(
				"Failed to parse `<subdir-tracking-globs>`.",
				"err", err,
			)
		}
		switch pair[0] {
		case "enter-subdirs":
		case "bundle-subdirs":
		case "ignore-subdirs":
		case "ignore-most":
		default:
			err := fmt.Errorf(
				"failed to parse `%s`: "+
					"invalid <subdir-tracking>", a,
			)
			lg.Fatalw(
				"Failed to parse `<subdir-tracking-globs>`.",
				"err", err,
			)
		}
		pairs = append(pairs, [2]string{pair[0], pair[1]})
	}
	return pairs
}

var siMap = map[string]int64{
	"k": 1 << 10,
	"m": 1 << 20,
	"g": 1 << 30,
	"t": 1 << 40,
}

func parseInt64SiNonNegative(s string) (int64, error) {
	s = strings.ToLower(s)

	m := int64(1)
	for suf, mult := range siMap {
		if strings.HasSuffix(s, suf) {
			m = mult
			s = s[0 : len(s)-len(suf)]
			break
		}
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if v < 0 {
		return 0, errors.New("must be positive")
	}
	if v > math.MaxInt64/m {
		return 0, errors.New("int overflow")
	}

	return v * m, nil
}
