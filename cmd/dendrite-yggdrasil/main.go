// Copyright 2020 The Matrix.org Foundation C.I.C.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/neilalexander/harmony/internal/caching"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib/spec"
	"github.com/neilalexander/harmony/internal/sqlutil"
	"github.com/neilalexander/harmony/setup/jetstream"
	"github.com/neilalexander/harmony/setup/process"

	"github.com/gorilla/mux"
	"github.com/neilalexander/harmony/cmd/dendrite-yggdrasil/embed"
	"github.com/neilalexander/harmony/cmd/dendrite-yggdrasil/signing"
	"github.com/neilalexander/harmony/cmd/dendrite-yggdrasil/yggconn"
	"github.com/neilalexander/harmony/cmd/dendrite-yggdrasil/yggrooms"
	"github.com/neilalexander/harmony/federationapi"
	"github.com/neilalexander/harmony/federationapi/api"
	"github.com/neilalexander/harmony/internal"
	"github.com/neilalexander/harmony/internal/httputil"
	"github.com/neilalexander/harmony/roomserver"
	"github.com/neilalexander/harmony/setup"
	basepkg "github.com/neilalexander/harmony/setup/base"
	"github.com/neilalexander/harmony/setup/config"
	"github.com/neilalexander/harmony/setup/mscs"
	"github.com/neilalexander/harmony/userapi"
	"github.com/sirupsen/logrus"
)

var (
	instancePort   = flag.Int("port", 8008, "the port that the client API will listen on")
	instancePeer   = flag.String("peer", "", "the static Yggdrasil peers to connect to, comma separated-list")
	instanceListen = flag.String("listen", "tls://:0", "the port Yggdrasil peers can connect to")
)

