package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	pbevents "github.com/nogproject/nog/backend/internal/fsoregistry/pbevents"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

type Event struct {
	Event  string `json:"event"`
	Id     string `json:"id"`
	Parent string `json:"parent"`
	Etime  string `json:"etime"`

	Note                 string `json:"note,omitempty"`
	Name                 string `json:"name,omitempty"`
	*RootInfo            `json:"rootInfo,omitempty"`
	RepoEventId          string `json:"repoEventId,omitempty"`
	*RepoInfo            `json:"repoInfo,omitempty"`
	ReinitReason         string `json:"reinitReason,omitempty"`
	RepoId               string `json:"repoId,omitempty"`
	GitlabNamespace      string `json:"gitlabNamespace,omitempty"`
	*pb.FsoRepoNaming    `json:"repoNaming,omitempty"`
	*RepoInitPolicy      `json:"repoInitPolicy,omitempty"`
	*SplitRootParams     `json:"splitRootParams,omitempty"`
	WorkflowId           string `json:"workflowId,omitempty"`
	EphemeralWorkflowsId string `json:"ephemeralWorkflowsId,omitempty"`
	*PathFlag            `json:"pathFlag,omitempty"`
	GpgKeyFingerprints   []string `json:"gpgKeyFingerprints,omitempty"`
	StatusCode           int32    `json:"statusCode,omitempty"`
	RepoAclPolicy        string   `json:"repoAclPolicy,omitempty"`
}

type RootInfo struct {
	GlobalRoot      string `json:"globalRoot"`
	Host            string `json:"host"`
	HostRoot        string `json:"hostRoot"`
	GitlabNamespace string `json:"gitlabNamespace"`
}

type RepoInfo struct {
	Id            string `json:"id"`
	GlobalPath    string `json:"globalPath,omitempty"`
	NewGlobalPath string `json:"newGlobalPath,omitempty"`
	CreatorName   string `json:"creatorName,omitempty"`
	CreatorEmail  string `json:"creatorEmail,omitempty"`
	Confirmed     bool   `json:"confirmed,omitempty"`
}

type RepoInitPolicy struct {
	GlobalRoot             string               `json:"globalRoot"`
	Policy                 string               `json:"policy"`
	SubdirTrackingGloblist []SubdirTrackingGlob `json:"subdirTrackingGloblist,omitempty"`
}

type SplitRootParams struct {
	GlobalRoot   string `json:"globalRoot"`
	MaxDepth     int32  `json:"maxDepth,omitempty"`
	MinDiskUsage int64  `json:"minDiskUsage,omitempty"`
	MaxDiskUsage int64  `json:"maxDiskUsage,omitempty"`
}

type SubdirTrackingGlob struct {
	SubdirTracking string `json:"subdirTracking"`
	Pattern        string `json:"pattern"`
}

type PathFlag struct {
	Path  string `json:"path"`
	Flags uint32 `json:"flags"`
}

func RepoInitPolicyFromPb(p pb.FsoRepoInitPolicy) RepoInitPolicy {
	ret := RepoInitPolicy{
		GlobalRoot: p.GlobalRoot,
		Policy:     p.Policy.String(),
	}

	var globList []SubdirTrackingGlob
	for _, g := range p.SubdirTrackingGloblist {
		globList = append(globList, SubdirTrackingGlob{
			SubdirTracking: g.SubdirTracking.String(),
			Pattern:        g.Pattern,
		})
	}
	ret.SubdirTrackingGloblist = globList

	return ret
}

