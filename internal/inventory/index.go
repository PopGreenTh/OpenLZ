package inventory

import (
	"fmt"
	"sort"
	"strings"
)

const findingOUPathAmbiguous = "ou-path-ambiguous"

// Index provides OU path and lookup helpers for one organization.
type Index struct {
	org           Organization
	ouByID        map[string]OU
	pathByID      map[string]string
	pathAmbiguous map[string]struct{}
	childrenByOU  map[string][]string
	accountsByOU  map[string][]Account
}

// BuildIndex constructs lookups for org and reports ambiguous OU paths.
func BuildIndex(org Organization) (*Index, []PathFinding) {
	idx := &Index{
		org:           org,
		ouByID:        make(map[string]OU, len(org.OUs)),
		pathByID:      make(map[string]string),
		pathAmbiguous: make(map[string]struct{}),
		childrenByOU:  make(map[string][]string),
		accountsByOU:  make(map[string][]Account),
	}
	for _, ou := range org.OUs {
		idx.ouByID[ou.ID] = ou
		idx.childrenByOU[ou.ParentID] = append(idx.childrenByOU[ou.ParentID], ou.ID)
	}
	for _, acct := range org.Accounts {
		idx.accountsByOU[acct.ParentID] = append(idx.accountsByOU[acct.ParentID], acct)
	}

	findings := idx.buildPaths()
	return idx, findings
}

func (idx *Index) buildPaths() []PathFinding {
	idx.pathByID[idx.org.RootID] = "/"

	byParentName := make(map[string]map[string][]string)
	queue := make([]string, 0, len(idx.org.OUs))
	for _, ou := range idx.org.OUs {
		if ou.ParentID == idx.org.RootID {
			queue = append(queue, ou.ID)
		}
	}

	seen := map[string]struct{}{}
	for len(queue) > 0 {
		ouID := queue[0]
		queue = queue[1:]
		if _, ok := seen[ouID]; ok {
			continue
		}
		seen[ouID] = struct{}{}

		ou := idx.ouByID[ouID]
		parentPath := idx.pathForParent(ou.ParentID)
		if parentPath == "" && ou.ParentID != idx.org.RootID {
			continue
		}
		if ou.ParentID == idx.org.RootID {
			parentPath = "/"
		}
		childPath := joinPath(parentPath, ou.Name)
		idx.pathByID[ou.ID] = childPath

		key := ou.ParentID
		if byParentName[key] == nil {
			byParentName[key] = make(map[string][]string)
		}
		byParentName[key][ou.Name] = append(byParentName[key][ou.Name], ou.ID)

		for _, childID := range idx.childrenByOU[ouID] {
			queue = append(queue, childID)
		}
	}

	var findings []PathFinding
	for parentID, names := range byParentName {
		for name, ids := range names {
			if len(ids) < 2 {
				continue
			}
			for _, id := range ids {
				idx.pathAmbiguous[id] = struct{}{}
				delete(idx.pathByID, id)
			}
			parentPath := idx.pathForParent(parentID)
			if parentID == idx.org.RootID {
				parentPath = "/"
			}
			subject := joinPath(parentPath, name)
			findings = append(findings, PathFinding{
				Code:    findingOUPathAmbiguous,
				Subject: subject,
				Message: fmt.Sprintf("multiple OUs named %q under the same parent", name),
			})
		}
	}
	return findings
}

func (idx *Index) pathForParent(parentID string) string {
	if parentID == idx.org.RootID {
		return "/"
	}
	if _, amb := idx.pathAmbiguous[parentID]; amb {
		return ""
	}
	return idx.pathByID[parentID]
}

func joinPath(parent, name string) string {
	if parent == "/" {
		return "/" + name
	}
	return parent + "/" + name
}

// OUByID returns the OU with the given ID, or false if unknown.
func (idx *Index) OUByID(id string) (OU, bool) {
	ou, ok := idx.ouByID[id]
	return ou, ok
}

// OUByPath resolves a path to an OU. Returns false when the path is unknown or ambiguous.
func (idx *Index) OUByPath(path string) (OU, bool) {
	if path == "/" {
		return OU{ID: idx.org.RootID, Name: "", ParentID: ""}, true
	}
	for id, p := range idx.pathByID {
		if p == path {
			if _, amb := idx.pathAmbiguous[id]; amb {
				return OU{}, false
			}
			ou := idx.ouByID[id]
			return ou, true
		}
	}
	return OU{}, false
}

// HasChildOUs reports whether the OU has any child OUs.
func (idx *Index) HasChildOUs(ouID string) bool {
	children := idx.childrenByOU[ouID]
	return len(children) > 0
}

// AccountOUChain returns OUs from the root down to the account's parent OU.
func (idx *Index) AccountOUChain(accountID string) ([]OU, bool) {
	var acct *Account
	for i := range idx.org.Accounts {
		if idx.org.Accounts[i].ID == accountID {
			acct = &idx.org.Accounts[i]
			break
		}
	}
	if acct == nil {
		return nil, false
	}
	if acct.ParentID == idx.org.RootID {
		return nil, true
	}
	var chain []OU
	cur := acct.ParentID
	for cur != idx.org.RootID {
		if _, amb := idx.pathAmbiguous[cur]; amb {
			return nil, false
		}
		ou, ok := idx.ouByID[cur]
		if !ok {
			return nil, false
		}
		chain = append([]OU{ou}, chain...)
		cur = ou.ParentID
	}
	return chain, true
}

// AccountPath returns the OU path for an account, or empty when unknown or ambiguous.
func (idx *Index) AccountPath(accountID string) string {
	for i := range idx.org.Accounts {
		if idx.org.Accounts[i].ID != accountID {
			continue
		}
		parent := idx.org.Accounts[i].ParentID
		if parent == idx.org.RootID {
			return "/"
		}
		if _, amb := idx.pathAmbiguous[parent]; amb {
			return ""
		}
		return idx.pathByID[parent]
	}
	return ""
}

// Organization returns the indexed organization snapshot.
func (idx *Index) Organization() Organization {
	return idx.org
}

// FormatShowTree renders OU paths and accounts for inventory show.
func FormatShowTree(org Organization) string {
	idx, _ := BuildIndex(org)
	var b strings.Builder

	var walk func(parentID, path string)
	walk = func(parentID, path string) {
		fmt.Fprintf(&b, "OU %s\n", path)
		for _, acct := range idx.accountsByOU[parentID] {
			email := acct.Email
			if email != "" {
				fmt.Fprintf(&b, "  account %s %s <%s> %s\n", acct.ID, acct.Name, email, acct.Status)
			} else {
				fmt.Fprintf(&b, "  account %s %s %s\n", acct.ID, acct.Name, acct.Status)
			}
		}
		childIDs := append([]string(nil), idx.childrenByOU[parentID]...)
		sort.Slice(childIDs, func(a, b int) bool {
			return idx.pathByID[childIDs[a]] < idx.pathByID[childIDs[b]]
		})
		for _, childID := range childIDs {
			if _, amb := idx.pathAmbiguous[childID]; amb {
				continue
			}
			childPath := idx.pathByID[childID]
			walk(childID, childPath)
		}
	}
	walk(org.RootID, "/")
	return strings.TrimRight(b.String(), "\n")
}
