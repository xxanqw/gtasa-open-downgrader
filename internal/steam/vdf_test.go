package steam

import "testing"

func TestParseVDFHandlesCommentsEscapesAndConditions(t *testing.T) {
	object, err := parseVDF([]byte(`
// realistic KeyValues syntax
"root"
{
    "quoted" "C:\\SteamLibrary"
    bare value [$LINUX]
    "nested" { "name" "GE-Proton\"Local" }
}
`))
	if err != nil {
		t.Fatal(err)
	}
	root, ok := object.object("ROOT")
	if !ok {
		t.Fatal("case-insensitive root object not parsed")
	}
	if got, _ := root.text("quoted"); got != `C:\SteamLibrary` {
		t.Fatalf("escaped path = %q", got)
	}
	if got, _ := root.text("bare"); got != "value" {
		t.Fatalf("conditional bare value = %q", got)
	}
	nested, _ := root.object("nested")
	if got, _ := nested.text("name"); got != `GE-Proton"Local` {
		t.Fatalf("escaped quote = %q", got)
	}
}

func TestParseVDFRejectsMalformedInput(t *testing.T) {
	for _, input := range []string{`"root" { "key"`, `"root" "unterminated`} {
		if _, err := parseVDF([]byte(input)); err == nil {
			t.Fatalf("malformed VDF accepted: %q", input)
		}
	}
}
