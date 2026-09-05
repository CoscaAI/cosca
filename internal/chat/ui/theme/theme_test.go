package theme

import "testing"

func TestPetrolPaletteIsSolidAndContrasting(t *testing.T) {
	if Petrol.Background == "" {
		t.Fatal("Petrol background must be solid")
	}
	if Petrol.Background != "#092F33" {
		t.Fatalf("Petrol background = %q, want #092F33", Petrol.Background)
	}
	if Petrol.Surface != "#05090A" {
		t.Fatalf("Petrol surface = %q, want #05090A", Petrol.Surface)
	}
	if Petrol.Foreground != "#F4F7F7" {
		t.Fatalf("Petrol foreground = %q, want #F4F7F7", Petrol.Foreground)
	}
	if Petrol.InputBackground != "#4A5557" || Petrol.InputFocusedBackground != "#596568" {
		t.Fatalf("Petrol input surfaces = %q/%q, want #4A5557/#596568", Petrol.InputBackground, Petrol.InputFocusedBackground)
	}
	if got := Petrol.App().GetBackground(); got != Petrol.Background {
		t.Fatalf("Petrol App background = %v, want %v", got, Petrol.Background)
	}
	if got := Petrol.InputBox().GetBackground(); got != Petrol.InputBackground {
		t.Fatalf("Petrol input background = %v, want %v", got, Petrol.InputBackground)
	}
}
