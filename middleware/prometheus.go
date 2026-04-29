package middleware

import (
	"fmt"
	"strings"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

func PrometheusHandler(m *Monitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		snap := m.Snapshot()
		audit := config.GetAuditCounters()

		var b strings.Builder
		b.WriteString("# HELP apig0_requests_total Total observed requests.\n")
		b.WriteString("# TYPE apig0_requests_total counter\n")
		b.WriteString(fmt.Sprintf("apig0_requests_total %d\n", snap.Total))
		b.WriteString("# HELP apig0_errors_total Total observed request errors.\n")
		b.WriteString("# TYPE apig0_errors_total counter\n")
		b.WriteString(fmt.Sprintf("apig0_errors_total %d\n", snap.Errors))
		b.WriteString("# HELP apig0_api_tokens_total Configured API tokens.\n")
		b.WriteString("# TYPE apig0_api_tokens_total gauge\n")
		b.WriteString(fmt.Sprintf("apig0_api_tokens_total %d\n", config.APITokenCount()))
		b.WriteString("# HELP apig0_policy_users_total Users with explicit route policies.\n")
		b.WriteString("# TYPE apig0_policy_users_total gauge\n")
		b.WriteString(fmt.Sprintf("apig0_policy_users_total %d\n", config.AccessPolicyUserCount()))
		b.WriteString("# HELP apig0_audit_events_total Audit events by decision.\n")
		b.WriteString("# TYPE apig0_audit_events_total counter\n")
		b.WriteString(fmt.Sprintf("apig0_audit_events_total{decision=\"allow\"} %d\n", audit.Allow))
		b.WriteString(fmt.Sprintf("apig0_audit_events_total{decision=\"deny\"} %d\n", audit.Deny))
		b.WriteString(fmt.Sprintf("apig0_audit_events_total{decision=\"other\"} %d\n", audit.Other))
		b.WriteString("# HELP apig0_service_requests_total Requests per configured service.\n")
		b.WriteString("# TYPE apig0_service_requests_total counter\n")
		for _, svc := range snap.Services {
			b.WriteString(fmt.Sprintf("apig0_service_requests_total{service=%q} %d\n", svc.Name, svc.Requests))
			b.WriteString(fmt.Sprintf("apig0_service_errors_total{service=%q} %d\n", svc.Name, svc.Errors))
			b.WriteString(fmt.Sprintf("apig0_service_avg_latency_ms{service=%q} %.3f\n", svc.Name, svc.AvgLatency))
		}

		c.Data(200, "text/plain; version=0.0.4; charset=utf-8", []byte(b.String()))
	}
}
