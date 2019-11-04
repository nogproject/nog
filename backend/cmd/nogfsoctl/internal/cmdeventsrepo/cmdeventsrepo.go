package cmdeventsrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/internal/fsoauthz"
	pbevents "github.com/nogproject/nog/backend/internal/fsorepos/pbevents"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const AAFsoReadRepo = fsoauthz.AAFsoReadRepo

type Event struct {
	Event  string `json:"event"`
	Id     string `json:"id"`
	Parent string `json:"parent"`
	Etime  string `json:"etime"`

	Note                  string `json:"note,omitempty"`
	*RepoInitInfo         `json:"repoInitInfo,omitempty"`
	*ShadowRepoInfo       `json:"shadowRepoInfo,omitempty"`
	*ArchiveRepoInfo      `json:"archiveRepoInfo,omitempty"`
	*ShadowBackupRepoInfo `json:"shadowBackupRepoInfo,omitempty"`
	*GitRepoInfo          `json:"gitRepoInfo,omitempty"`
	GitAuthor             *GitUser `json:"gitAuthor,omitempty"`
	RegistryEventId       string   `json:"registryEventId,omitempty"`
	WorkflowId            string   `json:"workflowId,omitempty"`
	WorkflowEventId       string   `json:"workflowEventId,omitempty"`
	GlobalPath            string   `json:"globalPath,omitempty"`
	FileHost              string   `json:"fileHost,omitempty"`
	HostPath              string   `json:"hostPath,omitempty"`
	ShadowPath            string   `json:"shadowPath,omitempty"`
	OldGlobalPath         string   `json:"oldGlobalPath,omitempty"`
	OldFileHost           string   `json:"oldFileHost,omitempty"`
	OldHostPath           string   `json:"oldHostPath,omitempty"`
	OldShadowPath         string   `json:"oldShadowPath,omitempty"`
	NewGlobalPath         string   `json:"newGlobalPath,omitempty"`
	NewFileHost           string   `json:"newFileHost,omitempty"`
	NewHostPath           string   `json:"newHostPath,omitempty"`
	GpgKeyFingerprints    []string `json:"gpgKeyFingerprints,omitempty"`
	ErrorMessage          string   `json:"errorMessage,omitempty"`
	StatusCode            int32    `json:"statusCode,omitempty"`
	StatusMessage         string   `json:"statusMessage,omitempty"`
}

type RepoInitInfo struct {
	Registry       string `json:"registry"`
	GlobalPath     string `json:"globalPath,omitempty"`
	CreatorName    string `json:"creatorName,omitempty"`
	CreatorEmail   string `json:"creatorEmail,omitempty"`
	FileHost       string `json:"fileHost,omitempty"`
	HostPath       string `json:"hostPath,omitempty"`
	GitlabHost     string `json:"GitlabHost,omitempty"`
	GitlabPath     string `json:"GitlabPath,omitempty"`
	GitToNogAddr   string `json:"GitToNogAddr,omitempty"`
	SubdirTracking string `json:"subdirTracking,omitempty"`
}

func RepoInitInfoFromPb(inf pb.FsoRepoInitInfo) RepoInitInfo {
	return RepoInitInfo{
		Registry:       inf.Registry,
		GlobalPath:     inf.GlobalPath,
		CreatorName:    inf.CreatorName,
		CreatorEmail:   inf.CreatorEmail,
		FileHost:       inf.FileHost,
		HostPath:       inf.HostPath,
		GitlabHost:     inf.GitlabHost,
		GitlabPath:     inf.GitlabPath,
		GitToNogAddr:   inf.GitToNogAddr,
		SubdirTracking: inf.SubdirTracking.String(),
	}
}

type ShadowRepoInfo struct {
	ShadowPath    string `json:"shadowPath,omitempty"`
	NewShadowPath string `json:"newShadowPath,omitempty"`
}

type ArchiveRepoInfo struct {
	ArchiveURL string `json:"archiveUrl"`
}

type ShadowBackupRepoInfo struct {
	ShadowBackupURL string `json:"shadowBackupUrl"`
}

type GitRepoInfo struct {
	GitlabProjectId int64 `json:"gitlabProjectId"`
}

type GitUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
	Fatalw(msg string, kv ...interface{})
}

