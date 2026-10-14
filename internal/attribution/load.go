package attribution

import (
	"context"
	"database/sql"
	"fmt"
)

// Node is one row of the node table: a valid Project path prefix and the Biz codes its levels
// reference. Empty strings are stored as NULL ("not defined at this level").
type Node struct {
	ID                    int
	Level                 int // 1 (Project) .. 5 (Function)
	P, A, E, F, Fn        string
	BU, CC, Own, PL, Cust string
}

// Evidence is one row of the evidence table.
type Evidence struct {
	Kind   string // override|rg|account|region|lz
	Key    string
	NodeID int
}

// Alias is one row of the alias table: Value is the invalid tag found at Level of Hier
// ("project" or "biz"); Code is the correct code at that level.
type Alias struct {
	Hier  string
	Level int
	Value string
	Code  string
}

// Line is one row of the lines table: the inputs attribution reads for one cost line.
// Empty strings are stored as NULL.
type Line struct {
	ID                                   int
	ResourceID, RG, Account, Region, LZ  string
	P, A, E, F, Fn                       string // Project tags
	BUTag, CCTag, OwnTag, PLTag, CustTag string // Biz tags
}

// Load (re)creates the node, evidence, alias and lines tables on db and fills them.
func Load(ctx context.Context, db *sql.DB, nodes []Node, evidence []Evidence, aliases []Alias, lines []Line) error {
	for _, t := range []string{"node", "evidence", "alias", "lines"} {
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+t); err != nil {
			return fmt.Errorf("drop %s: %w", t, err)
		}
	}
	if _, err := db.ExecContext(ctx, Schema); err != nil {
		return fmt.Errorf("create attribution tables: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ins := func(query string, rows [][]any) error {
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, r := range rows {
			if _, err := stmt.ExecContext(ctx, r...); err != nil {
				return err
			}
		}
		return nil
	}
	var rows [][]any
	for _, n := range nodes {
		rows = append(rows, []any{n.ID, n.Level, nul(n.P), nul(n.A), nul(n.E), nul(n.F), nul(n.Fn),
			nul(n.BU), nul(n.CC), nul(n.Own), nul(n.PL), nul(n.Cust)})
	}
	if err := ins("INSERT INTO node VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", rows); err != nil {
		return fmt.Errorf("load node: %w", err)
	}
	rows = nil
	for _, e := range evidence {
		rows = append(rows, []any{e.Kind, e.Key, e.NodeID})
	}
	if err := ins("INSERT INTO evidence VALUES (?,?,?)", rows); err != nil {
		return fmt.Errorf("load evidence: %w", err)
	}
	rows = nil
	for _, a := range aliases {
		rows = append(rows, []any{a.Hier, a.Level, a.Value, a.Code})
	}
	if err := ins("INSERT INTO alias VALUES (?,?,?,?)", rows); err != nil {
		return fmt.Errorf("load alias: %w", err)
	}
	rows = nil
	for _, l := range lines {
		rows = append(rows, []any{l.ID, nul(l.ResourceID), nul(l.RG), nul(l.Account), nul(l.Region), nul(l.LZ),
			nul(l.P), nul(l.A), nul(l.E), nul(l.F), nul(l.Fn),
			nul(l.BUTag), nul(l.CCTag), nul(l.OwnTag), nul(l.PLTag), nul(l.CustTag)})
	}
	if err := ins("INSERT INTO lines VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", rows); err != nil {
		return fmt.Errorf("load lines: %w", err)
	}
	return tx.Commit()
}

func nul(s string) any {
	if s == "" {
		return nil
	}
	return s
}
