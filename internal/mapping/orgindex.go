package mapping

import "fmt"

const codeOUPathAmbiguous = "ou-path-ambiguous"

type pathFinding struct {
	Code    string
	Subject string
	Message string
}

// OrgIndex provides OU path and lookup helpers for one organization.
type OrgIndex struct {
	org           Organization
	ouByID        map[string]OU
	pathByID      map[string]string
	pathAmbiguous map[string]struct{}
	childrenByOU  map[string][]string
}

// BuildOrgIndex constructs lookups for org and reports ambiguous OU paths.
func BuildOrgIndex(org Organization) (*OrgIndex, []pathFinding) {
	idx := &OrgIndex{
		org:           org,
		ouByID:        make(map[string]OU, len(org.OUs)),
		pathByID:      make(map[string]string),
		pathAmbiguous: make(map[string]struct{}),
		childrenByOU:  make(map[string][]string),
	}
	for _, ou := range org.OUs {
		idx.ouByID[ou.ID] = ou
		idx.childrenByOU[ou.ParentID] = append(idx.childrenByOU[ou.ParentID], ou.ID)
	}
	return idx, idx.buildPaths()
}

func (idx *OrgIndex) buildPaths() []pathFinding {
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
		idx.pathByID[ou.ID] = joinOUPath(parentPath, ou.Name)
		key := ou.ParentID
		if byParentName[key] == nil {
			byParentName[key] = make(map[string][]string)
		}
		byParentName[key][ou.Name] = append(byParentName[key][ou.Name], ou.ID)
		for _, childID := range idx.childrenByOU[ouID] {
			queue = append(queue, childID)
		}
	}
	var findings []pathFinding
	for parentID, names := range byParentName {
		for name, ids := range names {
			if len(ids) < 2 {
				continue
			}
			for _, id := range ids {
				idx.pathAmbiguous[id] = struct{}{}
				idx.invalidatePathsUnder(id)
			}
			parentPath := idx.pathForParent(parentID)
			if parentID == idx.org.RootID {
				parentPath = "/"
			}
			findings = append(findings, pathFinding{
				Code:    codeOUPathAmbiguous,
				Subject: joinOUPath(parentPath, name),
				Message: fmt.Sprintf("multiple OUs named %q under the same parent", name),
			})
		}
	}
	return findings
}

func (idx *OrgIndex) invalidatePathsUnder(ouID string) {
	delete(idx.pathByID, ouID)
	for _, childID := range idx.childrenByOU[ouID] {
		idx.invalidatePathsUnder(childID)
	}
}

func (idx *OrgIndex) pathForParent(parentID string) string {
	if parentID == idx.org.RootID {
		return "/"
	}
	if _, amb := idx.pathAmbiguous[parentID]; amb {
		return ""
	}
	return idx.pathByID[parentID]
}

func joinOUPath(parent, name string) string {
	if parent == "/" {
		return "/" + name
	}
	return parent + "/" + name
}

func (idx *OrgIndex) OUByID(id string) (OU, bool) {
	ou, ok := idx.ouByID[id]
	return ou, ok
}

func (idx *OrgIndex) OUByPath(path string) (OU, bool) {
	if path == "/" {
		return OU{ID: idx.org.RootID}, true
	}
	var match OU
	var count int
	for id, p := range idx.pathByID {
		if p != path {
			continue
		}
		if _, amb := idx.pathAmbiguous[id]; amb {
			return OU{}, false
		}
		match = idx.ouByID[id]
		count++
		if count > 1 {
			return OU{}, false
		}
	}
	if count == 0 {
		return OU{}, false
	}
	return match, true
}

func (idx *OrgIndex) HasChildOUs(ouID string) bool {
	return len(idx.childrenByOU[ouID]) > 0
}

func (idx *OrgIndex) AccountOUChain(accountID string) ([]OU, bool) {
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
		ou, ok := idx.ouByID[cur]
		if !ok {
			return nil, false
		}
		chain = append([]OU{ou}, chain...)
		cur = ou.ParentID
	}
	return chain, true
}

// OUPath returns the path for an OU ID, or empty when unknown or ambiguous.
func (idx *OrgIndex) OUPath(ouID string) string {
	if ouID == idx.org.RootID {
		return "/"
	}
	if _, amb := idx.pathAmbiguous[ouID]; amb {
		return ""
	}
	return idx.pathByID[ouID]
}

func (idx *OrgIndex) AccountPath(accountID string) string {
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

func (idx *OrgIndex) AccountByID(id string) (Account, bool) {
	for _, acct := range idx.org.Accounts {
		if acct.ID == id {
			return acct, true
		}
	}
	return Account{}, false
}

func (idx *OrgIndex) AccountIDs() map[string]struct{} {
	out := make(map[string]struct{}, len(idx.org.Accounts))
	for _, acct := range idx.org.Accounts {
		out[acct.ID] = struct{}{}
	}
	return out
}
