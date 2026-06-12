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

// CertificateHTMLHandler proxies the certifier's HTML report endpoint
//   GET <CertificatePDFBaseURL>/api/v1/certification/html?agent_id=...&experiment_id=...
//
// Reuses CERTIFICATE_PDF_BASE_URL — the HTML report lives on the same service.
// Returns the file as an attachment so the browser downloads it rather than
// navigating, which matches the behaviour of CertificatePDFHandler.
func CertificateHTMLHandler() gin.HandlerFunc {
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

		upstream := fmt.Sprintf("%s/api/v1/certification/html?agent_id=%s&experiment_id=%s",
			base, url.QueryEscape(agentID), url.QueryEscape(experimentID))

		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream, nil)
		if err != nil {
			logrus.WithError(err).Error("[CertHTML] build request failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			logrus.WithError(err).WithField("upstream", upstream).Error("[CertHTML] upstream request failed")
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			logrus.WithFields(logrus.Fields{
				"upstream": upstream,
				"status":   resp.StatusCode,
			}).Warn("[CertHTML] upstream returned error")
			c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
			return
		}

		if ct := resp.Header.Get("Content-Type"); ct != "" {
			c.Header("Content-Type", ct)
		} else {
			c.Header("Content-Type", "text/html; charset=utf-8")
		}
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			c.Header("Content-Disposition", cd)
		} else {
			c.Header("Content-Disposition",
				fmt.Sprintf("attachment; filename=\"certification-%s.html\"", experimentID))
		}
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			c.Header("Content-Length", cl)
		}

		c.Status(resp.StatusCode)
		if _, err := io.Copy(c.Writer, resp.Body); err != nil {
			logrus.WithError(err).Warn("[CertHTML] streaming to client failed")
		}
	}
}
