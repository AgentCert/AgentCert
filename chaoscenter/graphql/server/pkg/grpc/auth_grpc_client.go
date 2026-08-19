package grpc

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/protos"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// authRequestTimeout bounds every individual RPC to the Authentication
// service so a slow/unreachable auth pod fails that one call quickly instead
// of hanging the calling goroutine forever. See
// OPEN_WEIGHT_CERTIFICATION_HANDOFF.md §47 ("Other findings not acted on
// this session") for the incident that flagged this gap.
const authRequestTimeout = 5 * time.Second

var (
	authConn     *grpc.ClientConn
	authConnOnce sync.Once
	authConnErr  error
)

// InitAuthGRPCConn establishes the single, long-lived connection to the
// Authentication service that every request-scoped call below reuses. Call
// once at process startup (mirrors the mongodb.MgoClient init-once-in-main
// pattern already used elsewhere in this server) before serving traffic.
// grpc.NewClient itself doesn't dial eagerly -- connection establishment and
// reconnection happen lazily and automatically per gRPC's own connectivity
// state machine, so this succeeding does not by itself prove the auth
// service is reachable; per-call timeouts below are what actually bound a
// stuck/unreachable auth service for callers.
//
// Idempotent: safe to call more than once, only the first call dials.
func InitAuthGRPCConn() error {
	authConnOnce.Do(func() {
		enableHTTPSConnection, err := strconv.ParseBool(utils.Config.EnableInternalTls)
		if err != nil {
			logrus.Errorf("unable to parse boolean value %v", err)
		}

		target := utils.Config.LitmusAuthGrpcEndpoint + ":" + utils.Config.LitmusAuthGrpcPort

		if enableHTTPSConnection {
			if utils.Config.TlsCertPath == "" || utils.Config.TlsKeyPath == "" {
				authConnErr = errors.New("failed to init auth GRPC client: empty TLS cert file path and TLS key path")
				return
			}
			conf := utils.GetTlsConfig(utils.Config.TlsCertPath, utils.Config.TlsKeyPath, false)
			authConn, authConnErr = grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(conf)))
		} else {
			authConn, authConnErr = grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
	})
	return authConnErr
}

// GetAuthGRPCSvcClient returns an RPC client bound to the single shared
// connection to the Authentication service. InitAuthGRPCConn must have
// already succeeded at startup -- callers never dial their own connection or
// need to Close() anything; the shared connection lives for the process
// lifetime and is safe for concurrent use by many goroutines at once (that's
// the intended usage of a *grpc.ClientConn).
func GetAuthGRPCSvcClient() protos.AuthRpcServiceClient {
	return protos.NewAuthRpcServiceClient(authConn)
}

// ValidatorGRPCRequest sends a request to Authentication server to ensure
// user permission over the project. ctx should be the caller's own
// request-scoped context (e.g. the GraphQL request context) -- it is bounded
// with authRequestTimeout here so a stuck/unreachable auth service fails
// this one call after a few seconds instead of hanging forever, while
// respecting any earlier cancellation/deadline from further up the stack.
func ValidatorGRPCRequest(ctx context.Context, client protos.AuthRpcServiceClient,
	jwt string, projectID string, requiredRoles []string, invitation string) error {
	ctx, cancel := context.WithTimeout(ctx, authRequestTimeout)
	defer cancel()

	resp, err := client.ValidateRequest(ctx,
		&protos.ValidationRequest{
			Jwt:           jwt,
			ProjectId:     projectID,
			RequiredRoles: requiredRoles,
			Invitation:    invitation,
		})
	if err != nil {
		return err
	}
	if resp.Error != "" || !resp.IsValid {
		return errors.New(resp.Error)
	}
	return nil
}

// GetProjectById returns the project details based on its uid. See
// ValidatorGRPCRequest for the ctx/timeout contract.
func GetProjectById(ctx context.Context, client protos.AuthRpcServiceClient,
	projectId string) (*protos.GetProjectByIdResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, authRequestTimeout)
	defer cancel()

	resp, err := client.GetProjectById(ctx, &protos.GetProjectByIdRequest{ProjectID: projectId})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUserById returns the project details based on its uid. See
// ValidatorGRPCRequest for the ctx/timeout contract.
func GetUserById(ctx context.Context, client protos.AuthRpcServiceClient,
	userId string) (*protos.GetUserByIdResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, authRequestTimeout)
	defer cancel()

	resp, err := client.GetUserById(ctx, &protos.GetUserByIdRequest{UserID: userId})
	if err != nil {
		return nil, err
	}
	return resp, nil
}
