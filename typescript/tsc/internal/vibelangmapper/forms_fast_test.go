package vibelangmapper

import (
	"bytes"
	"testing"
)

// TestEmbeddedTableRejectsBrokenFormat ensures format changes fail explicitly.
func TestEmbeddedTableRejectsBrokenFormat(t *testing.T) {
	for _, data := range [][]byte{
		embeddedTable[:len(embeddedTable)-3],
		bytes.Replace(embeddedTable, []byte(`"sigs":`), []byte(`"unknown":`), 1),
		bytes.Replace(embeddedTable, []byte(`"n":"`), []byte(`"n":"bad`), 1),
	} {
		if _, err := readTable(data); err == nil {
			t.Fatal("accepted broken format")
		}
	}
}
