package fsorepos_test

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/stretchr/testify/require"
)

var cmdInitRepo1 = fsorepos.CmdInitRepo{
	Registry:       "fooReg",
	GlobalPath:     "/foo/1",
	CreatorName:    "A. U. Thor",
	CreatorEmail:   "author@example.com",
	FileHost:       "files.example.com",
	HostPath:       "/data/1",
	SubdirTracking: fsorepos.EnterSubdirs,
}

var cmdConfirmShadow1 = fsorepos.CmdConfirmShadow{
	ShadowPath: "/fooshadow/1",
}

func TestCmdInitRepo(t *testing.T) {
	st := &fsorepos.State{}

	cmd := cmdInitRepo1
	evs, err := tell(st, &cmd)
	require.NoError(t, err)
	require.Len(t, evs, 1)
	ev, pbev := remarshal(t, evs[0])

	require.Equal(t, pb.RepoEvent_EV_FSO_REPO_INIT_STARTED, pbev.Event)
	inf := pbev.FsoRepoInitInfo
	require.Equal(t, cmd.Registry, inf.Registry)
	require.Equal(t, cmd.GlobalPath, inf.GlobalPath)
	require.Equal(t, cmd.CreatorName, inf.CreatorName)
	require.Equal(t, cmd.CreatorEmail, inf.CreatorEmail)
	require.Equal(t, cmd.FileHost, inf.FileHost)
	require.Equal(t, cmd.HostPath, inf.HostPath)
	require.Equal(t, pb.SubdirTracking_ST_ENTER_SUBDIRS, inf.SubdirTracking)

	ad := fsorepos.Advancer{}
	st = ad.Advance(st, ev).(*fsorepos.State)
	require.Equal(t, cmd.Registry, st.Registry())
	require.Equal(t, cmd.GlobalPath, st.GlobalPath())
	require.Equal(t,
		fmt.Sprintf("%s:%s", cmd.FileHost, cmd.HostPath),
		st.FileLocation(),
	)

	evs, err = tell(st, &cmd)
	require.NoError(t, err)
	require.Len(t, evs, 0)

	cmd2 := cmd
	cmd2.GlobalPath += "-different-path"
	_, err = tell(st, &cmd2)
	require.Equal(t, fsorepos.ErrInitConflict, err)
}

func TestCmdConfirmShadow(t *testing.T) {
	st := &fsorepos.State{}
	st = apply(t, st, &cmdInitRepo1)

	cmd := cmdConfirmShadow1
	evs, err := tell(st, &cmd)
	require.NoError(t, err)
	require.Len(t, evs, 1)
	ev, pbev := remarshal(t, evs[0])

	require.Equal(t, pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED, pbev.Event)
	require.Equal(t, cmd.ShadowPath, pbev.FsoShadowRepoInfo.ShadowPath)

	ad := fsorepos.Advancer{}
	st = ad.Advance(st, ev).(*fsorepos.State)
	require.Equal(t,
		fmt.Sprintf("%s:%s", cmdInitRepo1.FileHost, cmd.ShadowPath),
		st.ShadowLocation(),
	)

	evs, err = tell(st, &cmd)
	require.NoError(t, err)
	require.Len(t, evs, 0)

	cmd2 := cmd
	cmd2.ShadowPath += "-different-path"
	_, err = tell(st, &cmd2)
	require.Equal(t, fsorepos.ErrInitConflict, err)
}

