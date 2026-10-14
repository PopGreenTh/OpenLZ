package finops

import (
	"context"

	"github.com/PopGreenTh/OpenLZ/internal/attribution"
	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// attributeRows attributes every analyzed cost row to one Project path and its Biz path with
// internal/attribution (spec §6), and keeps the Cloud side resolved from the row itself.
func attributeRows(ctx context.Context, engine *duckdb.Engine, rows []duckdb.AnalyzedCostRow, catalog *taxonomy.Catalog) ([]cloud.EnrichedRecord, error) {
	in := make([]attribution.Row, len(rows))
	for i, r := range rows {
		in[i] = attribution.Row{
			AccountID: r.AccountID, AccountName: r.AccountName, ConnectionID: r.ConnectionID,
			CSP: r.CSP, Region: r.Region, Zone: r.Zone,
			Service: r.Service, UsageType: r.UsageType, ResourceID: r.ResourceID,
			PrimaryTag: r.PrimaryTag, SecondaryTag: r.SecondaryTag,
		}
	}
	attributed, err := attribution.ResolveRows(ctx, engine.Attribute, catalog, in)
	if err != nil {
		return nil, err
	}
	out := make([]cloud.EnrichedRecord, 0, len(rows))
	for i, row := range rows {
		res := attributed[i].Cloud
		p, b := catalog.ProjectSlots(attributed[i].Result.ProjectHash), catalog.BizSlots(attributed[i].Result.BizHash)
		res.Project, res.App, res.Environment, res.Feature, res.Function = p[0], p[1], p[2], p[3], p[4]
		res.BusinessUnit, res.CostCenter, res.OwnerTeam, res.ProductLine, res.Customer = b[0], b[1], b[2], b[3], b[4]
		rec := enrichedRecord(row, res)
		rec.AttributionTrail = attributed[i].Result.Trail
		out = append(out, rec)
	}
	return out, nil
}
