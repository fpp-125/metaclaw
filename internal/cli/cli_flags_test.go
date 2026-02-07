package cli

import "testing"

func TestStringListFlag(t *testing.T) {
	var f stringListFlag
	if err := f.Set("TAVILY_API_KEY"); err != nil {
		t.Fatalf("set #1: %v", err)
	}
	if err := f.Set("ANOTHER_SECRET"); err != nil {
		t.Fatalf("set #2: %v", err)
	}
	vals := f.Values()
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d", len(vals))
	}
	if vals[0] != "TAVILY_API_KEY" || vals[1] != "ANOTHER_SECRET" {
		t.Fatalf("unexpected values: %+v", vals)
	}
	vals[0] = "MUTATED"
	if f.Values()[0] != "TAVILY_API_KEY" {
		t.Fatal("Values() should return a copy")
	}
}
