package cmd

import (
	"context"
	"fmt"

	cloudopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/cloudops"
	finopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/finops"
	secopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/secops"
	"github.com/spf13/cobra"
)

var (
	pipeSkipFinOps   bool
	pipeSkipSecOps   bool
	pipeSkipCloudOps bool
	pipeSkipEnforce  bool
	pipeDryRun       bool
	pipeMock         bool
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Run the full 5-stage OpenLZ pipeline across all (or selected) ops domains",
	Long:  "Executes Sense -> Analyze -> Enrich -> Enforce -> Report sequentially with granular skip options.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		fmt.Println("=========================================================")
		fmt.Println(" Starting OpenLZ Zero-ETL 5-Stage Multi-Ops Pipeline")
		fmt.Println("=========================================================")

		accounts := []string{"111122223333", "444455556666", "777788889999"}

		// 1. FinOps Pipeline
		if !pipeSkipFinOps {
			fmt.Println("\n>>> [Pipeline: FinOps Domain] <<<")
			rawP := "data/finops_raw.parquet"
			anomP := "data/finops_anomalies.parquet"
			enrichP := "data/finops_enriched.parquet"
			auditP := "data/finops_audit.parquet"

			fmt.Println(" (1/5) Sense: Concurrently querying Cost Explorer...")
			if _, err := finopsstages.ExecuteSenseCostExplorer(ctx, finopsstages.SenseCostExplorerInput{
				Accounts:    accounts,
				OutputPath:  rawP,
				CacheDBPath: cacheDBPath,
				Mock:        pipeMock,
			}); err != nil {
				return err
			}

			fmt.Println(" (2/5) Analyze: Executing DuckDB OLAP spike queries...")
			if _, err := finopsstages.ExecuteAnalyzeVarianceImpact(ctx, finopsstages.AnalyzeVarianceImpactInput{
				InputPath:  rawP,
				OutputPath: anomP,
				Threshold:  1.3,
			}); err != nil {
				return err
			}

			fmt.Println(" (3/5) Enrich: Annotating with Landing Zone metadata...")
			if _, err := finopsstages.ExecuteEnrichCostCenters(ctx, finopsstages.EnrichCostCentersInput{
				InputPath:  anomP,
				OutputPath: enrichP,
			}); err != nil {
				return err
			}

			if !pipeSkipEnforce {
				fmt.Printf(" (4/5) Enforce: Applying FinOps guardrails (dryRun=%v)...\n", pipeDryRun)
				if _, err := finopsstages.ExecuteEnforceTagging(ctx, finopsstages.EnforceTaggingInput{
					InputPath:  enrichP,
					OutputPath: auditP,
					DryRun:     pipeDryRun,
				}); err != nil {
					return err
				}
			} else {
				fmt.Println(" (4/5) Enforce: [SKIPPED by user flag]")
			}
		}

		// 2. SecOps Pipeline
		if !pipeSkipSecOps {
			fmt.Println("\n>>> [Pipeline: SecOps Domain] <<<")
			rawP := "data/secops_raw.parquet"
			anomP := "data/secops_anomalies.parquet"
			enrichP := "data/secops_enriched.parquet"
			auditP := "data/secops_audit.parquet"

			fmt.Println(" (1/5) Sense: Scanning security posture...")
			if _, err := secopsstages.ExecuteSensePosture(ctx, secopsstages.SensePostureInput{
				Accounts:   accounts,
				OutputPath: rawP,
				Mock:       pipeMock,
			}); err != nil {
				return err
			}

			fmt.Println(" (2/5) Analyze: Filtering critical vulnerabilities...")
			if _, err := secopsstages.ExecuteAnalysisRiskPosture(ctx, secopsstages.AnalysisRiskPostureInput{
				InputPath:  rawP,
				OutputPath: anomP,
			}); err != nil {
				return err
			}

			fmt.Println(" (3/5) Enrich: Annotating blast radius...")
			if _, err := secopsstages.ExecuteEnrichMetadata(ctx, secopsstages.EnrichMetadataInput{
				InputPath:  anomP,
				OutputPath: enrichP,
			}); err != nil {
				return err
			}

			if !pipeSkipEnforce {
				fmt.Printf(" (4/5) Enforce: Remediating open access (dryRun=%v)...\n", pipeDryRun)
				if _, err := secopsstages.ExecuteEnforceRemediation(ctx, secopsstages.EnforceRemediationInput{
					InputPath:  enrichP,
					OutputPath: auditP,
					DryRun:     pipeDryRun,
				}); err != nil {
					return err
				}
			} else {
				fmt.Println(" (4/5) Enforce: [SKIPPED by user flag]")
			}
		}

		// 3. CloudOps Pipeline
		if !pipeSkipCloudOps {
			fmt.Println("\n>>> [Pipeline: CloudOps Domain] <<<")
			rawP := "data/cloudops_raw.parquet"
			anomP := "data/cloudops_anomalies.parquet"
			enrichP := "data/cloudops_enriched.parquet"
			auditP := "data/cloudops_audit.parquet"

			fmt.Println(" (1/5) Sense: Scanning operational hygiene...")
			if _, err := cloudopsstages.ExecuteSenseHygiene(ctx, cloudopsstages.SenseHygieneInput{
				Accounts:   accounts,
				OutputPath: rawP,
				Mock:       pipeMock,
			}); err != nil {
				return err
			}

			fmt.Println(" (2/5) Analyze: Detecting orphaned assets & waste...")
			if _, err := cloudopsstages.ExecuteAnalysisWasteHygiene(ctx, cloudopsstages.AnalysisWasteHygieneInput{
				InputPath:  rawP,
				OutputPath: anomP,
			}); err != nil {
				return err
			}

			fmt.Println(" (3/5) Enrich: Adding lifecycle and owner tags...")
			if _, err := cloudopsstages.ExecuteEnrichLifecycle(ctx, cloudopsstages.EnrichLifecycleInput{
				InputPath:  anomP,
				OutputPath: enrichP,
			}); err != nil {
				return err
			}

			if !pipeSkipEnforce {
				fmt.Printf(" (4/5) Enforce: Cleaning up orphaned resources (dryRun=%v)...\n", pipeDryRun)
				if _, err := cloudopsstages.ExecuteEnforceCleanup(ctx, cloudopsstages.EnforceCleanupInput{
					InputPath:  enrichP,
					OutputPath: auditP,
					DryRun:     pipeDryRun,
				}); err != nil {
					return err
				}
			} else {
				fmt.Println(" (4/5) Enforce: [SKIPPED by user flag]")
			}
		}

		// 4. Report
		fmt.Println("\n>>> [Pipeline: Stage 5 - Executive Reporting] <<<")
		_ = reportExecutiveCmd.RunE(cmd, args)

		fmt.Println("\n=========================================================")
		fmt.Println(" Pipeline Execution Completed Successfully!")
		fmt.Println(" All Parquet artifacts generated in ./data/")
		fmt.Println(" Connect Power BI or Excel via: openlz report serve")
		fmt.Println("=========================================================")
		return nil
	},
}

func init() {
	pipelineCmd.Flags().BoolVar(&pipeSkipFinOps, "skip-finops", false, "Skip FinOps domain")
	pipelineCmd.Flags().BoolVar(&pipeSkipSecOps, "skip-secops", false, "Skip SecOps domain")
	pipelineCmd.Flags().BoolVar(&pipeSkipCloudOps, "skip-cloudops", false, "Skip CloudOps domain")
	pipelineCmd.Flags().BoolVar(&pipeSkipEnforce, "skip-enforce", false, "Skip Enforce remediation stage")
	pipelineCmd.Flags().BoolVar(&pipeDryRun, "dry-run", true, "Perform dry-run simulation for enforcement")
	pipelineCmd.Flags().BoolVar(&pipeMock, "mock", true, "Use mock/offline cloud data")
}
