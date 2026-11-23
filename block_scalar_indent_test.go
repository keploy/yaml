package yaml_test

import (
	"bytes"
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"
)

// A block scalar whose value starts with a space or a line break needs an
// indentation indicator, and the indicator is the distance from the parent
// node's indentation to the content's. The emitter wrote best_indent there
// whatever the content's indentation came to, and inside a sequence item a
// mapping sits two columns off a multiple of best_indent, so the content
// landed two columns deeper than the indicator said: `|4` over content two
// columns in, which no parser reads back ("did not find expected key"). A
// keploy report whose replayed body was Django's 500 page (it starts with
// "\n") was written so, and every later read of the report failed.
func TestBlockScalarWithLeadingBreakOrSpaceRoundTrips(t *testing.T) {
	values := []string{
		"\n<!doctype html>\n<html lang=\"en\">\n  <title>Server Error (500)</title>\n</html>\n",
		"\n\nsecond\nthird\n",
		"  leading spaces\nnext line\n",
		" one space\n  two\n",
		"\n",
		"\nno trailing break",
	}
	type leaf struct {
		Normal bool   `yaml:"normal"`
		Actual string `yaml:"actual"`
	}
	shapes := map[string]func(v string) interface{}{
		"root scalar":           func(v string) interface{} { return v },
		"mapping value":         func(v string) interface{} { return map[string]string{"actual": v} },
		"sequence item":         func(v string) interface{} { return []string{v} },
		"mapping in a sequence": func(v string) interface{} { return []leaf{{Actual: v}} },
		"mapping in a sequence in a mapping in a sequence": func(v string) interface{} {
			return []map[string][]leaf{{"body_result": {{Actual: v}}}}
		},
		"sequence in a sequence": func(v string) interface{} { return [][]string{{v}} },
	}
	for _, indent := range []int{2, 3, 4, 8} {
		for name, shape := range shapes {
			for i, v := range values {
				t.Run(fmt.Sprintf("indent %d/%s/%d", indent, name, i), func(t *testing.T) {
					in := shape(v)
					var buf bytes.Buffer
					enc := yaml.NewEncoder(&buf)
					enc.SetIndent(indent)
					if err := enc.Encode(in); err != nil {
						t.Fatalf("encode: %v", err)
					}
					if err := enc.Close(); err != nil {
						t.Fatalf("close: %v", err)
					}
					var node yaml.Node
					if err := yaml.Unmarshal(buf.Bytes(), &node); err != nil {
						t.Fatalf("the emitted document does not parse: %v\n%s", err, buf.String())
					}
					var back interface{}
					switch in.(type) {
					case string:
						var s string
						if err := yaml.Unmarshal(buf.Bytes(), &s); err != nil {
							t.Fatal(err)
						}
						back = s
					case map[string]string:
						m := map[string]string{}
						if err := yaml.Unmarshal(buf.Bytes(), &m); err != nil {
							t.Fatal(err)
						}
						back = m
					case []string:
						var s []string
						if err := yaml.Unmarshal(buf.Bytes(), &s); err != nil {
							t.Fatal(err)
						}
						back = s
					case []leaf:
						var s []leaf
						if err := yaml.Unmarshal(buf.Bytes(), &s); err != nil {
							t.Fatal(err)
						}
						back = s
					case []map[string][]leaf:
						var s []map[string][]leaf
						if err := yaml.Unmarshal(buf.Bytes(), &s); err != nil {
							t.Fatal(err)
						}
						back = s
					case [][]string:
						var s [][]string
						if err := yaml.Unmarshal(buf.Bytes(), &s); err != nil {
							t.Fatal(err)
						}
						back = s
					}
					if fmt.Sprintf("%#v", back) != fmt.Sprintf("%#v", in) {
						t.Fatalf("round trip changed the value:\n in: %#v\nout: %#v\n%s", in, back, buf.String())
					}
				})
			}
		}
	}
}

// The same, through nodes, for what Marshal never picks on its own: folded
// style, and a line comment on the block scalar's header (its write ends the
// header line, so the emitter must not end it a second time). And values that
// start with a tab: a tab is leading white space, so the content needs an
// indentation indicator as it does for a space.
func TestBlockScalarNodesRoundTrip(t *testing.T) {
	values := []string{
		"\n<!doctype html>\n<html>\n  <title>Server Error (500)</title>\n</html>\n",
		"\n\nsecond\nthird\n",
		"  leading spaces\nnext line\n",
		"\tleading tab\nnext line\n",
		"\t{\n\t\t\"a\": 1\n\t}\n",
		"plain\nlines\n",
		"foo\nbar\n\n",
		"x\n  y\nz\n",
		"\n",
		"\nno trailing break",
	}
	scalar := func(v string, style yaml.Style, comment string) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v, Style: style, LineComment: comment}
	}
	key := func(k string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k} }
	mapping := func(k string, v *yaml.Node) *yaml.Node {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{key(k), v}}
	}
	seq := func(v *yaml.Node) *yaml.Node {
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{v}}
	}
	// Each shape wraps the scalar and says how to find it again in a decoded
	// document node.
	type shape struct {
		wrap func(*yaml.Node) *yaml.Node
		find func(doc *yaml.Node) *yaml.Node
	}
	shapes := map[string]shape{
		"root": {func(n *yaml.Node) *yaml.Node { return n },
			func(d *yaml.Node) *yaml.Node { return d.Content[0] }},
		"mapping value": {func(n *yaml.Node) *yaml.Node { return mapping("k", n) },
			func(d *yaml.Node) *yaml.Node { return d.Content[0].Content[1] }},
		"sequence item": {seq,
			func(d *yaml.Node) *yaml.Node { return d.Content[0].Content[0] }},
		"mapping in a sequence": {func(n *yaml.Node) *yaml.Node { return seq(mapping("actual", n)) },
			func(d *yaml.Node) *yaml.Node { return d.Content[0].Content[0].Content[1] }},
		"sequence in a mapping in a sequence": {func(n *yaml.Node) *yaml.Node { return seq(mapping("body_result", seq(n))) },
			func(d *yaml.Node) *yaml.Node { return d.Content[0].Content[0].Content[1].Content[0] }},
		"sequence in a sequence": {func(n *yaml.Node) *yaml.Node { return seq(seq(n)) },
			func(d *yaml.Node) *yaml.Node { return d.Content[0].Content[0].Content[0] }},
	}
	styles := map[string]yaml.Style{"literal": yaml.LiteralStyle, "folded": yaml.FoldedStyle}
	for _, indent := range []int{2, 3, 4, 5, 7, 9} {
		for sname, sh := range shapes {
			for stname, style := range styles {
				for _, comment := range []string{"", "# c"} {
					for i, v := range values {
						name := fmt.Sprintf("indent %d/%s/%s/comment %q/%d", indent, sname, stname, comment, i)
						t.Run(name, func(t *testing.T) {
							var buf bytes.Buffer
							enc := yaml.NewEncoder(&buf)
							enc.SetIndent(indent)
							if err := enc.Encode(sh.wrap(scalar(v, style, comment))); err != nil {
								t.Fatalf("encode: %v", err)
							}
							if err := enc.Close(); err != nil {
								t.Fatalf("close: %v", err)
							}
							var doc yaml.Node
							if err := yaml.Unmarshal(buf.Bytes(), &doc); err != nil {
								t.Fatalf("the emitted document does not parse: %v\n%s", err, buf.String())
							}
							if got := sh.find(&doc).Value; got != v {
								t.Fatalf("round trip changed the value:\n in: %q\nout: %q\n%s", v, got, buf.String())
							}
						})
					}
				}
			}
		}
	}
}
