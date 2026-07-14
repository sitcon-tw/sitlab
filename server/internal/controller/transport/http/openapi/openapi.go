package openapi

import _ "embed"

//go:embed openapi.json
var document []byte

func Document() []byte {
	copyOfDocument := make([]byte, len(document))
	copy(copyOfDocument, document)
	return copyOfDocument
}
