package internal

import (
	"context"
	"crypto/ed25519"

	"github.com/nats-io/nats.go"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib/fclient"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib/spec"
	"github.com/neilalexander/harmony/internal/util"
	"github.com/sirupsen/logrus"

	fsAPI "github.com/neilalexander/harmony/federationapi/api"
	"github.com/neilalexander/harmony/internal/caching"
	"github.com/neilalexander/harmony/roomserver/acls"
	"github.com/neilalexander/harmony/roomserver/api"
	"github.com/neilalexander/harmony/roomserver/internal/input"
	"github.com/neilalexander/harmony/roomserver/internal/perform"
	"github.com/neilalexander/harmony/roomserver/internal/query"
	"github.com/neilalexander/harmony/roomserver/producers"
	"github.com/neilalexander/harmony/roomserver/storage"
	"github.com/neilalexander/harmony/roomserver/types"
	"github.com/neilalexander/harmony/setup/config"
	"github.com/neilalexander/harmony/setup/jetstream"
	"github.com/neilalexander/harmony/setup/process"
	userapi "github.com/neilalexander/harmony/userapi/api"
)

// RoomserverInternalAPI is an implementation of api.RoomserverInternalAPI
type RoomserverInternalAPI struct {
	*input.Inputer
	*query.Queryer
	*perform.Inviter
	*perform.Joiner
	*perform.Leaver
	*perform.Publisher
	*perform.Backfiller
	*perform.Forgetter
	*perform.Upgrader
	*perform.Admin
	*perform.Creator
	ProcessContext         *process.ProcessContext
	DB                     storage.Database
	Cfg                    *config.Dendrite
	Cache                  caching.RoomServerCaches
	ServerName             spec.ServerName
	KeyRing                gomatrixserverlib.JSONVerifier
	ServerACLs             *acls.ServerACLs
	fsAPI                  fsAPI.RoomserverFederationAPI
	NATSClient             *nats.Conn
	JetStream              nats.JetStreamContext
	Durable                string
	InputRoomEventTopic    string // JetStream topic for new input room events
	OutputProducer         *producers.RoomEventProducer
	PerspectiveServerNames []spec.ServerName
	enableMetrics          bool
	defaultRoomVersion     gomatrixserverlib.RoomVersion
}

func NewRoomserverAPI(
	processContext *process.ProcessContext, dendriteCfg *config.Dendrite, roomserverDB storage.Database,
	js nats.JetStreamContext, nc *nats.Conn, caches caching.RoomServerCaches, enableMetrics bool,
) *RoomserverInternalAPI {
	var perspectiveServerNames []spec.ServerName
	for _, kp := range dendriteCfg.FederationAPI.KeyPerspectives {
		perspectiveServerNames = append(perspectiveServerNames, kp.ServerName)
	}

	serverACLs := acls.NewServerACLs(roomserverDB)
	producer := &producers.RoomEventProducer{
		Topic:     string(dendriteCfg.Global.JetStream.Prefixed(jetstream.OutputRoomEvent)),
		JetStream: js,
		ACLs:      serverACLs,
	}
	a := &RoomserverInternalAPI{
		ProcessContext:         processContext,
		DB:                     roomserverDB,
		Cfg:                    dendriteCfg,
		Cache:                  caches,
		ServerName:             dendriteCfg.Global.ServerName,
		PerspectiveServerNames: perspectiveServerNames,
		InputRoomEventTopic:    dendriteCfg.Global.JetStream.Prefixed(jetstream.InputRoomEvent),
		OutputProducer:         producer,
		JetStream:              js,
		NATSClient:             nc,
		Durable:                dendriteCfg.Global.JetStream.Durable("RoomserverInputConsumer"),
		ServerACLs:             serverACLs,
		enableMetrics:          enableMetrics,
		defaultRoomVersion:     dendriteCfg.RoomServer.DefaultRoomVersion,
		// perform-er structs + queryer struct get initialised when we have a federation sender to use
	}
	return a
}