// nolint: gocyclo
func main() {
	flag.Parse()
	internal.SetupPprof()

	cfg := setup.ParseFlags(true)
	configErrors := &config.ConfigErrors{}
	cfg.Verify(configErrors)
	if len(*configErrors) > 0 {
		for _, err := range *configErrors {
			logrus.Errorf("Configuration error: %s", err)
		}
		logrus.Fatalf("Failed to start due to configuration errors")
	}

	sk := cfg.Global.PrivateKey
	pk := sk.Public().(ed25519.PublicKey)

	cfg.Global.ServerName = spec.ServerName(hex.EncodeToString(pk))
	cfg.Global.KeyID = gomatrixserverlib.KeyID(signing.KeyID)

	internal.SetupStdLogging()
	internal.SetupHookLogging(cfg.Logging)
	internal.SetupPprof()

	logrus.Infof("Dendrite version %s", internal.VersionString())

	if !cfg.ClientAPI.RegistrationDisabled && cfg.ClientAPI.OpenRegistrationWithoutVerificationEnabled {
		logrus.Warn("Open registration is enabled")
	}

	processCtx := process.NewProcessContext()
	internal.SetupPyroscope(processCtx)

	cm := sqlutil.NewConnectionManager(processCtx, cfg.Global.DatabaseOptions)
	routers := httputil.NewRouters()

	basepkg.ConfigureAdminEndpoints(processCtx, routers)
	defer func() {
		processCtx.ShutdownDendrite()
		processCtx.WaitForShutdown()
	}() // nolint: errcheck

	ygg, err := yggconn.Setup(sk, *instancePeer, *instanceListen)
	if err != nil {
		panic(err)
	}

	federation := ygg.CreateFederationClient(cfg)

	serverKeyAPI := &signing.YggdrasilKeys{}
	keyRing := serverKeyAPI.KeyRing()

	caches := caching.NewRistrettoCache(cfg.Global.Cache.EstimatedMaxSize, cfg.Global.Cache.MaxAge, caching.EnableMetrics)
	natsInstance := jetstream.NATSInstance{}
	rsAPI := roomserver.NewInternalAPI(processCtx, cfg, cm, &natsInstance, caches, caching.EnableMetrics)

	fsAPI := federationapi.NewInternalAPI(
		processCtx, cfg, cm, &natsInstance, federation, rsAPI, caches, keyRing, true,
	)
	rsAPI.SetFederationAPI(fsAPI, keyRing)

	userAPI := userapi.NewInternalAPI(processCtx, cfg, cm, &natsInstance, rsAPI, federation, caching.EnableMetrics, fsAPI.IsBlacklistedOrBackingOff)

	client := ygg.CreateClient()

	monolith := setup.Monolith{
		Config:    cfg,
		Client:    client,
		FedClient: federation,
		KeyRing:   keyRing,

		FederationAPI: fsAPI,
		RoomserverAPI: rsAPI,
		UserAPI:       userAPI,
		ExtPublicRoomsProvider: yggrooms.NewYggdrasilRoomProvider(
			ygg, fsAPI, federation,
		),
	}
	monolith.AddAllPublicRoutes(processCtx, cfg, routers, cm, &natsInstance, caches, caching.EnableMetrics)
	if err := mscs.Enable(cfg, cm, routers, &monolith, caches); err != nil {
		logrus.WithError(err).Fatalf("Failed to enable MSCs")
	}

	mediaProxy := func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		serverName := params["serverName"]
		if serverName == string(cfg.Global.ServerName) {
			routers.Media.ServeHTTP(w, r)
			return
		}
		logrus.WithFields(logrus.Fields{
			"server_name": serverName,
			"media_id":    params["mediaID"],
		}).Infof("Proxying media request")
		r.URL.Scheme = "matrix"
		r.URL.Host = serverName
		res, err := client.DoHTTPRequest(r.Context(), &http.Request{
			Method: r.Method,
			URL:    r.URL,
		})
		if err != nil {
			w.WriteHeader(400)
			return
		}
		for header, values := range res.Header {
			if header == "Authorization" {
				continue
			}
			for _, value := range values {
				w.Header().Add(header, value)
			}
		}
		_, _ = io.Copy(w, res.Body)
	}

	httpRouter := mux.NewRouter().SkipClean(true).UseEncodedPath()
	httpRouter.PathPrefix("/_matrix/media/{proto}/download/{serverName}/{mediaID}").HandlerFunc(mediaProxy)
	httpRouter.PathPrefix("/_matrix/media/{proto}/thumbnail/{serverName}/{mediaID}").HandlerFunc(mediaProxy)
	httpRouter.PathPrefix(httputil.PublicClientPathPrefix).Handler(routers.Client)
	httpRouter.PathPrefix(httputil.PublicMediaPathPrefix).Handler(routers.Media)
	httpRouter.PathPrefix(httputil.DendriteAdminPathPrefix).Handler(routers.DendriteAdmin)
	httpRouter.PathPrefix(httputil.SynapseAdminPathPrefix).Handler(routers.SynapseAdmin)
	embed.Embed(httpRouter, *instancePort, "Yggdrasil")

	yggRouter := mux.NewRouter().SkipClean(true).UseEncodedPath()
	yggRouter.PathPrefix(httputil.PublicFederationPathPrefix).Handler(routers.Federation)
	yggRouter.PathPrefix(httputil.PublicMediaPathPrefix).Handler(routers.Media)

	// Build both ends of a HTTP multiplex.
	httpServer := &http.Server{
		Addr:         ":0",
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return context.Background()
		},
		Handler: yggRouter,
	}

	go func() {
		logrus.Info("Listening on ", ygg.DerivedServerName())
		logrus.Fatal(httpServer.Serve(ygg))
	}()
	go func() {
		httpBindAddr := fmt.Sprintf(":%d", *instancePort)
		logrus.Info("Listening on ", httpBindAddr)
		logrus.Fatal(http.ListenAndServe(httpBindAddr, httpRouter))
	}()
	go func() {
		logrus.Info("Sending wake-up message to known nodes")
		req := &api.PerformBroadcastEDURequest{}
		res := &api.PerformBroadcastEDUResponse{}
		if err := fsAPI.PerformBroadcastEDU(context.TODO(), req, res); err != nil {
			logrus.WithError(err).Error("Failed to send wake-up message to known nodes")
		}
	}()

	// We want to block forever to let the HTTP and HTTPS handler serve the APIs
	basepkg.WaitForShutdown(processCtx)
}
