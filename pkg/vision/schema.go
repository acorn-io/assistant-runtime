package vision

import (
	"github.com/acorn-io/aml"
	"github.com/acorn-io/aml/pkg/jsonschema"
	"github.com/acorn-io/aml/pkg/value"
)

var (
	Schema jsonschema.Schema
)

func init() {
	var schema value.Schema
	err := aml.Unmarshal([]byte(`
// Instructions on how the passed image should be analyzed
text:        string
// The base64 encoded value of the image if an image URL is not specified
base64:      string 
// The content type of the image such as "image/jpeg" or "image/png"
contentType: string
// The URL to the image to be processed. This should be set if base64 is not set
url:         string

`), &schema)
	if err != nil {
		panic(err)
	}

	v, _, err := value.NativeValue(schema)
	if err != nil {
		panic(err)
	}
	Schema = *(v.(*jsonschema.Schema))
}

type inputMessage struct {
	Text        string `json:"text,omitempty"`
	Base64      string `json:"base64,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	URL         string `json:"url,omitempty"`
}
