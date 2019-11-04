// vim: sw=8

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/pkg/mulog"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("tartt-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(strings.TrimSpace(`
Usage:
  tartt [-C <repo>] init [--store=<name>] --origin=<absdir> [--driver-localtape-tardir=<absdir>]
  tartt [-C <repo>] tar (--recipient=<gpgid>...|--plaintext-secret|--insecure-plaintext) [--cipher-algo=<cipher>] [--warning-fatal|--error-continue] [--store=<name>] [--lock-wait=<duration>] [--limit=<bandwidth>] [--full] [--full-hook=<cmd>]
  tartt [-C <repo>] sign [--no-skip-signed|--skip-good-from=<substring>] <tspaths>...
  tartt [-C <repo>] ls-tar [--no-lock] [--no-preload-secrets] [--notify-preload-secrets-done=<path>] [--limit=<bandwidth>] [--unquote] [-z] <tspath>
  tartt [-C <repo>] restore [--no-lock] [--no-preload-secrets] [--notify-preload-secrets-done=<path>] [--limit=<bandwidth>] [--no-same-owner] [--no-same-permissions] --dest=<emptydir> <tspath> [--] [<members>...]
  tartt [-C <repo>] ls [--no-lock]
  tartt [-C <repo>] gc [--dry-run] [--lock-wait=<duration>]
  tartt [-C <repo>] lock [--lock-wait=<duration>] [--] <cmd>...
  tartt [-C <repo>] backup (--recipient=<gpgid>...|--insecure-plaintext) [--full] [--limit=<bandwidth>] [--warning-fatal|--error-continue] [--full-hook=<cmd>]

Options:
  -C <repo>          Run as if tartt was started in ''<repo>''.
  --store=<name>     Name of the store.  ''init'' by default uses the hostname.
                     ''tar'' by default uses the first store.
  --origin=<absdir>  The directory to be archived.
  --dest=<emptydir>  Empty directory for restore.
  --lock-wait=<duration>  [default: 5s]
                     Maximum time to wait for a lock.
  --no-lock          Do not lock the store, which is safe with concurrent
                     append-only operations, specifically ''tar''.
  --dry-run          Print what would be removed without removing anything.
  --full             Force a full tar archive.
  --limit=<bandwidth>  Bandwidth limit in bytes per second on the uncompressed
                     tar stream.  ''k'', ''m'', ... can be used, which are
                     interpreted as binary SI.
  --recipient=<gpgid>  GPG keys to which to encrypt the archive secret.
  --plaintext-secret   Save archive secret as plaintext.
  --cipher-algo=<cipher>  [default: AES]
                     Passed via ''tartt-store'' to ''gpg --cipher-algo'' when
                     encrypting data.  Supported ciphers: AES, AES192, AES256.
                     Per-archive secret keys are always encrypted with AES256.
  --insecure-plaintext  Disable encryption.
  --no-preload-secrets  Disable decrypting all secrets during startup.
  --notify-preload-secrets-done=<file>  Write ''preload-secrets-done\n'' to
                     ''<file>'' after secret preloading has completed.
                     ''<file>'' must exist and usually is a FIFO.
  --full-hook=<cmd>  Run ''<cmd>'' when creating a full archive.  See below.
  --warning-fatal    Fail on warnings.
  --error-continue   Continue on non-fatal errors, like "tar: cannot open".
                     The default is to continue on warnings and fail on all
                     errors.
  --no-skip-signed   Do not skip signing if a signature file exists.
  --skip-good-from=<substring>  Skip signing if a good signature exists whose
                     primary user id contains ''<substring>''; "aka" lines are
                     ignored in the output of ''gpg --verify''.
                     The default is to skip signing if a signature file exists.
  --driver-localtape-tardir=<absdir>  Set the store driver to ''localtape'',
                     with the store-specific tape base directory ''<absdir>'',
                     to which tspaths are appended to construct individual
                     archive directories.
  --unquote          Unquote tar member names.
  -z                 Output line delimiter is NUL, not newline.

''tartt init'' initializes an empty directory as a tar time tree repo.

''tartt tar'' creates an archive of origin.  It must be run in a tar time tree
repo.  It uses the repo level configuration to decide whether to create a full
archive or an incremental archive.  A full archive can be forced with
''--full''.  Archives are written to disk with ''tartt-store''.  See
''tartt-store --help'' for details, including suggestions how to read the
low-level tar stream.

If the repo root contains a file ''exclude'', it is copied to the archive and
applied as an anchored exclude list: ''tar --anchored --exclude-from=exclude''.

''tartt tar'' exit codes: 0 complete success; 10 completed with warnings; 11
completed with non-fatal errors; 1 fatal errors.

''--full-hook=<cmd>'' is used when creating a full archive.  ''<cmd>'' is
evaluated in Bash in the ''.inprogress'' archive directory after tar has
completed and before metadata tar starts.  ''<cmd>'' may create files and print
their names to stdout, one per line, to tell ''tartt'' to include the files in
''metadata.tar''; ''tartt'' deletes the files after adding them to
''metadata.tar''.  As a special case, ''README.md'' will not be added to
''metadata.tar'' but included as plaintext in the general ''README.md''.

''tartt sign'' signs archive manifests with the default GPG key.  For example,
to sign all manifests that you have not yet signed:

    tartt ls | grep Z$ | cut -d $'\t' -f 2 \
    | xargs tartt sign --skip-good-from=<your-primary-email>

''tartt ls-tar'' lists archive members for the full and incremental archives
that lead to ''<tspath>''.  The output are lines:

    <archive-tspath> <colon> <space> <tar-member-name>

''<tar-member-names>'' are quoted in style "escape" as describe in the GNU tar
manual "Quoting Member Names".  Use ''--unquote'' for literal names.

Use ''-z'' to terminate lines with NUL instead of newline.  For example, to
create a list of paths that match a pattern:

    tspath=...
    rgx=...
    tartt ls-tar -z "${tspath}" 2>/dev/null \
    | grep -z "${rgx}" \
    | cut -z -d ' ' -f 2- | sort -z -u \
    | xargs -0 -n 1

''tartt restore'' restores files into the directory ''--dest'', which must be
an absolute path to an empty directory.  ''<tspath>'' is a path to an archive
as reported with ''tartt ls''.  The restore uses the full and incremental
archives that lead to ''<tspath>''.

You can use ''<members>'', which must use GNU tar quoting style "escape", to
restrict the paths to be restored.  GNU tar will report errors if ''<members>''
are missing in some archives; ''tartt'' logs such errors and ignores them,
because it is expected that files may be missing in incremental archives.  The
restore may nontheless be correct.

''tartt restore'' tries to restore the owner and permissions by default, even
if run as non-root.  Use ''--no-same-owner'' and ''--no-same-permissions'' to
control the behavior.

GnuPG must be able to decrypt archive secrets.  The recommended approach is
agent forwarding, see <https://wiki.gnupg.org/AgentForwarding>.  ''tartt'' will
by default decrypt all required secrets during startup and keep them in memory
until they are needed for untar.  The GnuPG agent can then be disconnected.
''--no-preload-secrets'' disables preloading; the GnuPG agent is contacted
right before each untar.

''tartt ls'' lists the archive tar time tree as lines:

    <lc> <size> <type> <tmin> <tmax><tab><path>

Where:

 - ''<lc>'' is the life cycle code: ''a'' active or ''f'' frozen.
 - ''<size>'' is the number of children.
 - ''<type>'' is the archive type, ''full'' or ''patch'', or level symbol,
   like ''mo1'', ''d1'', ''s0''.
 - ''<tmin>'' and ''<tmax>'' is the time range covered by the node and its
   children, as ISO UTC times.
 - ''<path>'' is the node directory path starting with the store name.

The fields may be separated by multiple spaces for alignment.

''tartt gc'' removes expired archives and unnecessary details from frozen
archives.

''tartt lock'' runs ''<cmd>...'' in the tartt repository while holding a lock
on the repo and all its stores.  The exit code is the exit code of ''<cmd>'';
or it is 1 if an error happens before starting ''<cmd>''.

`))

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
	Fatalw(msg string, kv ...interface{})
}

