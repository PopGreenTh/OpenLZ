package taxonomy

import "strings"

// Evidence kinds understood by internal/attribution.
const (
	EvidenceOverride = "override" // Resource id → Project node (always wins)
	EvidenceRG       = "rg"       // ResourceGroup code → Project node
	EvidenceRegion   = "region"   // Region code → Environment node (weak)
	EvidenceAcctEnv  = "acctenv"  // Account code → Environment node
	EvidenceAccount  = "account"  // Account code → App node
	EvidenceLZ       = "lz"       // LZ code → Project node (last resort)
)

// Evidence is one Cloud-to-Project link for the attribution query.
type Evidence struct {
	Kind   string
	Key    string
	NodeID int // ProjectNode.ID
}

// AttributionTables returns the node and evidence tables the attribution query reads:
// every valid Project path (plus the system UNSPEC nodes) and the Cloud links that point at
// them. Node ids are consistent between the two lists.
func (c *Catalog) AttributionTables() ([]ProjectNode, []Evidence) {
	nodes := c.ProjectNodes()
	id := map[string]int{}
	for _, n := range nodes {
		if !n.System {
			id[key(n.Path)] = n.ID
		}
	}
	var ev []Evidence
	add := func(kind, k string, target *node) {
		if target == nil || k == "" {
			return
		}
		if nid, ok := id[key(target.path)]; ok {
			ev = append(ev, Evidence{Kind: kind, Key: k, NodeID: nid})
		}
	}
	for _, n := range c.ordered[Resources] {
		add(EvidenceOverride, n.id, n.mapped)
	}
	for _, n := range c.ordered[ResourceGroups] {
		add(EvidenceRG, n.id, n.mapped)
	}
	// An account's own App links win; otherwise it is looked up through its OU chain, where
	// the nearest OU with linked Apps decides.
	for _, acc := range c.ordered[Accounts] {
		for _, app := range accountApps(acc) {
			add(EvidenceAccount, acc.id, app)
		}
	}
	for _, acc := range c.ordered[Accounts] {
		for _, env := range acc.multi[Environments] {
			add(EvidenceAcctEnv, acc.id, env)
		}
	}
	for _, env := range c.ordered[Environments] {
		for _, region := range env.multi[Regions] {
			add(EvidenceRegion, region.id, env)
		}
	}
	for _, prj := range c.ordered[Projects] {
		for _, lz := range prj.multi[LandingZones] {
			add(EvidenceLZ, lz.id, prj)
		}
	}
	return nodes, ev
}

// CodeOf returns the canonical code of the node a raw tag value names (by code or unique
// name) in coll. A value that matches nothing is returned cleaned but unchanged, so a typo
// stays visible to the attribution (and is repaired from deeper tags). Untagged is "".
func (c *Catalog) CodeOf(coll, raw string) string {
	if n := c.lookup(coll, raw); n != nil {
		return n.id
	}
	return clean(raw)
}

// SystemName is the display name of an UNSPEC or UNALLOC code ("" for any other code).
func SystemName(code string) string {
	switch {
	case strings.HasSuffix(code, "-UNSPEC"):
		return "Unspecified"
	case strings.HasSuffix(code, "-UNALLOC"):
		return "Unallocated"
	}
	return ""
}

func splitHash(hash string) []string {
	hash = strings.Trim(hash, "|")
	if hash == "" {
		return nil
	}
	return strings.Split(hash, "|")
}

// ProjectSlots names the five levels of a Project path hash such as |PRJ|APP|ENV|FEAT|FUNC|.
func (c *Catalog) ProjectSlots(hash string) [5]Slot {
	var out [5]Slot
	path := "|"
	for i, code := range splitHash(hash) {
		if i >= len(out) {
			break
		}
		path += code + "|"
		out[i] = Slot{ID: code, Name: SystemName(code)}
		if out[i].Name == "" {
			if n := c.byPath[key(path)]; n != nil {
				out[i].Name = n.name
			}
		}
	}
	return out
}

// BizSlots names the five levels of a Biz hash such as |BU|CC|TEAM|PL|CUST|.
func (c *Catalog) BizSlots(hash string) [5]Slot {
	var out [5]Slot
	levels := Levels(HierBiz)
	for i, code := range splitHash(hash) {
		if i >= len(out) {
			break
		}
		out[i] = Slot{ID: code, Name: SystemName(code)}
		if out[i].Name == "" {
			if n := c.lookup(levels[i], code); n != nil {
				out[i].Name = n.name
			}
		}
	}
	return out
}

// AliasRow repairs one invalid tag value at one level (spec §3.3, §6 step 1).
type AliasRow struct {
	Hier  string // "project" or "biz"
	Level int    // 1..5 within the hierarchy
	Value string // the invalid tag value as sensed
	Code  string // the correct code at that level
}

// AliasRows lists the alias table for the attribution query.
func (c *Catalog) AliasRows() []AliasRow {
	var out []AliasRow
	nProject := len(Levels(HierProject))
	for _, a := range c.ordered[Aliases] {
		level := indexOf(AliasLevels, a.attrs["level"])
		target := a.attrs["target"]
		row := AliasRow{Value: a.id}
		if level < nProject {
			n := c.byPath[key(target)]
			if n == nil {
				continue
			}
			row.Hier, row.Level, row.Code = "project", level+1, n.id
		} else {
			n := c.lookup(Levels(HierBiz)[level-nProject], target)
			if n == nil {
				continue
			}
			row.Hier, row.Level, row.Code = "biz", level-nProject+1, n.id
		}
		out = append(out, row)
	}
	return out
}
