package cost

import "testing"

func TestConservationFailure(t *testing.T) {
	raw := []RawRow{{Month: "2026-09", Metric: "UnblendedCost", Amount: 100, Currency: "USD"}}
	allocated := []AllocatedRow{{Month: "2026-09", Metric: "UnblendedCost", Amount: 99, Currency: "USD"}}
	if err := checkConservation(raw, allocated); err == nil {
		t.Fatal("expected conservation error")
	}
}
