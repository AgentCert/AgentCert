package authorization

import (
	"context"
	"errors"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/grpc"
	"github.com/sirupsen/logrus"
)

// ValidateRole Validates the role of a user in a given project
func ValidateRole(ctx context.Context, projectID string,
	requiredRoles []string, invitation string) error {
	jwt := ctx.Value(AuthKey).(string)
	client := grpc.GetAuthGRPCSvcClient()
	err := grpc.ValidatorGRPCRequest(ctx, client, jwt, projectID,
		requiredRoles,
		invitation)
	if err != nil {
		logrus.Error(err)
		return errors.New("permission_denied: " + err.Error())
	}
	return nil
}
