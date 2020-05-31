package model

type Base struct {
	tags Tags
}

func (b *Base) Tags() Tags {
	if b.tags == nil {
		b.tags = NewTags()
	}
	return b.tags
}
