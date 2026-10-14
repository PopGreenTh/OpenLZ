package finops

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/attribution"
	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// attributeRows attributes every cost row to one Project path and its Biz path with
// internal/attribution (spec §6). The Cloud side of the row (account, connection, LZ, region,
// zone, service, usage, resource) is resolved without any Project tag, so a tag can never
// leak into it.
//
// The tag-scoring step compares every line with every node, so distinct line combinations are
// resolved once and joined back to the rows.
func attributeRows(ctx context.Context, engine *duckdb.Engine, rows []duckdb.AnalyzedCostRow, catalog *taxonomy.Catalog) ([]cloud.EnrichedRecord, error) {
	type cloudKey struct{ account, name, conn, csp, region, zone, service, usage, resource string }
	type lineKey struct{ resource, rg, account, region, lz, p, a string }

	cloudOut := map[cloudKey]taxonomy.Output{}
	lineIDs := map[lineKey]int{}
	var lines []attribution.Line
	rowCloud := make([]taxonomy.Output, len(rows))
	rowLine := make([]int, len(rows))

	for i, row := range rows {
		ck := cloudKey{row.AccountID, row.AccountName, row.ConnectionID, row.CSP, row.Region, row.Zone, row.Service, row.UsageType, row.ResourceID}
		co, ok := cloudOut[ck]
		if !ok {
			co = catalog.ResolveCloud(cloudInput(row))
			cloudOut[ck] = co
		}
		rowCloud[i] = co

		region := co.Region.ID
		if region == "" {
			region = row.Region
		}
		lk := lineKey{
			resource: row.ResourceID,
			rg:       co.ResourceGroup.ID,
			account:  co.Account.ID,
			region:   region,
			lz:       co.LZ.ID,
			p:        catalog.CodeOf(taxonomy.Projects, row.PrimaryTag),
			a:        catalog.CodeOf(taxonomy.Apps, row.SecondaryTag),
		}
		id, ok := lineIDs[lk]
		if !ok {
			id = len(lines) + 1
			lineIDs[lk] = id
			lines = append(lines, attribution.Line{
				ID: id, ResourceID: lk.resource, RG: lk.rg, Account: lk.account, Region: lk.region, LZ: lk.lz,
				P: lk.p, A: lk.a,
			})
		}
		rowLine[i] = id
	}

	tn, tev := catalog.AttributionTables()
	nodes := make([]attribution.Node, len(tn))
	for i, n := range tn {
		nodes[i] = attribution.Node{
			ID: n.ID, Level: n.Level,
			P: n.Codes[0], A: n.Codes[1], E: n.Codes[2], F: n.Codes[3], Fn: n.Codes[4],
			BU: n.Biz[0], CC: n.Biz[1], Own: n.Biz[2], PL: n.Biz[3], Cust: n.Biz[4],
		}
	}
	evidence := make([]attribution.Evidence, len(tev))
	for i, e := range tev {
		evidence[i] = attribution.Evidence{Kind: e.Kind, Key: e.Key, NodeID: e.NodeID}
	}

	results, err := engine.Attribute(ctx, nodes, evidence, lines)
	if err != nil {
		return nil, fmt.Errorf("attribute cost rows: %w", err)
	}
	byLine := make(map[int]attribution.Result, len(results))
	for _, r := range results {
		byLine[r.LineID] = r
	}

	out := make([]cloud.EnrichedRecord, 0, len(rows))
	for i, row := range rows {
		r, ok := byLine[rowLine[i]]
		if !ok {
			return nil, fmt.Errorf("attribution returned no result for line %d", rowLine[i])
		}
		res := rowCloud[i]
		p, b := catalog.ProjectSlots(r.ProjectHash), catalog.BizSlots(r.BizHash)
		res.Project, res.App, res.Environment, res.Feature, res.Function = p[0], p[1], p[2], p[3], p[4]
		res.BusinessUnit, res.CostCenter, res.OwnerTeam, res.ProductLine, res.Customer = b[0], b[1], b[2], b[3], b[4]
		rec := enrichedRecord(row, res)
		rec.AttributionTrail = r.Trail
		out = append(out, rec)
	}
	return out, nil
}