func Cmd(lg Logger, args map[string]interface{}) {
	ctx := context.Background()

	conn, err := connect.DialX509(
		args["--nogfsoregd"].(string),
		args["--tls-cert"].(string),
		args["--tls-ca"].(string),
	)
	if err != nil {
		lg.Fatalw("Failed to dial nogfsoregd.", "err", err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			lg.Errorw("Failed to close conn.", "err", err)
		}
	}()

	c := pb.NewReposClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	req := pb.RepoEventsI{
		Repo: uuI[:],
	}
	if a, ok := args["--after"].(ulid.I); ok {
		req.After = a[:]
	}
	if args["--watch"].(bool) {
		// Don't timeout during watch.
		req.Watch = true
	} else {
		ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		ctx = ctxTimeout
	}

	creds, err := connect.GetRPCCredsRepoId(ctx, args, AAFsoReadRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	stream, err := c.Events(ctx, &req, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	jsonOut := json.NewEncoder(os.Stdout)
	jsonOut.SetEscapeHTML(false)
	for {
		rsp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			lg.Fatalw("Stream recv failed.", "err", err)
		}
		for _, ev := range rsp.Events {
			id, err := ulid.ParseBytes(ev.Id)
			if err != nil {
				lg.Fatalw("Failed to parse Id.", "err", err)
			}
			parent, err := ulid.ParseBytes(ev.Parent)
			if err != nil {
				lg.Fatalw(
					"Failed to parse Parent.", "err", err,
				)
			}
			outev := Event{
				Event:  ev.Event.String(),
				Id:     id.String(),
				Parent: parent.String(),
				Etime:  ulid.TimeString(id),
			}
			switch ev.Event {
			case pb.RepoEvent_EV_FSO_REPO_INIT_STARTED:
				inf := RepoInitInfoFromPb(*ev.FsoRepoInitInfo)
				outev.RepoInitInfo = &inf

			case pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED:
				inf := RepoInitInfoFromPb(*ev.FsoRepoInitInfo)
				outev.RepoInitInfo = &inf

			case pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED:
				evi := ev.FsoShadowRepoInfo
				outev.ShadowRepoInfo = &ShadowRepoInfo{
					ShadowPath: evi.ShadowPath,
				}

			case pb.RepoEvent_EV_FSO_REPO_MOVE_STARTED:
				x := pbevents.FromPbMust(*ev).(*pbevents.EvRepoMoveStarted)
				outev.RegistryEventId = x.RegistryEventId.String()
				outev.WorkflowId = x.WorkflowId.String()
				outev.OldGlobalPath = x.OldGlobalPath
				outev.OldFileHost = x.OldFileHost
				outev.OldHostPath = x.OldHostPath
				outev.OldShadowPath = x.OldShadowPath
				outev.NewGlobalPath = x.NewGlobalPath
				outev.NewFileHost = x.NewFileHost
				outev.NewHostPath = x.NewHostPath

			case pb.RepoEvent_EV_FSO_REPO_MOVED:
				x := pbevents.FromPbMust(*ev).(*pbevents.EvRepoMoved)
				outev.WorkflowId = x.WorkflowId.String()
				outev.WorkflowEventId = x.WorkflowEventId.String()
				outev.GlobalPath = x.GlobalPath
				outev.FileHost = x.FileHost
				outev.HostPath = x.HostPath
				outev.ShadowPath = x.ShadowPath

			case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
				outev.WorkflowId = mustParseWorkflowId(
					lg, ev.WorkflowId,
				).String()
				evi := ev.FsoShadowRepoInfo
				outev.ShadowRepoInfo = &ShadowRepoInfo{
					NewShadowPath: evi.NewShadowPath,
				}

			case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVED:
				outev.WorkflowId = mustParseWorkflowId(
					lg, ev.WorkflowId,
				).String()
				if ev.WorkflowEventId != nil {
					outev.WorkflowEventId = mustParseWorkflowEventId(
						lg, ev.WorkflowEventId,
					).String()
				}
				outev.ShadowRepoInfo = &ShadowRepoInfo{
					NewShadowPath: ev.FsoShadowRepoInfo.NewShadowPath,
				}

			case pb.RepoEvent_EV_FSO_TARTT_REPO_CREATED:
				evi := ev.FsoArchiveRepoInfo
				outev.ArchiveRepoInfo = &ArchiveRepoInfo{
					ArchiveURL: evi.ArchiveUrl,
				}

			case pb.RepoEvent_EV_FSO_ARCHIVE_RECIPIENTS_UPDATED:
				outev.GpgKeyFingerprints = asHexStrings(
					ev.FsoGpgKeyFingerprints,
				)

			case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_CREATED:
				fallthrough
			case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_MOVED:
				evi := ev.FsoShadowBackupRepoInfo
				outev.ShadowBackupRepoInfo = &ShadowBackupRepoInfo{
					ShadowBackupURL: evi.ShadowBackupUrl,
				}

			case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED:
				outev.GpgKeyFingerprints = asHexStrings(
					ev.FsoGpgKeyFingerprints,
				)

			case pb.RepoEvent_EV_FSO_GIT_REPO_CREATED:
				evi := ev.FsoGitRepoInfo
				outev.GitRepoInfo = &GitRepoInfo{
					GitlabProjectId: evi.GitlabProjectId,
				}

			case pb.RepoEvent_EV_FSO_GIT_TO_NOG_CLONED:
				outev.Note = "unimplemented event type"

			case pb.RepoEvent_EV_FSO_REPO_ERROR_SET:
				outev.ErrorMessage = ev.FsoRepoErrorMessage

			case pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED:

			// The legacy event
			// `RepoEvent_EV_FSO_FREEZE_REPO_STARTED` has been
			// replaced by
			// `RepoEvent_EV_FSO_FREEZE_REPO_STARTED_2`.
			case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED:
				outev.GitAuthor = &GitUser{
					Name:  ev.GitAuthor.Name,
					Email: ev.GitAuthor.Email,
				}

			// The legacy event
			// `RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED` has been
			// replaced by
			// `RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED_2`.
			case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED:
				outev.StatusCode = ev.StatusCode
				outev.StatusMessage = ev.StatusMessage

			// The legacy event
			// `RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED` has ben
			// replaced by
			// `RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED_2`.
			case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED:
				outev.GitAuthor = &GitUser{
					Name:  ev.GitAuthor.Name,
					Email: ev.GitAuthor.Email,
				}

			// The legacy event
			// `RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED` has been
			// replaced by
			// `RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2`.
			case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED:
				outev.StatusCode = ev.StatusCode
				outev.StatusMessage = ev.StatusMessage

			case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED_2:
				wfId := mustParseWorkflowId(lg, ev.WorkflowId)
				outev.WorkflowId = wfId.String()

			case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
				wfId := mustParseWorkflowId(lg, ev.WorkflowId)
				outev.WorkflowId = wfId.String()
				outev.StatusCode = ev.StatusCode

			case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
				wfId := mustParseWorkflowId(lg, ev.WorkflowId)
				outev.WorkflowId = wfId.String()

			case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
				wfId := mustParseWorkflowId(lg, ev.WorkflowId)
				outev.WorkflowId = wfId.String()
				outev.StatusCode = ev.StatusCode

			case pb.RepoEvent_EV_FSO_ARCHIVE_REPO_STARTED:
				wfId := mustParseWorkflowId(lg, ev.WorkflowId)
				outev.WorkflowId = wfId.String()

			case pb.RepoEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
				wfId := mustParseWorkflowId(lg, ev.WorkflowId)
				outev.WorkflowId = wfId.String()
				outev.StatusCode = ev.StatusCode

			case pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
				wfId := mustParseWorkflowId(lg, ev.WorkflowId)
				outev.WorkflowId = wfId.String()

			case pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
				wfId := mustParseWorkflowId(lg, ev.WorkflowId)
				outev.WorkflowId = wfId.String()
				outev.StatusCode = ev.StatusCode

			default:
				outev.Note = "nogfsoctl: unknown event type"
			}

			if err := jsonOut.Encode(&outev); err != nil {
				lg.Fatalw("JSON marshal failed.", "err", err)
			}
		}
		if rsp.WillBlock {
			fmt.Fprintln(os.Stderr, "# Waiting for more events.")
		}
	}
}

func mustParseWorkflowId(lg Logger, b []byte) uuid.I {
	id, err := uuid.FromBytes(b)
	if err != nil {
		lg.Fatalw("Failed to parse workflow ID.", "err", err)
	}
	return id
}

func mustParseWorkflowEventId(lg Logger, b []byte) ulid.I {
	id, err := ulid.ParseBytes(b)
	if err != nil {
		lg.Fatalw("Failed to parse workflow event ID.", "err", err)
	}
	return id
}

func asHexStrings(bs [][]byte) []string {
	if len(bs) == 0 {
		return nil
	}
	ss := make([]string, 0, len(bs))
	for _, b := range bs {
		ss = append(ss, fmt.Sprintf("%X", b))
	}
	return ss
}
