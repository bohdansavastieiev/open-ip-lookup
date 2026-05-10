package dataset

type IPKind int

const (
	IPKindUnallocated IPKind = iota
	IPKindSpecialUse
	IPKindRoutable
)

var ipKindDefs = []struct {
	kind  IPKind
	label string
}{
	{IPKindUnallocated, "Unallocated"},
	{IPKindSpecialUse, "Special Use"},
	{IPKindRoutable, "Routable"},
}

func (k IPKind) Label() string {
	for _, d := range ipKindDefs {
		if k == d.kind {
			return d.label
		}
	}
	return ""
}

func (k IPKind) IsBogon() bool {
	return k == IPKindUnallocated || k == IPKindSpecialUse
}
