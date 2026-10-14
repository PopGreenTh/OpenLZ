package delta

import (
	"fmt"
	"io"
	"sort"
	"time"
)

type moveKey struct{ from, to, currency, metric string }

// Movement is the cost that moved from one Project path to another in one run.
type Movement struct {
	From, To         string
	Currency, Metric string
	Lines            int
	Amount           float64
}

// LineMove is a line that moved to or from an UNSPEC/UNALLOC path; it feeds the audit view
// where an analyst turns it into an override rule.
type LineMove struct {
	LineKey  string
	From, To string
	Currency string
	Amount   float64
}

// Report summarizes one Sync run.
type Report struct {
	RunAt               time.Time
	TaxonomyVersion     string
	PrevTaxonomyVersion string

	// Data delta counts by kind.
	New, Amount, Keys, Removed int

	Selected     int // lines considered for re-attribution
	Reattributed int // lines whose result changed (including NEW lines)
	RowsWritten  int // rows written to the store; 0 for an idempotent run

	Moved      []Movement
	Unresolved []LineMove
	Totals     map[string]float64 // active cost per "currency/metric"
}

func (r *Report) sort() {
	sort.Slice(r.Moved, func(i, j int) bool {
		a, b := r.Moved[i], r.Moved[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		if a.Currency != b.Currency {
			return a.Currency < b.Currency
		}
		return a.Metric < b.Metric
	})
	sort.Slice(r.Unresolved, func(i, j int) bool { return r.Unresolved[i].LineKey < r.Unresolved[j].LineKey })
}

// Print writes the movement report.
func (r *Report) Print(w io.Writer) {
	fmt.Fprintf(w, "CloudValueDelta  taxonomy %s (previous %s)\n", r.TaxonomyVersion, orNone(r.PrevTaxonomyVersion))
	fmt.Fprintf(w, "  data delta   NEW %d  AMOUNT %d  KEYS %d  REMOVED %d\n", r.New, r.Amount, r.Keys, r.Removed)
	fmt.Fprintf(w, "  re-attributed %d of %d selected lines, %d rows written\n", r.Reattributed, r.Selected, r.RowsWritten)
	if len(r.Moved) > 0 {
		fmt.Fprintln(w, "  cost moved between projects:")
		for _, m := range r.Moved {
			fmt.Fprintf(w, "    %s -> %s  %d lines  %.2f %s (%s)\n", m.From, m.To, m.Lines, m.Amount, m.Currency, m.Metric)
		}
	}
	if len(r.Unresolved) > 0 {
		fmt.Fprintln(w, "  lines that moved to or from UNSPEC/UNALLOC:")
		for _, l := range r.Unresolved {
			fmt.Fprintf(w, "    %s  %s -> %s  %.2f %s\n", l.LineKey, l.From, l.To, l.Amount, l.Currency)
		}
	}
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
