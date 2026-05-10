package source

type FreshnessKind int

const (
	FreshnessKindLastModified FreshnessKind = iota + 1
	FreshnessKindETag
)

func (fk FreshnessKind) isValid() bool {
	switch fk {
	case FreshnessKindLastModified, FreshnessKindETag:
		return true
	default:
		return false
	}
}

type ArtifactKind int

const (
	ArtifactKindDirectFile ArtifactKind = iota + 1
	ArtifactKindTarGzFile
	ArtifactKindTarGzDir
)

func (ak ArtifactKind) isValid() bool {
	switch ak {
	case ArtifactKindDirectFile, ArtifactKindTarGzFile, ArtifactKindTarGzDir:
		return true
	default:
		return false
	}
}

type AuthKind int

const (
	AuthKindNone AuthKind = iota + 1
	AuthKindMaxMind
)

func (ak AuthKind) isValid() bool {
	switch ak {
	case AuthKindNone, AuthKindMaxMind:
		return true
	default:
		return false
	}
}
