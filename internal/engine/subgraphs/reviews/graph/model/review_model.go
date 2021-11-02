package model

type Review struct {
	ID       string
	Body     *string
	AuthorID string
	Product  Product
	Metadata []MetadataOrError
}

func (Review) IsEntity() {}
