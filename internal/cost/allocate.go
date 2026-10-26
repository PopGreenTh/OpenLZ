package cost

import (
	"fmt"
	"math"
	"sort"

	"github.com/PopGreenTh/OpenLZ/internal/mapping"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

const conservationEpsilon = 1e-6

// Allocate splits raw cost rows through the account map (WI-0104).
func Allocate(tax *taxonomy.Taxonomy, accountMap *mapping.AccountMapDocument, raw []RawRow) (AllocateResult, error) {
	if accountMap == nil {
		return AllocateResult{}, fmt.Errorf("account map is nil")
	}
	byAccount := groupAllocations(accountMap.Allocations)
	var out []AllocatedRow
	var findings []Finding

	sharedAccounts := map[string]struct{}{}
	for acct, allocs := range byAccount {
		if len(allocs) > 1 || (len(allocs) == 1 && allocs[0].ShareRule == "tag") {
			sharedAccounts[acct] = struct{}{}
		}
	}

	sawAppTag := false
	for _, row := range raw {
		if row.AppTag != "" {
			sawAppTag = true
		}
		rows, rowFindings, err := allocateRow(tax, byAccount, row, accountMap.TaxonomyHash)
		if err != nil {
			return AllocateResult{}, err
		}
		out = append(out, rows...)
		findings = append(findings, rowFindings...)
	}

	if len(sharedAccounts) > 0 && !sawAppTag {
		tagKey := "Application"
		if tax.CostAllocation != nil && tax.CostAllocation.AppTag != "" {
			tagKey = tax.CostAllocation.AppTag
		}
		findings = append(findings, Finding{
			Code:    "app-tag-not-seen",
			Subject: tagKey,
			Message: fmt.Sprintf("no cost rows carried App tag %q", tagKey),
		})
	}

	if err := checkConservation(raw, out); err != nil {
		return AllocateResult{}, err
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].AccountID != out[j].AccountID {
			return out[i].AccountID < out[j].AccountID
		}
		if out[i].App != out[j].App {
			return out[i].App < out[j].App
		}
		return out[i].AppTag < out[j].AppTag
	})
	sortFindings(findings)
	return AllocateResult{Rows: out, Findings: findings}, nil
}

func groupAllocations(allocs []mapping.Allocation) map[string][]mapping.Allocation {
	out := make(map[string][]mapping.Allocation)
	for _, a := range allocs {
		out[a.AccountID] = append(out[a.AccountID], a)
	}
	for acct := range out {
		sort.Slice(out[acct], func(i, j int) bool { return out[acct][i].App < out[acct][j].App })
	}
	return out
}

func allocateRow(tax *taxonomy.Taxonomy, byAccount map[string][]mapping.Allocation, row RawRow, taxonomyHash string) ([]AllocatedRow, []Finding, error) {
	var findings []Finding
	allocs := byAccount[row.AccountID]
	lzCurrency := lzCurrency(tax, row.LZ)

	if len(allocs) == 0 {
		findings = append(findings, Finding{
			Code:    "account-not-in-inventory",
			Subject: row.AccountID,
			Message: fmt.Sprintf("account %s has cost but no account map entry", row.AccountID),
		})
		r := unallocRow(row, taxonomyHash)
		return []AllocatedRow{r}, findings, nil
	}

	if lzCurrency != "" && row.Currency != "" && row.Currency != lzCurrency {
		findings = append(findings, Finding{
			Code:    "currency-mismatch",
			Subject: row.LZ + "/" + row.Month,
			Message: fmt.Sprintf("currency %s differs from LZ currency %s", row.Currency, lzCurrency),
		})
	}

	if len(allocs) == 1 && allocs[0].ShareRule != "tag" {
		return []AllocatedRow{fromMapping(row, allocs[0], row.Amount, 1, allocs[0].ShareRule, taxonomyHash)}, findings, nil
	}

	return allocateSharedRow(tax, row, allocs, taxonomyHash, &findings)
}