func cmdEventsRegistry(args map[string]interface{}) {
	ctx := context.Background()

	conn, err := dialX509(
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

	c := pb.NewRegistryClient(conn)
	req := pb.RegistryEventsI{
		Registry: args["<registry>"].(string),
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
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoReadRegistry,
		Name:   req.Registry,
	})
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
			if ev.RepoEventId != nil {
				vid, err := ulid.ParseBytes(ev.RepoEventId)
				if err != nil {
					lg.Fatalw(
						"failed to decode ULID.",
						"err", err,
					)
				}
				outev.RepoEventId = vid.String()
			}
			switch ev.Event {
			case pb.RegistryEvent_EV_FSO_REGISTRY_ADDED:
				outev.Name = ev.FsoRegistryInfo.Name

			case pb.RegistryEvent_EV_EPHEMERAL_WORKFLOWS_ENABLED:
				x := pbevents.FromPbMust(*ev).(*pbevents.EvEphemeralWorkflowsEnabled)
				outev.EphemeralWorkflowsId = x.EphemeralWorkflowsId.String()

			case pb.RegistryEvent_EV_FSO_REPO_ACL_POLICY_UPDATED:
				x := pbevents.FromPbMust(*ev).(*pbevents.EvRepoAclPolicyUpdated)
				outev.RepoAclPolicy = x.Policy.String()

			case pb.RegistryEvent_EV_FSO_ROOT_ADDED:
				fallthrough
			case pb.RegistryEvent_EV_FSO_ROOT_REMOVED:
				fallthrough
			case pb.RegistryEvent_EV_FSO_ROOT_UPDATED:
				outev.RootInfo = &RootInfo{
					GlobalRoot:      ev.FsoRootInfo.GlobalRoot,
					Host:            ev.FsoRootInfo.Host,
					HostRoot:        ev.FsoRootInfo.HostRoot,
					GitlabNamespace: ev.FsoRootInfo.GitlabNamespace,
				}

			case pb.RegistryEvent_EV_FSO_REPO_NAMING_UPDATED:
				fallthrough
			case pb.RegistryEvent_EV_FSO_REPO_NAMING_CONFIG_UPDATED:
				outev.FsoRepoNaming = ev.FsoRepoNaming

			case pb.RegistryEvent_EV_FSO_REPO_INIT_POLICY_UPDATED:
				policy := RepoInitPolicyFromPb(
					*ev.FsoRepoInitPolicy,
				)
				outev.RepoInitPolicy = &policy

			case pb.RegistryEvent_EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED:
				outev.RootInfo = &RootInfo{
					GlobalRoot: ev.FsoRootInfo.GlobalRoot,
				}
				outev.GpgKeyFingerprints = asHexStrings(
					ev.FsoGpgKeyFingerprints,
				)
			case pb.RegistryEvent_EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED:
				outev.RootInfo = &RootInfo{
					GlobalRoot: ev.FsoRootInfo.GlobalRoot,
				}
				outev.GpgKeyFingerprints = asHexStrings(
					ev.FsoGpgKeyFingerprints,
				)

			case pb.RegistryEvent_EV_FSO_REPO_ACCEPTED:
				fallthrough
			case pb.RegistryEvent_EV_FSO_REPO_ADDED:
				inf := ev.FsoRepoInfo
				uu, err := uuid.FromBytes(inf.Id)
				if err != nil {
					lg.Fatalw(
						"failed to decode UUID.",
						"err", err,
					)
				}
				outev.RepoInfo = &RepoInfo{
					Id:           uu.String(),
					GlobalPath:   inf.GlobalPath,
					CreatorName:  inf.CreatorName,
					CreatorEmail: inf.CreatorEmail,
					Confirmed:    inf.Confirmed,
				}

			case pb.RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED:
				x := pbevents.FromPbMust(*ev).(*pbevents.EvRepoMoveAccepted)
				outev.WorkflowId = x.WorkflowId.String()
				outev.RepoInfo = &RepoInfo{
					Id:            x.RepoId.String(),
					NewGlobalPath: x.NewGlobalPath,
				}

			case pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED:
				repoId, err := uuid.FromBytes(ev.RepoId)
				if err != nil {
					lg.Fatalw(
						"failed to decode UUID.",
						"err", err,
					)
				}
				outev.RepoId = repoId.String()
				outev.GitlabNamespace = ev.GitlabNamespace

			case pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED:
				inf := ev.FsoRepoInfo
				uu, err := uuid.FromBytes(inf.Id)
				if err != nil {
					lg.Fatalw(
						"failed to decode UUID.",
						"err", err,
					)
				}
				outev.RepoInfo = &RepoInfo{
					Id:         uu.String(),
					GlobalPath: inf.GlobalPath,
				}
				outev.ReinitReason = ev.FsoRepoReinitReason

			case pb.RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
				id := mustParseRepoId(ev.RepoId)
				evId := mustParseRepoEventId(ev.RepoEventId)
				wfId := mustParseWorkflowId(ev.WorkflowId)
				outev.RepoId = id.String()
				outev.RepoEventId = evId.String()
				outev.WorkflowId = wfId.String()

			case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_ENABLED:
				outev.SplitRootParams = &SplitRootParams{
					GlobalRoot: ev.FsoSplitRootParams.GlobalRoot,
				}

			case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_PARAMS_UPDATED:
				outev.SplitRootParams = &SplitRootParams{
					GlobalRoot:   ev.FsoSplitRootParams.GlobalRoot,
					MaxDepth:     ev.FsoSplitRootParams.MaxDepth,
					MinDiskUsage: ev.FsoSplitRootParams.MinDiskUsage,
					MaxDiskUsage: ev.FsoSplitRootParams.MaxDiskUsage,
				}

			case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_DISABLED:
				outev.SplitRootParams = &SplitRootParams{
					GlobalRoot: ev.FsoSplitRootParams.GlobalRoot,
				}

			case pb.RegistryEvent_EV_FSO_PATH_FLAG_SET:
				fallthrough
			case pb.RegistryEvent_EV_FSO_PATH_FLAG_UNSET:
				outev.PathFlag = &PathFlag{
					Path:  ev.FsoPathFlag.Path,
					Flags: ev.FsoPathFlag.Flags,
				}

			case pb.RegistryEvent_EV_FSO_FREEZE_REPO_STARTED_2:
				repoId := mustParseRepoId(ev.RepoId)
				wfId := mustParseWorkflowId(ev.WorkflowId)
				outev.RepoId = repoId.String()
				outev.WorkflowId = wfId.String()

			case pb.RegistryEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
				repoId := mustParseRepoId(ev.RepoId)
				wfId := mustParseWorkflowId(ev.WorkflowId)
				outev.RepoId = repoId.String()
				outev.WorkflowId = wfId.String()
				outev.StatusCode = ev.StatusCode

			case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
				repoId := mustParseRepoId(ev.RepoId)
				wfId := mustParseWorkflowId(ev.WorkflowId)
				outev.RepoId = repoId.String()
				outev.WorkflowId = wfId.String()

			case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
				repoId := mustParseRepoId(ev.RepoId)
				wfId := mustParseWorkflowId(ev.WorkflowId)
				outev.RepoId = repoId.String()
				outev.WorkflowId = wfId.String()
				outev.StatusCode = ev.StatusCode

			case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_STARTED:
				repoId := mustParseRepoId(ev.RepoId)
				wfId := mustParseWorkflowId(ev.WorkflowId)
				outev.RepoId = repoId.String()
				outev.WorkflowId = wfId.String()

			case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
				repoId := mustParseRepoId(ev.RepoId)
				wfId := mustParseWorkflowId(ev.WorkflowId)
				outev.RepoId = repoId.String()
				outev.WorkflowId = wfId.String()
				outev.StatusCode = ev.StatusCode

			case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
				repoId := mustParseRepoId(ev.RepoId)
				wfId := mustParseWorkflowId(ev.WorkflowId)
				outev.RepoId = repoId.String()
				outev.WorkflowId = wfId.String()

			case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
				repoId := mustParseRepoId(ev.RepoId)
				wfId := mustParseWorkflowId(ev.WorkflowId)
				outev.RepoId = repoId.String()
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

func mustParseRepoId(b []byte) uuid.I {
	id, err := uuid.FromBytes(b)
	if err != nil {
		lg.Fatalw("Failed to decode repo UUID.", "err", err)
	}
	return id
}

func mustParseRepoEventId(b []byte) ulid.I {
	id, err := ulid.ParseBytes(b)
	if err != nil {
		lg.Fatalw("Failed to decode repo event ULID.", "err", err)
	}
	return id
}

func mustParseWorkflowId(b []byte) uuid.I {
	id, err := uuid.FromBytes(b)
	if err != nil {
		lg.Fatalw("Failed to parse workflow ID.", "err", err)
	}
	return id
}
