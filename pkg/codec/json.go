package codec

import (
	"encoding/json"
	"io"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/orio"
)

// JSON is a factory for JSON encoders and decoders.
type JSON struct {
	MaxSnippetSize int
}

// NewEncoder does not do anything different from the default encoder.
// Unfortunately, the default encoder does not do any writes at all
// until the full encoding has succeeded, so we cannot buffer the
// write and add it to the error like is done for the json.Decoder.
func (j *JSON) NewEncoder(w io.Writer) *jsonEncoder { //nolint:revive // Why: we want to expose json.Encoder but override some methods
	return &jsonEncoder{json.NewEncoder(w)}
}

// NewDecoder returns a new encoding/json style decoder agumenting it
// with snippets of the payload in case of errors.
func (j *JSON) NewDecoder(r io.Reader) *jsonDecoder { //nolint:revive // Why: we want to expose json.Decoder but override some methods
	w := &orio.BufferedWriter{N: j.snippetSize()}
	d := json.NewDecoder(io.TeeReader(r, w))
	return &jsonDecoder{d, w}
}

func (j *JSON) snippetSize() int {
	if j.MaxSnippetSize > 0 {
		return j.MaxSnippetSize
	}
	return 2000
}

type jsonDecoder struct {
	// Decoder is embedded to export all underlying functionality.
	*json.Decoder

	// buf holds the snippet buffer
	buf *orio.BufferedWriter
}

// Decode wraps the standard encoding/json.Decoder but includes a
// snippet of the payload on errors.
func (j *jsonDecoder) Decode(v interface{}) error {
	if err := j.Decoder.Decode(v); err != nil {
		return orerr.Info(err, log.F{"error.json": string(j.buf.Bytes())})
	}
	return nil
}

type jsonEncoder struct {
	// Encoder is embedded to export al underlying functionality.
	*json.Encoder
}