var lg Logger = mulog.Printer{}

func main() {
	args := argparse()

	if d, ok := args["-C"].(string); ok {
		if err := os.Chdir(d); err != nil {
			lg.Fatalw("Failed to apply -C.", "err", err)
		}
	}

	switch {
	case args["init"].(bool):
		cmdInit(args)
	case args["backup"].(bool):
		lg.Warnw("DEPRECATED command `backup`.  Use `tar` instead.")
		cmdTar(args)
	case args["tar"].(bool):
		cmdTar(args)
	case args["sign"].(bool):
		cmdSign(args)
	case args["ls-tar"].(bool):
		cmdLsTar(args)
	case args["restore"].(bool):
		cmdRestore(args)
	case args["ls"].(bool):
		cmdLs(args)
	case args["gc"].(bool):
		cmdGc(args)
	case args["lock"].(bool):
		os.Exit(cmdLock(args))
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
		"--limit",
	} {
		if arg, ok := args[k].(string); ok {
			v, err := parseUint64Si(arg)
			if err != nil {
				msg := fmt.Sprintf("Invalid %s.", k)
				lg.Fatalw(msg, "err", err)
			}
			args[k] = v
		}
	}

	for _, k := range []string{
		"--lock-wait",
	} {
		if arg, ok := args[k].(string); ok {
			v, err := time.ParseDuration(arg)
			if err != nil {
				msg := fmt.Sprintf("Invalid %s.", k)
				lg.Fatalw(msg, "err", err)
			}
			args[k] = v
		}
	}

	switch args["--cipher-algo"].(string) {
	case "AES", "AES192", "AES256":
		break // ok
	default:
		lg.Fatalw("Invalid --cipher-algo.")
	}

	return args
}

var siMap = map[string]uint64{
	"k": 1 << 10,
	"m": 1 << 20,
	"g": 1 << 30,
	"t": 1 << 40,
}

func parseUint64Si(s string) (uint64, error) {
	s = strings.ToLower(s)

	m := uint64(1)
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
		err := fmt.Errorf("must be positive, got %d", v)
		return 0, err
	}

	return uint64(v) * m, nil
}
