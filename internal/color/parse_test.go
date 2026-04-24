package color

import "testing"

func TestParse_Hex(t *testing.T) {
	tests := []struct {
		in   string
		want Color
	}{
		{"#ff0000", Color{255, 0, 0}},
		{"ff0000", Color{255, 0, 0}},
		{"#FF0000", Color{255, 0, 0}},
		{"#f00", Color{255, 0, 0}},
		{"f00", Color{255, 0, 0}},
		{"#5a3b8c", Color{90, 59, 140}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil { t.Fatalf("Parse(%q) error: %v", tt.in, err) }
		if got != tt.want { t.Errorf("Parse(%q) = %v, want %v", tt.in, got, tt.want) }
	}
}

func TestParse_Named(t *testing.T) {
	tests := []struct { in string; want Color }{
		{"red", Color{255, 0, 0}},
		{"Red", Color{255, 0, 0}},
		{"white", Color{255, 255, 255}},
		{"black", Color{0, 0, 0}},
		{"papayawhip", Color{255, 239, 213}},
		{"rebeccapurple", Color{102, 51, 153}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil { t.Fatalf("Parse(%q) error: %v", tt.in, err) }
		if got != tt.want { t.Errorf("Parse(%q) = %v, want %v", tt.in, got, tt.want) }
	}
}

func TestParse_Random(t *testing.T) {
	c, err := Parse("random")
	if err != nil { t.Fatalf("Parse(\"random\") error: %v", err) }
	_ = c
}

func TestParse_Invalid(t *testing.T) {
	inputs := []string{"", "not-a-color", "#xyz", "#1234", "#12345", "rgb(1,2,3)"}
	for _, in := range inputs {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = nil err, want error", in)
		}
	}
}
