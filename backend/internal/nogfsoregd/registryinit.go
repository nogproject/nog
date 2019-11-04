package nogfsoregd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsomain"
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/registryinit"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func ProcessRegistryInit(
	ctx context.Context,
	lg Logger,
	n *shorteruuid.Names,
	mainJ *events.Journal,
	main *fsomain.Main,
	mainId uuid.I,
	reg *fsoregistry.Registry,
) error {
	p := registryinit.NewProcessor(n, lg, mainJ, main, mainId, reg)
	return p.Process(ctx)
}
