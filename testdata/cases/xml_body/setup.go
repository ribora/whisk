package xml_body

type Doc struct {
	ID   string `xml:"id,attr"`
	Body string `xml:"body" validate:"required"`
}
