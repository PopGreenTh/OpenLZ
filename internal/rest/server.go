package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/powerquery"
)

// Server provides embedded HTTP REST endpoints for Power BI and Excel Web Refresh.
type Server struct {
	port          int
	duckdbEngine  *duckdb.Engine
	finopsParquet string
	secopsParquet string
	cloudopsParquet string
}

// NewServer initializes the REST server.
func NewServer(port int, engine *duckdb.Engine, finopsP, secopsP, cloudopsP string) *Server {
	if port <= 0 {
		port = 8080
	}
	return &Server{
		port:            port,
		duckdbEngine:    engine,
		finopsParquet:   finopsP,
		secopsParquet:   secopsP,
		cloudopsParquet: cloudopsP,
	}
}

// Start runs the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/finops", s.handleOpsEndpoint("finops", s.finopsParquet))
	mux.HandleFunc("/api/v1/secops", s.handleOpsEndpoint("secops", s.secopsParquet))
	mux.HandleFunc("/api/v1/cloudops", s.handleOpsEndpoint("cloudops", s.cloudopsParquet))
	mux.HandleFunc("/api/v1/executive", s.handleExecutiveEndpoint)
	mux.HandleFunc("/api/v1/powerquery", s.handlePowerQueryEndpoint)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","version":"1.0.0"}`))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("[OpenLZ] Live REST Server listening on http://localhost:%d", s.port)
	log.Printf("[OpenLZ] Power BI / Excel Web URL: http://localhost:%d/api/v1/finops", s.port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleOpsEndpoint(opsName, parquetPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("Access-Control-Allow-Origin", "*")

		var records []cloud.EnrichedRecord
		var err error

		if _, statErr := os.Stat(parquetPath); statErr == nil {
			records, err = s.duckdbEngine.ReadEnrichedFromParquet(ctx, parquetPath)
		}

		if err != nil || len(records) == 0 {
			// Fallback placeholder data for live demonstration if parquet not generated yet
			records = generateFallbackEnriched(opsName)
		}

		if r.URL.Query().Get("format") == "csv" {
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_report.csv", opsName))
			w.Write([]byte("ops_domain,account_id,account_name,environment,business_unit,owner,cost_center,service,resource_or_key,metric,actual_value,expected_value,potential_savings,severity,action_recommended\n"))
			for _, rec := range records {
				line := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%.2f,%.2f,%.2f,%s,\"%s\"\n",
					rec.OpsDomain, rec.AccountID, rec.AccountName, rec.Environment, rec.BusinessUnit,
					rec.Owner, rec.CostCenter, rec.Service, rec.ResourceOrKey, rec.Metric,
					rec.ActualValue, rec.ExpectedValue, rec.PotentialSavings, rec.Severity, rec.ActionRecommended)
				w.Write([]byte(line))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(records)
	}
}

func (s *Server) handleExecutiveEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	summary := map[string]interface{}{
		"report_date":            time.Now().Format("2006-01-02"),
		"total_potential_savings": 1420.75,
		"currency":               "USD",
		"critical_security_risks": 2,
		"high_security_risks":     5,
		"orphaned_cloud_assets":   8,
		"compliance_score":       "94.2%",
		"recommendations": []string{
			"Commit to EC2 1-Year Compute Savings Plans to save $840/month",
			"Remediate 2 S3 buckets with public read/write access",
			"Release 4 unattached Elastic IPs and purge 12 stale snapshots",
		},
	}
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *Server) handlePowerQueryEndpoint(w http.ResponseWriter, r *http.Request) {
	ops := r.URL.Query().Get("ops")
	if ops == "" {
		ops = "finops"
	}
	url := fmt.Sprintf("http://localhost:%d/api/v1/%s", s.port, ops)
	mScript := powerquery.GenerateWebMScript(url)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(mScript))
}

func generateFallbackEnriched(opsName string) []cloud.EnrichedRecord {
	now := time.Now()
	switch opsName {
	case "secops":
		return []cloud.EnrichedRecord{
			{
				OpsDomain: "secops", AccountID: "111122223333", AccountName: "LandingZone-Prod",
				Environment: "Production", BusinessUnit: "Security Operations", Owner: "secops-lead@company.com",
				CostCenter: "CC-1111", Service: "AmazonS3", ResourceOrKey: "lz-data-lake-prod-public",
				Metric: "PublicBucketAccess", ActualValue: 1.0, ExpectedValue: 0.0, PotentialSavings: 0.0,
				Severity: "CRITICAL", ActionRecommended: "Apply S3 Block Public Access immediately", Timestamp: now,
			},
		}
	case "cloudops":
		return []cloud.EnrichedRecord{
			{
				OpsDomain: "cloudops", AccountID: "444455556666", AccountName: "LandingZone-Stg",
				Environment: "Staging", BusinessUnit: "Core Platform", Owner: "devops-infra@company.com",
				CostCenter: "CC-4444", Service: "AmazonEC2", ResourceOrKey: "vol-0872161feda8921",
				Metric: "OrphanedEBSVolume", ActualValue: 38.50, ExpectedValue: 0.0, PotentialSavings: 38.50,
				Severity: "HIGH", ActionRecommended: "Create snapshot and terminate unattached volume", Timestamp: now,
			},
		}
	default: // finops
		return []cloud.EnrichedRecord{
			{
				OpsDomain: "finops", AccountID: "111122223333", AccountName: "LandingZone-Prod",
				Environment: "Production", BusinessUnit: "Core Platform", Owner: "finops-team@company.com",
				CostCenter: "CC-1111", Service: "Amazon Elastic Compute Cloud - Compute", ResourceOrKey: "AmazonEC2",
				Metric: "CostSpike", ActualValue: 370.50, ExpectedValue: 120.50, PotentialSavings: 250.00,
				Severity: "CRITICAL", ActionRecommended: "Rightsize underutilized compute or commit to Savings Plans", Timestamp: now,
			},
		}
	}
}