func allocateSharedRow(tax *taxonomy.Taxonomy, row RawRow, allocs []mapping.Allocation, taxonomyHash string, findings *[]Finding) ([]AllocatedRow, []Finding, error) {
	tag := row.AppTag
	if tag == "" {
		*findings = append(*findings, Finding{
			Code:    "shared-untagged",
			Subject: row.AccountID + "/" + row.Month,
			Message: "untagged shared-account cost split equally",
		})
		rows, _, err := splitUntagged(row, allocs, taxonomyHash)
		return rows, *findings, err
	}

	for _, a := range allocs {
		if TagMatchesApp(tag, a.App) {
			share := 1.0
			if a.Share != nil {
				share = *a.Share
			}
			return []AllocatedRow{fromMapping(row, a, row.Amount, share, "tag", taxonomyHash)}, *findings, nil
		}
	}

	if app, ok := tax.AppByTag(tag); ok {
		served := false
		for _, a := range allocs {
			if a.App == app.Code {
				served = true
				break
			}
		}
		if !served {
			*findings = append(*findings, Finding{
				Code:    "shared-tag-wrong-app",
				Subject: row.AccountID + "/" + tag,
				Message: fmt.Sprintf("tag %q matches app %q but account does not serve it", tag, app.Code),
			})
		}
	} else {
		*findings = append(*findings, Finding{
			Code:    "shared-tag-unknown",
			Subject: row.AccountID + "/" + tag,
			Message: fmt.Sprintf("tag %q matches no app", tag),
		})
	}

	env := taxonomy.ENVUnalloc
	envRule := "unalloc"
	if len(allocs) > 0 {
		env = allocs[0].Environment
		envRule = allocs[0].EnvironmentRule
	}
	invalid := mapping.Allocation{
		LZ: row.LZ, AccountID: row.AccountID, AccountName: allocs[0].AccountName, OUPath: allocs[0].OUPath,
		Project: taxonomy.PRJUnalloc, App: taxonomy.APPInvalid, Environment: env,
		BusinessUnit: taxonomy.BUUnalloc, CostCenter: taxonomy.CCUnalloc,
		AppRule: "invalid", EnvironmentRule: envRule,
	}
	return []AllocatedRow{fromMapping(row, invalid, row.Amount, 1, "tag", taxonomyHash)}, *findings, nil
}

func splitUntagged(row RawRow, allocs []mapping.Allocation, taxonomyHash string) ([]AllocatedRow, []Finding, error) {
	n := float64(len(allocs))
	if n == 0 {
		return nil, nil, fmt.Errorf("split untagged: no allocations")
	}
	share := 1 / n
	var rows []AllocatedRow
	remaining := row.Amount
	for i, a := range allocs {
		var amt float64
		if i == len(allocs)-1 {
			amt = remaining
		} else {
			amt = row.Amount * share
			remaining -= amt
		}
		rows = append(rows, fromMapping(row, a, amt, share, "untagged-equal", taxonomyHash))
	}
	return rows, nil, nil
}

func fromMapping(row RawRow, a mapping.Allocation, amount, share float64, shareRule, taxonomyHash string) AllocatedRow {
	return AllocatedRow{
		LZ:              row.LZ,
		AccountID:       row.AccountID,
		AccountName:     a.AccountName,
		OUPath:          a.OUPath,
		Project:         a.Project,
		App:             a.App,
		Environment:     a.Environment,
		BusinessUnit:    a.BusinessUnit,
		CostCenter:      a.CostCenter,
		Share:           share,
		ShareRule:       shareRule,
		AppRule:         a.AppRule,
		EnvironmentRule: a.EnvironmentRule,
		AppTag:          row.AppTag,
		Month:           row.Month,
		Metric:          row.Metric,
		Amount:          amount,
		Currency:        row.Currency,
		Estimated:       row.Estimated,
		TaxonomyHash:    taxonomyHash,
	}
}

func unallocRow(row RawRow, taxonomyHash string) AllocatedRow {
	return AllocatedRow{
		LZ: row.LZ, AccountID: row.AccountID, Project: taxonomy.PRJUnalloc, App: taxonomy.APPUnalloc,
		Environment: taxonomy.ENVUnalloc, BusinessUnit: taxonomy.BUUnalloc, CostCenter: taxonomy.CCUnalloc,
		Share: 1, ShareRule: "single", AppRule: "unalloc", EnvironmentRule: "unalloc",
		AppTag: row.AppTag, Month: row.Month, Metric: row.Metric, Amount: row.Amount,
		Currency: row.Currency, Estimated: row.Estimated, TaxonomyHash: taxonomyHash,
	}
}

func lzCurrency(tax *taxonomy.Taxonomy, lzCode string) string {
	lz, ok := tax.LZByCode(lzCode)
	if !ok || lz == nil {
		return ""
	}
	return lz.Currency
}

func checkConservation(raw []RawRow, allocated []AllocatedRow) error {
	type key struct {
		month, metric, currency string
	}
	rawSum := map[key]float64{}
	for _, r := range raw {
		k := key{r.Month, r.Metric, r.Currency}
		rawSum[k] += r.Amount
	}
	allocSum := map[key]float64{}
	for _, a := range allocated {
		k := key{a.Month, a.Metric, a.Currency}
		allocSum[k] += a.Amount
	}
	for k, want := range rawSum {
		got := allocSum[k]
		if math.Abs(got-want) > conservationEpsilon {
			return fmt.Errorf("conservation failed for %s/%s/%s: raw=%g allocated=%g", k.month, k.metric, k.currency, want, got)
		}
	}
	for k := range allocSum {
		if _, ok := rawSum[k]; !ok {
			return fmt.Errorf("conservation failed: unexpected allocated bucket %v", k)
		}
	}
	return nil
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return findings[i].Subject < findings[j].Subject
	})
}
