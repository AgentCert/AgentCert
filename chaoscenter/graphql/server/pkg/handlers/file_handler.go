package handlers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"

	"github.com/sirupsen/logrus"
)

// FileHandler dynamically generates the manifest file and sends it as a response.
//
// This route is the direct-download path: it is meant to be fetched straight from
// the target host with `kubectl apply -f <url>` or `curl`, so the infra manifest
// never has to be downloaded in a browser and copied over separately. Because of
// that it must not depend on a Referer header — kubectl/curl never send one.
// GetK8sInfraYaml already prefers the configured CHAOS_CENTER_UI_ENDPOINT over any
// host hint, so the Referer/Host fallback below only matters for deployments that
// leave that setting unset.
func FileHandler(mongodbOperator mongodb.MongoOperator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSuffix(c.Param("key"), ".yaml")

		infraId, err := chaos_infrastructure.InfraValidateJWT(token)
		if err != nil {
			logrus.Error(err)
			utils.WriteHeaders(&c.Writer, 500)
			c.Writer.Write([]byte(err.Error()))
			return
		}

		infra, err := dbChaosInfra.NewInfrastructureOperator(mongodbOperator).GetInfra(infraId)
		if err != nil {
			logrus.Error(err)
			utils.WriteHeaders(&c.Writer, 500)
			c.Writer.Write([]byte(err.Error()))
			return
		}

		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		if forwardedProto := c.Request.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
			scheme = forwardedProto
		}
		host := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

		// A browser-driven request (e.g. the "re-download" button on an existing
		// infra row) still carries a Referer; prefer it when present since it
		// reflects the page the user is actually on.
		if referrer := c.Request.Header.Get("Referer"); referrer != "" {
			if referrerURL, err := url.Parse(referrer); err == nil && referrerURL.Host != "" {
				host = fmt.Sprintf("%s://%s", referrerURL.Scheme, referrerURL.Host)
			}
		}

		response, err := chaos_infrastructure.GetK8sInfraYaml(host, infra)
		if err != nil {
			logrus.Error(err)
			utils.WriteHeaders(&c.Writer, 500)
			c.Writer.Write([]byte(err.Error()))
			return
		}

		utils.WriteHeaders(&c.Writer, 200)
		c.Writer.Write(response)
	}
}
