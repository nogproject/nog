package main

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"google.golang.org/grpc/status"
)

func logFatalRPC(lg Logger, err error) {
	st, ok := status.FromError(err)
	if !ok {
		lg.Fatalw("RPC failed without GRPC status.", "err", err)
	}

	details := st.Details()
	if len(details) == 0 {
		lg.Fatalw(
			"RPC failed without GRPC status details.",
			"err", err,
		)
	}
	if len(details) > 1 {
		lg.Fatalw(
			"RPC failed with multiple GRPC status details.",
			"err", err,
			"details", details,
		)
	}

	switch sc := details[0].(type) {
	case *pb.AuthRequiredScope:
		lg.Fatalw(
			"RPC requires authorization.",
			"scope", sc,
		)
	default:
		lg.Fatalw(
			"RPC failed with unknown GRPC status detail.",
			"err", err,
			"details", details,
		)
	}
}
