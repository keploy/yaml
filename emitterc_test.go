// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// largeDocument returns a mapping with n entries, each holding nested
// mappings, sequences and empty collections, and the exact text the
// encoder must produce for it.
func largeDocument(n int) (map[string]interface{}, string) {
	doc := make(map[string]interface{}, n)
	var want strings.Builder
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("k%06d", i)
		doc[key] = map[string]interface{}{
			"empty": map[string]interface{}{},
			"list":  []interface{}{i, "x", []interface{}{}},
			"name":  key,
			"nested": map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{"b": "c"},
				},
			},
		}
		fmt.Fprintf(&want, "%s:\n", key)
		want.WriteString("    empty: {}\n")
		want.WriteString("    list:\n")
		fmt.Fprintf(&want, "        - %d\n", i)
		want.WriteString("        - x\n        - []\n")
		fmt.Fprintf(&want, "    name: %s\n", key)
		want.WriteString("    nested:\n        a:\n")
		want.WriteString("            - b: c\n")
	}
	return doc, want.String()
}

// The emitter only needs a few events of lookahead, so its event queue
// must not keep every event of the document once they have been written.
func TestEmitterEventQueueStaysSmall(t *testing.T) {
	doc, want := largeDocument(10000)

	e := newEncoder()
	defer e.destroy()
	e.marshalDoc("", reflect.ValueOf(doc))
	e.finish()

	if got := string(e.out); got != want {
		t.Fatalf("output differs from the expected document")
	}
	if n := cap(e.emitter.events); n > initial_queue_size {
		t.Errorf("event queue grew to %d events, want at most %d",
			n, initial_queue_size)
	}
}

// A long-lived Encoder must not keep the events of every document it has
// written.
func TestEncoderEventQueueStaysSmallAcrossDocuments(t *testing.T) {
	doc, one := largeDocument(10)

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for i := 0; i < 1000; i++ {
		if err := enc.Encode(doc); err != nil {
			t.Fatal(err)
		}
	}
	if n := cap(enc.encoder.emitter.events); n > initial_queue_size {
		t.Errorf("event queue grew to %d events, want at most %d",
			n, initial_queue_size)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}

	want := one + strings.Repeat("---\n"+one, 999)
	if buf.String() != want {
		t.Fatalf("output differs from the expected stream")
	}
}

func BenchmarkEncodeLargeDocument(b *testing.B) {
	doc, _ := largeDocument(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(doc); err != nil {
			b.Fatal(err)
		}
	}
}