// SetFederationInputAPI passes in a federation input API reference so that we can
// avoid the chicken-and-egg problem of both the roomserver input API and the
// federation input API being interdependent.
func (r *RoomserverInternalAPI) SetFederationAPI(fsAPI fsAPI.RoomserverFederationAPI, keyRing *gomatrixserverlib.KeyRing) {
	r.fsAPI = fsAPI
	r.KeyRing = keyRing

	r.Queryer = &query.Queryer{
		DB:                r.DB,
		Cache:             r.Cache,
		IsLocalServerName: r.Cfg.Global.IsLocalServerName,
		ServerACLs:        r.ServerACLs,
		Cfg:               r.Cfg,
		FSAPI:             fsAPI,
	}

	r.Inputer = &input.Inputer{
		Cfg:                 &r.Cfg.RoomServer,
		ProcessContext:      r.ProcessContext,
		DB:                  r.DB,
		InputRoomEventTopic: r.InputRoomEventTopic,
		OutputProducer:      r.OutputProducer,
		JetStream:           r.JetStream,
		NATSClient:          r.NATSClient,
		Durable:             nats.Durable(r.Durable),
		ServerName:          r.ServerName,
		SigningIdentity:     r.SigningIdentityFor,
		FSAPI:               fsAPI,
		RSAPI:               r,
		KeyRing:             keyRing,
		ACLs:                r.ServerACLs,
		Queryer:             r.Queryer,
		EnableMetrics:       r.enableMetrics,
	}
	r.Inviter = &perform.Inviter{
		DB:      r.DB,
		Cfg:     &r.Cfg.RoomServer,
		FSAPI:   r.fsAPI,
		RSAPI:   r,
		Inputer: r.Inputer,
	}
	r.Joiner = &perform.Joiner{
		Cfg:     &r.Cfg.RoomServer,
		DB:      r.DB,
		FSAPI:   r.fsAPI,
		RSAPI:   r,
		Inputer: r.Inputer,
		Queryer: r.Queryer,
	}
	r.Leaver = &perform.Leaver{
		Cfg:     &r.Cfg.RoomServer,
		DB:      r.DB,
		FSAPI:   r.fsAPI,
		RSAPI:   r,
		Inputer: r.Inputer,
	}
	r.Publisher = &perform.Publisher{
		DB: r.DB,
	}
	r.Backfiller = &perform.Backfiller{
		IsLocalServerName: r.Cfg.Global.IsLocalServerName,
		DB:                r.DB,
		FSAPI:             r.fsAPI,
		Querier:           r.Queryer,
		KeyRing:           r.KeyRing,
		// Perspective servers are trusted to not lie about server keys, so we will also
		// prefer these servers when backfilling (assuming they are in the room) rather
		// than trying random servers
		PreferServers: r.PerspectiveServerNames,
	}
	r.Forgetter = &perform.Forgetter{
		DB: r.DB,
	}
	r.Upgrader = &perform.Upgrader{
		Cfg:    &r.Cfg.RoomServer,
		URSAPI: r,
	}
	r.Admin = &perform.Admin{
		DB:      r.DB,
		Cfg:     &r.Cfg.RoomServer,
		Inputer: r.Inputer,
		Queryer: r.Queryer,
		Leaver:  r.Leaver,
	}
	r.Creator = &perform.Creator{
		DB:    r.DB,
		Cfg:   &r.Cfg.RoomServer,
		RSAPI: r,
	}

	if err := r.Inputer.Start(); err != nil {
		logrus.WithError(err).Panic("failed to start roomserver input API")
	}
}

func (r *RoomserverInternalAPI) SetUserAPI(userAPI userapi.RoomserverUserAPI) {
	r.Leaver.UserAPI = userAPI
	r.Inputer.UserAPI = userAPI
}

func (r *RoomserverInternalAPI) DefaultRoomVersion() gomatrixserverlib.RoomVersion {
	return r.defaultRoomVersion
}

func (r *RoomserverInternalAPI) IsKnownRoom(ctx context.Context, roomID spec.RoomID) (bool, error) {
	return r.Inviter.IsKnownRoom(ctx, roomID)
}

func (r *RoomserverInternalAPI) StateQuerier() gomatrixserverlib.StateQuerier {
	return r.Inviter.StateQuerier()
}

