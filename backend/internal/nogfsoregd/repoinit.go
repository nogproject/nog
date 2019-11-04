package nogfsoregd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsomain"
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/repoinit"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func ProcessRepoInit(
	ctx context.Context,
	lg Logger,
	n *shorteruuid.Names,
	mainJ *events.Journal,
	main *fsomain.Main,
	mainId uuid.I,
	regJ *events.Journal,
	reg *fsoregistry.Registry,
	repos *fsorepos.Repos,
) error {
	p := repoinit.NewProcessor(
		n, lg, mainJ, main, mainId, regJ, reg, repos,
	)
	return p.Process(ctx)
}
