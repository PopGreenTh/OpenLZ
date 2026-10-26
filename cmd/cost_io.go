package cmd

import (
	"github.com/PopGreenTh/OpenLZ/internal/cost"
	"github.com/PopGreenTh/OpenLZ/internal/store"
)

func writeRawParquet(path string, rows []cost.RawRow) error {
	rec := make([]store.RawRecord, len(rows))
	for i, r := range rows {
		rec[i] = store.RawRecord{
			LZ: r.LZ, AccountID: r.AccountID, Month: r.Month, Metric: r.Metric,
			Amount: r.Amount, AppTag: r.AppTag, Currency: r.Currency, Estimated: r.Estimated,
		}
	}
	return store.WriteRawParquet(path, rec)
}

func readRawParquet(path string) ([]cost.RawRow, error) {
	rec, err := store.ReadRawParquet(path)
	if err != nil {
		return nil, err
	}
	out := make([]cost.RawRow, len(rec))
	for i, r := range rec {
		out[i] = cost.RawRow{
			LZ: r.LZ, AccountID: r.AccountID, Month: r.Month, Metric: r.Metric,
			Amount: r.Amount, AppTag: r.AppTag, Currency: r.Currency, Estimated: r.Estimated,
		}
	}
	return out, nil
}

func writeAllocatedParquet(path string, rows []cost.AllocatedRow) error {
	rec := make([]store.AllocatedRecord, len(rows))
	for i, r := range rows {
		rec[i] = store.AllocatedRecord{
			LZ: r.LZ, AccountID: r.AccountID, AccountName: r.AccountName, OUPath: r.OUPath,
			Project: r.Project, App: r.App, Environment: r.Environment, BusinessUnit: r.BusinessUnit,
			CostCenter: r.CostCenter, Share: r.Share, ShareRule: r.ShareRule, AppRule: r.AppRule,
			EnvironmentRule: r.EnvironmentRule, AppTag: r.AppTag, Month: r.Month, Metric: r.Metric,
			Amount: r.Amount, Currency: r.Currency, Estimated: r.Estimated, TaxonomyHash: r.TaxonomyHash,
		}
	}
	return store.WriteAllocatedParquet(path, rec)
}
