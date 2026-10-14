package taxonomy

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// canonicalRow renders a row with sorted keys. Names are display text only and never change
// an attribution, so they are left out unless withName is set.
func canonicalRow(row Row, withName bool) string {
	keys := make([]string, 0, len(row))
	for k, v := range row {
		if v == "" || (k == "name" && !withName) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(row[k])
		b.WriteByte(';')
	}
	return b.String()
}

// Version identifies the content of a snapshot: the same data always gives the same version,
// whatever the order of the rows in the file.
func Version(snap *Snapshot) string {
	h := sha256.New()
	for _, spec := range Schema {
		rows := make([]string, 0, len(snap.Rows(spec.Name)))
		for _, r := range snap.Rows(spec.Name) {
			rows = append(rows, canonicalRow(r, true))
		}
		sort.Strings(rows)
		h.Write([]byte(spec.Name + "\n"))
		for _, r := range rows {
			h.Write([]byte(r + "\n"))
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ChangeSet is the mapping delta between two taxonomy snapshots (spec §8.1): what a Directus
// edit changed in the tree, the evidence and the alias table.
type ChangeSet struct {
	OldVersion, NewVersion string

	NodesAdded    []string // Project paths that exist only in the new snapshot
	NodesRemoved  []string // Project paths that exist only in the old one (removed, renamed, moved)
	NodesModified []string // same path, different Biz refs

	// Keys are the changed evidence, alias and cloud keys, written "<kind>:<key>":
	// resource, rg, account, region, lz, connection and alias:<level>|<value>.
	Keys map[string]struct{}

	// EvidenceAdded is set when an evidence or alias key exists only in the new snapshot.
	EvidenceAdded bool
}

// NodeSetChanged reports whether any Project node was added, removed or modified.
func (c ChangeSet) NodeSetChanged() bool {
	return len(c.NodesAdded)+len(c.NodesRemoved)+len(c.NodesModified) > 0
}

// ChangedPaths lists the paths whose lines must be re-attributed: removed and modified nodes.
func (c ChangeSet) ChangedPaths() []string {
	return append(append([]string(nil), c.NodesRemoved...), c.NodesModified...)
}

// AddedAnything is set when nodes or evidence were added, so unresolved lines may improve.
func (c ChangeSet) AddedAnything() bool { return len(c.NodesAdded) > 0 || c.EvidenceAdded }

// Empty reports whether the two snapshots attribute identically.
func (c ChangeSet) Empty() bool { return !c.NodeSetChanged() && len(c.Keys) == 0 }

// Diff compares two snapshots. Both must be valid.
func Diff(oldSnap, newSnap *Snapshot) (ChangeSet, error) {
	oldCat, err := Build(oldSnap)
	if err != nil {
		return ChangeSet{}, err
	}
	newCat, err := Build(newSnap)
	if err != nil {
		return ChangeSet{}, err
	}
	cs := ChangeSet{OldVersion: Version(oldSnap), NewVersion: Version(newSnap), Keys: map[string]struct{}{}}

	// Project tree: paths and the Biz codes each path references.
	oldNodes, newNodes := nodeTuples(oldCat), nodeTuples(newCat)
	for p, t := range newNodes {
		if ot, ok := oldNodes[p]; !ok {
			cs.NodesAdded = append(cs.NodesAdded, p)
		} else if ot != t {
			cs.NodesModified = append(cs.NodesModified, p)
		}
	}
	for p := range oldNodes {
		if _, ok := newNodes[p]; !ok {
			cs.NodesRemoved = append(cs.NodesRemoved, p)
		}
	}
	sort.Strings(cs.NodesAdded)
	sort.Strings(cs.NodesRemoved)
	sort.Strings(cs.NodesModified)

	// Evidence: the Project paths each Cloud key points at.
	oldEv, newEv := evidenceSets(oldCat), evidenceSets(newCat)
	for k, v := range newEv {
		ov, ok := oldEv[k]
		if !ok {
			cs.EvidenceAdded = true
		}
		if !ok || ov != v {
			cs.Keys[k] = struct{}{}
		}
	}
	for k := range oldEv {
		if _, ok := newEv[k]; !ok {
			cs.Keys[k] = struct{}{}
		}
	}

	// Cloud rows that decide a line's inputs, and the alias table.
	for coll, prefix := range map[string]string{Resources: "resource", Accounts: "account", Connections: "connection", Regions: "region"} {
		changedRows(oldSnap, newSnap, coll, func(code string) { cs.Keys[prefix+":"+code] = struct{}{} })
	}
	changedRows(oldSnap, newSnap, Aliases, func(code string) { cs.Keys["alias:"+code] = struct{}{} })
	for _, a := range newCat.ordered[Aliases] {
		if findRow(oldSnap, Aliases, a.attrs["level"], a.id) == nil {
			cs.EvidenceAdded = true
		}
	}
	return cs, nil
}

// AliasKey is the change key of an alias row.
func AliasKey(level, value string) string { return "alias:" + level + "|" + value }

func findRow(snap *Snapshot, coll, level, code string) Row {
	for _, r := range snap.Rows(coll) {
		if r.Code() == code && (level == "" || r["level"] == level) {
			return r
		}
	}
	return nil
}

// changedRows calls fn with the code of every row that was added, removed or changed
// (ignoring names). Alias rows are keyed by level and value.
func changedRows(oldSnap, newSnap *Snapshot, coll string, fn func(code string)) {
	index := func(s *Snapshot) map[string]string {
		m := map[string]string{}
		for _, r := range s.Rows(coll) {
			code := r.Code()
			if coll == Aliases {
				code = r["level"] + "|" + code
			}
			m[code] = canonicalRow(r, false)
		}
		return m
	}
	o, n := index(oldSnap), index(newSnap)
	for code, v := range n {
		if ov, ok := o[code]; !ok || ov != v {
			fn(code)
		}
	}
	for code := range o {
		if _, ok := n[code]; !ok {
			fn(code)
		}
	}
}

func nodeTuples(c *Catalog) map[string]string {
	out := map[string]string{}
	for _, n := range c.ProjectNodes() {
		if !n.System {
			out[n.Path] = strings.Join(n.Biz[:], "|")
		}
	}
	return out
}

// evidenceSets maps "<kind>:<key>" to the sorted Project paths it points at.
func evidenceSets(c *Catalog) map[string]string {
	nodes, ev := c.AttributionTables()
	path := map[int]string{}
	for _, n := range nodes {
		path[n.ID] = n.Path
	}
	paths := map[string][]string{}
	kindName := map[string]string{EvidenceOverride: "resource"}
	for _, e := range ev {
		kind := e.Kind
		if n, ok := kindName[kind]; ok {
			kind = n
		}
		k := kind + ":" + e.Key
		paths[k] = append(paths[k], path[e.NodeID])
	}
	out := map[string]string{}
	for k, p := range paths {
		sort.Strings(p)
		out[k] = strings.Join(p, ",")
	}
	return out
}
