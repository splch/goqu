package token

import "testing"

func TestTypeString(t *testing.T) {
	tests := []struct {
		typ  Type
		want string
	}{
		{EOF, "EOF"},
		{IDENT, "IDENT"},
		{INT, "INT"},
		{FLOAT, "FLOAT"},
		{LPAREN, "("},
		{RPAREN, ")"},
		{SEMICOLON, ";"},
		{AT, "@"},
		{CTRL, "ctrl"},
		{NEGCTRL, "negctrl"},
		{INV, "inv"},
		{POW, "pow"},
		{OPENQASM, "OPENQASM"},
		{MEASURE, "measure"},
		{PI, "pi"},
		{Type(9999), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.typ.String(); got != tt.want {
			t.Errorf("Type(%d).String() = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

func TestLookupIdent(t *testing.T) {
	tests := []struct {
		ident string
		want  Type
	}{
		{"ctrl", CTRL},
		{"negctrl", NEGCTRL},
		{"inv", INV},
		{"pow", POW},
		{"qubit", QUBIT},
		{"bit", BIT},
		{"measure", MEASURE},
		{"pi", PI},
		{"π", PI},
		{"U", U},
		{"mygate", IDENT},
		{"foobar", IDENT},
	}
	for _, tt := range tests {
		if got := LookupIdent(tt.ident); got != tt.want {
			t.Errorf("LookupIdent(%q) = %v, want %v", tt.ident, got, tt.want)
		}
	}
}

func TestKeywordsConsistency(t *testing.T) {
	// Every keyword should have a corresponding tokenNames entry.
	for kw, typ := range Keywords {
		name := typ.String()
		if name == "UNKNOWN" {
			t.Errorf("keyword %q maps to type %d which has no tokenNames entry", kw, typ)
		}
	}
}

func TestAllTokenTypesNamed(t *testing.T) {
	// Spot-check that the range of defined token types has names.
	for typ := EOF; typ <= STRETCH; typ++ {
		if typ.String() == "UNKNOWN" {
			t.Errorf("Type(%d) has no name in tokenNames", typ)
		}
	}
}
