package mapping

// Source exposes inventory lookups for the resolver (WI-0103).
type Source interface {
	Organizations() []Organization
	PathFindings() []Finding
	OrgIndex(org Organization) *OrgIndex
}

// StaticSource is an in-memory inventory source.
type StaticSource struct {
	Orgs         []Organization
	Findings     []Finding
	indexByOrgID map[string]*OrgIndex
}

// NewStaticSource builds indexes for all organizations.
func NewStaticSource(inv Inventory) *StaticSource {
	s := &StaticSource{
		Orgs:         inv.Organizations,
		Findings:     inv.PathFindings,
		indexByOrgID: make(map[string]*OrgIndex, len(inv.Organizations)),
	}
	for _, org := range inv.Organizations {
		idx, findings := BuildOrgIndex(org)
		s.indexByOrgID[org.ID] = idx
		for _, f := range findings {
			s.Findings = append(s.Findings, Finding{
				Severity: SeverityWarning,
				Code:     f.Code,
				Subject:  f.Subject,
				Message:  f.Message,
			})
		}
	}
	return s
}

func (s *StaticSource) Organizations() []Organization { return s.Orgs }

func (s *StaticSource) PathFindings() []Finding { return s.Findings }

func (s *StaticSource) OrgIndex(org Organization) *OrgIndex {
	if idx, ok := s.indexByOrgID[org.ID]; ok {
		return idx
	}
	idx, _ := BuildOrgIndex(org)
	return idx
}
