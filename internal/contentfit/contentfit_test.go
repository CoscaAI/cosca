package contentfit

import "testing"

// TestExtract_RemovesJunk valida que tags de lixo (script/style/nav) são
// removidas e só o conteúdo semântico fica.
func TestExtract_RemovesJunk(t *testing.T) {
	html := `<html><head><style>.x{color:red}</style><script>alert('hi')</script></head>` +
		`<body><nav><a href="/x">Menu</a></nav>` +
		`<main><article><h1>Title</h1><p>Conteudo principal aqui.</p></article></main>` +
		`<footer>footer text</footer></body></html>`

	res := Extract(html)
	if res.CharCount == 0 {
		t.Fatal("esperava conteudo extraido")
	}
	if contains(res.Text, "alert(") || contains(res.Text, "color:red") {
		t.Fatalf("lixo (script/style) nao removido: %q", res.Text)
	}
	if contains(res.Text, "Menu") {
		t.Fatalf("nav deveria ser removido: %q", res.Text)
	}
	if !contains(res.Text, "Conteudo principal aqui") {
		t.Fatalf("conteudo principal deveria estar presente: %q", res.Text)
	}
}

// TestExtract_Minimal verifica o caso de HTML simples.
func TestExtract_Minimal(t *testing.T) {
	res := Extract(`<p>apenas texto</p>`)
	if !contains(res.Text, "apenas texto") {
		t.Fatalf("esperava 'apenas texto', got %q", res.Text)
	}
}

// TestExtract_InvalidHTML não quebra em HTML mal-formado.
func TestExtract_InvalidHTML(t *testing.T) {
	res := Extract(`<p>texto parcial`)
	if res.Text == "" {
		t.Fatal("nao deveria perder tudo em HTML mal-formado")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
