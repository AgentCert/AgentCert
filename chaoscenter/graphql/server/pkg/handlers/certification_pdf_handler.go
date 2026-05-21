package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
)

// CertificatePDFHandler proxies the certifier's PDF endpoint
//   GET <CertificatePDFBaseURL>/api/certification/pdf?agent_id=...&experiment_id=...
//
// The frontend hits this through the webpack dev proxy (`/api/certification/pdf`
// → graphql `/certification/pdf`). The base URL of the upstream PDF service is
// configurable via the CERTIFICATE_PDF_BASE_URL env var; the path on the
// upstream service is always /api/certification/pdf.
//
// We stream the response body straight back to the caller and forward the
// Content-Type / Content-Disposition headers so the browser triggers a
// download instead of a navigation.
func CertificatePDFHandler() gin.HandlerFunc {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	return func(c *gin.Context) {
		agentID := c.Query("agent_id")
		experimentID := c.Query("experiment_id")
		if agentID == "" || experimentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "agent_id and experiment_id query params are required",
			})
			return
		}

		base := strings.TrimRight(utils.Config.CertificatePDFBaseURL, "/")
		if base == "" {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "CERTIFICATE_PDF_BASE_URL is not configured",
			})
			return
		}

		upstream := fmt.Sprintf("%s/api/v1/certification/pdf?agent_id=%s&experiment_id=%s",
			base, url.QueryEscape(agentID), url.QueryEscape(experimentID))

		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream, nil)
		if err != nil {
			logrus.WithError(err).Error("[CertPDF] build request failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			logrus.WithError(err).WithField("upstream", upstream).Error("[CertPDF] upstream request failed")
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			logrus.WithFields(logrus.Fields{
				"upstream": upstream,
				"status":   resp.StatusCode,
			}).Warn("[CertPDF] upstream returned error")
			c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
			return
		}

		// Forward headers needed for a clean download.
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			c.Header("Content-Type", ct)
		} else {
			c.Header("Content-Type", "application/pdf")
		}
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			c.Header("Content-Disposition", cd)
		} else {
			c.Header("Content-Disposition",
				fmt.Sprintf("attachment; filename=\"certification-%s.pdf\"", experimentID))
		}
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			c.Header("Content-Length", cl)
		}

		c.Status(resp.StatusCode)
		if _, err := io.Copy(c.Writer, resp.Body); err != nil {
			logrus.WithError(err).Warn("[CertPDF] streaming to client failed")
		}
	}
}
