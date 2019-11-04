package execute

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"syscall"

	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"golang.org/x/sync/semaphore"
)

type Repo struct {
	Id                     uuid.I   `json:"id"`
	Vid                    ulid.I   `json:"vid"`
	Registry               string   `json:"registry"`
	GlobalPath             string   `json:"globalPath"`
	File                   string   `json:"file,omitempty"`
	Shadow                 string   `json:"shadow,omitempty"`
	Archive                string   `json:"archive,omitempty"`
	ArchiveRecipients      []string `json:"archiveRecipients,omitempty"`
	ShadowBackup           string   `json:"shadowBackup,omitempty"`
	ShadowBackupRecipients []string `json:"shadowBackupRecipients,omitempty"`
}

type Config struct {
	Cmd     string
	CmdArgs []string
}

type Processor struct {
	lg      Logger
	cmd     string
	cmdArgs []string
	// `ctxSlow` gives background commands more time during graceful
	// shutdown.  The caller of `Run()` cancels `ProcessX(ctx)` immediately
	// and `ctxSlow` later, after a grace period.
	ctxSlow context.Context
	// Processor uses a semaphore with combined weight 1 to serialize
	// execution with context cancelation.  We use a semaphore, because a
	// simple `sync.Lock` does not support context.
	lock *semaphore.Weighted
}

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

func NewProcessor(ctxSlow context.Context, lg Logger, cfg *Config) *Processor {
	p := &Processor{
		lg:      lg,
		cmd:     cfg.Cmd,
		cmdArgs: cfg.CmdArgs,
		ctxSlow: ctxSlow,
		lock:    semaphore.NewWeighted(1),
	}
	if p.cmd == "" {
		p.cmd = "echo"
	}
	return p
}

// `ProcessRepoId()` is currently unused.  It is a reminder that we may
// consider adding alternative calling conventions.  For example, it may be
// faster to avoid fetching repo details from the registry and only pass the
// repo ID instead.
//
// Further ideas, which may be useful if process creation overhead turns out to
// be a problem:
//
//  - Pass multipe repos in a single run, like xargs.
//  - Pass repos via stdin, expecting the command to confirm processing on
//    stdout.
//
func (p *Processor) ProcessRepoId(ctx context.Context, repoId uuid.I) error {
	if err := p.lock.Acquire(ctx, 1); err != nil {
		return err
	}
	defer p.lock.Release(1)
	if err := p.runCmd(repoId.String()); err != nil {
		return err
	}
	return ctx.Err()
}

func (p *Processor) ProcessRepo(ctx context.Context, repo *Repo) error {
	if err := p.lock.Acquire(ctx, 1); err != nil {
		return err
	}
	defer p.lock.Release(1)

	arg, err := jsonMarshalStringRepo(repo)
	if err != nil {
		return err
	}
	if err := p.runCmd(string(arg)); err != nil {
		return err
	}

	return ctx.Err()
}

func jsonMarshalStringRepo(r *Repo) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return "", err
	}
	// `Encode()` writes a newline that we don't want.
	b.Truncate(b.Len() - 1)
	return b.String(), nil
}

// `runCmd()` handles most errors, so that callers of `ProcessX()` are isolated
// from command execution.  But it returns an error if the child has been
// signaled to tell the observer to skip saving the journal position during
// shutdown.
func (p *Processor) runCmd(arg string) error {
	args := append(p.cmdArgs, arg)
	cmd := exec.CommandContext(p.ctxSlow, p.cmd, args...)
	// Maybe pass repo details as JSON via stdin.
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if isShutdownSignal(err) {
		return err
	}

	// XXX Perhaps implement some kind of limited retry policy.
	if err != nil {
		p.lg.Errorw(
			"Command failed.",
			"cmd", p.cmd,
			"cmdArgs", args,
			"err", err,
		)
	}

	return nil
}

func isShutdownSignal(err error) bool {
	exit, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	status, ok := exit.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}

	// See <http://man7.org/linux/man-pages/man7/signal.7.html>
	sig := status.Signal()
	const SIGINT = 2
	const SIGTERM = 15
	return sig == SIGINT || sig == SIGTERM
}
