package model

type Target interface {
	Hash() uint64
	Tags() Tags
	TUID() string
}

type Group interface {
	Targets() []Target
	Source() string
}
