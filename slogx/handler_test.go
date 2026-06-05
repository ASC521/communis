package slogx_test

import (
	"bytes"
	"testing"
	"testing/slogtest"

	"github.com/ASC521/communis/slogx"
)

func TestMyHandler(t *testing.T) {
	var buf bytes.Buffer
	err := slogtest.TestHandler(slogx.NewPipeHandler(&buf, nil), func() []map[string]any {
		return parseLogEntries(t, buf.Bytes())
	})
	if err != nil {
		t.Error(err)
	}
}

func parseLogEntries(t *testing.T, data []byte) []map[string]any {

}
