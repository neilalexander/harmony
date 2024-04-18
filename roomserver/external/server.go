package external

import (
	"context"

	"github.com/nats-io/nats.go/micro"
	"github.com/neilalexander/harmony/roomserver/api"
	"github.com/neilalexander/harmony/setup/jetstream"
)

type RoomserverAPIServer struct {
	nats *jetstream.NATSInstance
	api.RoomserverInternalAPI
}

func NewRoomserverAPIServer(intapi api.RoomserverInternalAPI, nats *jetstream.NATSInstance) (*RoomserverAPIServer, error) {
	s := &RoomserverAPIServer{
		RoomserverInternalAPI: intapi,
		nats:                  nats,
	}
	ctx := context.TODO()

	config := micro.Config{
		Name:    "HarmonyRoomserver",
		Version: "0.0.1",
	}
	svc, err := micro.AddService(nats.Conn, config)
	if err != nil {
		return nil, err
	}
	for name, handler := range map[string]micro.HandlerFunc{
		"PerformBackfill": jetstream.MicroAPI(
			func(req *api.PerformBackfillRequest, res *api.PerformBackfillResponse) error {
				return s.RoomserverInternalAPI.PerformBackfill(ctx, req, res)
			},
		),
	} {
		if err := svc.AddEndpoint(name, handler); err != nil {
			return nil, err
		}
	}

	return s, nil
}
