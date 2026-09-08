package mdparse

import (
	"reflect"
	"testing"
)

func TestParseLinksAndAnchors(t *testing.T) {
	source := []byte("# Hello, 世界!\n\n# Hello, 世界!\n\n# Hello, 世界!-1\n\n# Hello, 世界!\n\n" +
		"[inline](docs/a%20b.md#hello-%E4%B8%96%E7%95%8C \"title\")\n" +
		"![image](assets/pic.png)\n\n" +
		"[reference]: <other.md#part> \"title\"\n" +
		"[multiline]:\n  multiline.md\n\n" +
		"> [quoted]: quoted.md\n\n" +
		"- [listed]: listed.md\n\n" +
		"`[code](missing.md)`\n" +
		"```md\n[fenced](missing.md)\n```\n" +
		"<div>\n[html block](missing.md)\n</div>\n")

	doc := Parse(source)
	var destinations []string
	for _, link := range doc.Links {
		destinations = append(destinations, link.Destination)
	}
	want := []string{"docs/a%20b.md#hello-%E4%B8%96%E7%95%8C", "assets/pic.png", "other.md#part", "multiline.md", "quoted.md", "listed.md"}
	if !reflect.DeepEqual(destinations, want) {
		t.Fatalf("destinations = %#v, want %#v", destinations, want)
	}
	for _, anchor := range []string{"hello-世界", "hello-世界-1", "hello-世界-1-1", "hello-世界-2"} {
		if _, ok := doc.Anchors[anchor]; !ok {
			t.Errorf("missing anchor %q in %#v", anchor, doc.Anchors)
		}
	}
}

func TestParseLineStartInlineAndEmptyDestination(t *testing.T) {
	doc := Parse([]byte("[line start](target.md)\n[self]()\n[multi\nline](multi.md)\n[invalid](bad.md \"title\" trailing)\n[![nested](image.png)](page.md)\n[entity](a&amp;b.md)\n"))
	if len(doc.Links) != 6 {
		t.Fatalf("got %d links, want 6: %#v", len(doc.Links), doc.Links)
	}
	got := []string{doc.Links[0].Destination, doc.Links[1].Destination, doc.Links[2].Destination, doc.Links[3].Destination, doc.Links[4].Destination, doc.Links[5].Destination}
	want := []string{"target.md", "", "multi.md", "image.png", "page.md", "a&amp;b.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected links: %#v", doc.Links)
	}
}

func TestParseUsesCommonMarkSemantics(t *testing.T) {
	source := []byte("[invalid](missing.md \"title\" extra)\n\n```bad` [real](target.md)\n")
	doc := Parse(source)
	if len(doc.Links) != 1 || doc.Links[0].Destination != "target.md" {
		t.Fatalf("unexpected semantic links: %#v", doc.Links)
	}
}

func TestApplyPreservesEverythingOutsideDestination(t *testing.T) {
	source := []byte("before [label](old.md \"title\") after\r\n")
	doc := Parse(source)
	if len(doc.Links) != 1 {
		t.Fatalf("got %d links", len(doc.Links))
	}
	result := Apply(source, []Edit{{Start: doc.Links[0].Start, End: doc.Links[0].End, Text: "new/path.md"}})
	want := "before [label](new/path.md \"title\") after\r\n"
	if string(result) != want {
		t.Fatalf("result = %q, want %q", result, want)
	}
}

func TestDestinationHelpers(t *testing.T) {
	if !IsLocal("../a%20b.md?raw=1#part") || IsLocal("https://example.com/a.md") || IsLocal("/site/path") {
		t.Fatal("unexpected local URL classification")
	}
	path, err := DecodePath(`a\(b\).md#part`)
	if err != nil || path != "a(b).md" {
		t.Fatalf("DecodePath = %q, %v", path, err)
	}
	if got := EncodePath("../a b.md"); got != "../a%20b.md" {
		t.Fatalf("EncodePath = %q", got)
	}
	if fragment, ok := Fragment("a.md#part%20one"); !ok || fragment != "part one" {
		t.Fatalf("Fragment = %q, %v", fragment, ok)
	}
	if decoded := DecodeDestination("a&amp;b.md"); decoded != "a&b.md" {
		t.Fatalf("DecodeDestination = %q", decoded)
	}
}

func TestGitHubStyleSlugCharacters(t *testing.T) {
	doc := Parse([]byte("# 😄 emoji\n\n# non\u00a0breaking\n\n# hi <em>there</em>\n\n# `a  b`\n\n# a_ ‿ ⁀b\n\n# &#x20;a\n\n# a&#x20;\n\nfoo\nbar\n---\n"))
	for _, anchor := range []string{"-emoji", "nonbreaking", "hi-there", "a--b", "a_-‿-⁀b", "-a", "a-", "foobar"} {
		if _, ok := doc.Anchors[anchor]; !ok {
			t.Errorf("missing anchor %q in %#v", anchor, doc.Anchors)
		}
	}
}

func FuzzParseReturnsValidSourceSpans(f *testing.F) {
	for _, seed := range []string{
		"[link](target.md)",
		"[ref]: <a b.md>\n",
		"```md\n[x](ignored.md)\n```\n",
		"[![nested](image.png)](page.md)",
		"# 日本語 heading\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		source := []byte(input)
		doc := Parse(source)
		previousEnd := 0
		for _, link := range doc.Links {
			if link.Start < 0 || link.Start > link.End || link.End > len(source) {
				t.Fatalf("invalid span [%d:%d] for %d bytes", link.Start, link.End, len(source))
			}
			if link.Start < previousEnd {
				t.Fatalf("overlapping or unsorted span at %d after %d", link.Start, previousEnd)
			}
			if string(source[link.Start:link.End]) != link.Destination {
				t.Fatalf("span contents %q != destination %q", source[link.Start:link.End], link.Destination)
			}
			previousEnd = link.End
		}
	})
}