func TestCmdInitTartt(t *testing.T) {
	var err error

	st := &fsorepos.State{}
	_, err = tell(st, &fsorepos.CmdInitTartt{
		TarttURL: "irrelevant",
	})
	require.Equal(t, fsorepos.ErrNotInitialized, err)

	st = apply(t, st, &cmdInitRepo1)

	for _, u := range []string{
		"invalid",
		"tartt://host/valid/path?driver=invalid",
		"tartt://host/valid/path?driver=local&tardir=/unexpected/param",
		"tartt://host/valid/path?driver=localtape&tardir=/invalid%20/param",
	} {
		_, err = tell(st, &fsorepos.CmdInitTartt{
			TarttURL: u,
		})
		require.Equal(t, fsorepos.ErrMalformedTarttURL, err)
	}

	for _, u := range []string{
		"tartt://files.example.com/archive/1?driver=local",
		"tartt://files.example.com/archive/1?driver=localtape&tardir=/tape/1",
	} {
		cmd := fsorepos.CmdInitTartt{
			TarttURL: u,
		}
		cmd2 := fsorepos.CmdInitTartt{
			TarttURL: "tartt://host/different/path?driver=local",
		}

		evs, err := tell(st, &cmd)
		require.NoError(t, err)
		require.Len(t, evs, 1)
		ev, pbev := remarshal(t, evs[0])

		require.Equal(t, pb.RepoEvent_EV_FSO_TARTT_REPO_CREATED, pbev.Event)
		require.Equal(t, cmd.TarttURL, pbev.FsoArchiveRepoInfo.ArchiveUrl)

		ad := fsorepos.Advancer{}
		st2 := ad.Advance(st, ev).(*fsorepos.State)
		require.Equal(t, u, st2.ArchiveURL())

		evs, err = tell(st2, &cmd)
		require.NoError(t, err)
		require.Len(t, evs, 0)

		_, err = tell(st2, &cmd2)
		require.Equal(t, fsorepos.ErrInitConflict, err)
	}
}

func TestCmdInitShadowBackup(t *testing.T) {
	var err error

	st := &fsorepos.State{}
	_, err = tell(st, &fsorepos.CmdInitShadowBackup{
		ShadowBackupURL: "irrelevant",
	})
	require.Equal(t, fsorepos.ErrNotInitialized, err)

	st = apply(t, st, &cmdInitRepo1)

	for _, u := range []string{
		"invalid",
		"nogfsobak://host/invalid/path with space",
	} {
		_, err = tell(st, &fsorepos.CmdInitShadowBackup{
			ShadowBackupURL: u,
		})
		require.Equal(t, fsorepos.ErrMalformedShadowBackupURL, err)
	}

	for _, u := range []string{
		"nogfsobak://files.example.com/backup",
		"nogfsobak://files.example.com/backup/2-2/foo.ext",
	} {
		cmd := fsorepos.CmdInitShadowBackup{
			ShadowBackupURL: u,
		}
		cmd2 := fsorepos.CmdInitShadowBackup{
			ShadowBackupURL: "nogfsobak://files.example.com/different-path",
		}

		evs, err := tell(st, &cmd)
		require.NoError(t, err)
		require.Len(t, evs, 1)
		ev, pbev := remarshal(t, evs[0])

		require.Equal(t,
			pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_CREATED,
			pbev.Event,
		)
		require.Equal(t,
			cmd.ShadowBackupURL,
			pbev.FsoShadowBackupRepoInfo.ShadowBackupUrl,
		)

		ad := fsorepos.Advancer{}
		st2 := ad.Advance(st, ev).(*fsorepos.State)
		require.Equal(t, u, st2.ShadowBackupURL())

		evs, err = tell(st2, &cmd)
		require.NoError(t, err)
		require.Len(t, evs, 0)

		_, err = tell(st2, &cmd2)
		require.Equal(t, fsorepos.ErrInitConflict, err)
	}
}

func remarshal(
	t testing.TB, ev events.Event,
) (*fsorepos.Event, *pb.RepoEvent) {
	t.Helper()

	dat, err := ev.MarshalProto()
	require.NoError(t, err)

	var pbev pb.RepoEvent
	require.NoError(t, proto.Unmarshal(dat, &pbev))

	var ev2 fsorepos.Event
	require.NoError(t, ev2.UnmarshalProto(dat))

	return &ev2, &pbev
}

func tell(
	st *fsorepos.State, cmd events.Command,
) ([]events.Event, error) {
	bh := fsorepos.Behavior{}
	return bh.Tell(st, cmd)
}

func apply(
	t testing.TB, st *fsorepos.State, cmd events.Command,
) *fsorepos.State {
	t.Helper()

	evs, err := tell(st, cmd)
	require.NoError(t, err)

	ad := fsorepos.Advancer{}
	for _, ev := range evs {
		st = ad.Advance(st, ev).(*fsorepos.State)
	}
	return st
}