func (r *RoomserverInternalAPI) HandleInvite(
	ctx context.Context, inviteEvent *types.HeaderedEvent,
) error {
	outputEvents, err := r.Inviter.ProcessInviteMembership(ctx, inviteEvent)
	if err != nil {
		return err
	}
	return r.OutputProducer.ProduceRoomEvents(inviteEvent.RoomID().String(), outputEvents)
}

func (r *RoomserverInternalAPI) PerformCreateRoom(
	ctx context.Context, userID spec.UserID, roomID spec.RoomID, createRequest *api.PerformCreateRoomRequest,
) (string, *util.JSONResponse) {
	return r.Creator.PerformCreateRoom(ctx, userID, roomID, createRequest)
}

func (r *RoomserverInternalAPI) PerformInvite(
	ctx context.Context,
	req *api.PerformInviteRequest,
) error {
	return r.Inviter.PerformInvite(ctx, req)
}

func (r *RoomserverInternalAPI) PerformLeave(
	ctx context.Context,
	req *api.PerformLeaveRequest,
	res *api.PerformLeaveResponse,
) error {
	outputEvents, err := r.Leaver.PerformLeave(ctx, req, res)
	if err != nil {
		return err
	}
	if len(outputEvents) == 0 {
		return nil
	}
	return r.OutputProducer.ProduceRoomEvents(req.RoomID, outputEvents)
}

func (r *RoomserverInternalAPI) PerformForget(
	ctx context.Context,
	req *api.PerformForgetRequest,
	resp *api.PerformForgetResponse,
) error {
	return r.Forgetter.PerformForget(ctx, req, resp)
}

// GetOrCreateUserRoomPrivateKey gets the user room key for the specified user. If no key exists yet, a new one is created.
func (r *RoomserverInternalAPI) GetOrCreateUserRoomPrivateKey(ctx context.Context, userID spec.UserID, roomID spec.RoomID) (ed25519.PrivateKey, error) {
	key, err := r.DB.SelectUserRoomPrivateKey(ctx, userID, roomID)
	if err != nil {
		return nil, err
	}
	// no key found, create one
	if len(key) == 0 {
		_, key, err = ed25519.GenerateKey(nil)
		if err != nil {
			return nil, err
		}
		key, err = r.DB.InsertUserRoomPrivatePublicKey(ctx, userID, roomID, key)
		if err != nil {
			return nil, err
		}
	}
	return key, nil
}

func (r *RoomserverInternalAPI) StoreUserRoomPublicKey(ctx context.Context, senderID spec.SenderID, userID spec.UserID, roomID spec.RoomID) error {
	pubKeyBytes, err := senderID.RawBytes()
	if err != nil {
		return err
	}
	_, err = r.DB.InsertUserRoomPublicKey(ctx, userID, roomID, ed25519.PublicKey(pubKeyBytes))
	return err
}

func (r *RoomserverInternalAPI) SigningIdentityFor(ctx context.Context, roomID spec.RoomID, senderID spec.UserID) (fclient.SigningIdentity, error) {
	roomVersion, ok := r.Cache.GetRoomVersion(roomID.String())
	if !ok {
		roomInfo, err := r.DB.RoomInfo(ctx, roomID.String())
		if err != nil {
			return fclient.SigningIdentity{}, err
		}
		if roomInfo != nil {
			roomVersion = roomInfo.RoomVersion
		}
	}
	if roomVersion == gomatrixserverlib.RoomVersionPseudoIDs {
		privKey, err := r.GetOrCreateUserRoomPrivateKey(ctx, senderID, roomID)
		if err != nil {
			return fclient.SigningIdentity{}, err
		}
		return fclient.SigningIdentity{
			PrivateKey: privKey,
			KeyID:      "ed25519:1",
			ServerName: spec.ServerName(spec.SenderIDFromPseudoIDKey(privKey)),
		}, nil
	}
	identity, err := r.Cfg.Global.SigningIdentityFor(senderID.Domain())
	if err != nil {
		return fclient.SigningIdentity{}, err
	}
	return *identity, err
}

func (r *RoomserverInternalAPI) AssignRoomNID(ctx context.Context, roomID spec.RoomID, roomVersion gomatrixserverlib.RoomVersion) (roomNID types.RoomNID, err error) {
	return r.DB.AssignRoomNID(ctx, roomID, roomVersion)
}
