package statdsd

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) IsInitRepoAllowed(
	ctx context.Context,
	repo, host, hostPath string,
	subdirTracking pb.SubdirTracking,
) (reason string, err error) {
	if ctx == nil {
		ctx = srv.ctx
	}

	se, err := srv.authPathSession(ctx, AAFsoInitRepo, repo)
	if err != nil {
		return "", err
	}

	c := pb.NewStatdsCallbackClient(se.conn)
	o, err := c.IsInitRepoAllowed(ctx, &pb.IsInitRepoAllowedI{
		Repo:           repo,
		FileHost:       host,
		HostPath:       hostPath,
		SubdirTracking: subdirTracking,
	})
	if err != nil {
		return "", err
	}

	if o.IsAllowed {
		return "", nil
	} else {
		// `reason != ""` indicates deny.
		return o.Reason, nil
	}
}
