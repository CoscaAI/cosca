package domain

import "testing"

func TestPostCreatePayloadValidate(t *testing.T) {
	valid := &PostCreatePayload{
		Content: "olá mundo",
		Platforms: []PostTargetCreatePayload{
			{Platform: "x", AccountID: "account_abc", PlatformSpecificData: map[string]any{"visibility": "public"}},
		},
		MediaURLs: []string{"https://cdn.rizomai.app/media_1"},
	}
	if verr := valid.Validate(); verr != nil {
		t.Fatalf("payload válido deveria passar, erro: %v", verr.Fields)
	}

	cases := []struct {
		name      string
		mutate    func(p *PostCreatePayload)
		wantField string
	}{
		{"content vazio", func(p *PostCreatePayload) { p.Content = "" }, "content"},
		{"content > 4000", func(p *PostCreatePayload) { p.Content = string(make([]byte, 4001)) }, "content"},
		{"sem plataformas", func(p *PostCreatePayload) { p.Platforms = nil }, "platforms"},
		{"> 50 plataformas", func(p *PostCreatePayload) {
			for i := 0; i < 51; i++ {
				p.Platforms = append(p.Platforms, PostTargetCreatePayload{Platform: "x", AccountID: "account_abc"})
			}
		}, "platforms"},
		{"plataforma desconhecida", func(p *PostCreatePayload) { p.Platforms[0].Platform = "whatsapp" }, "platforms[0].platform"},
		{"accountId sem prefixo", func(p *PostCreatePayload) { p.Platforms[0].AccountID = "xyz" }, "platforms[0].accountId"},
		{"timezone inválido", func(p *PostCreatePayload) { p.Timezone = "Mars/Olympus" }, "timezone"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// cópia profunda rasa: plataformas são re-apontadas no mutate
			p := &PostCreatePayload{
				Content:   "olá mundo",
				Platforms: []PostTargetCreatePayload{{Platform: "x", AccountID: "account_abc"}},
			}
			tc.mutate(p)
			verr := p.Validate()
			if verr == nil {
				t.Fatalf("esperava erro no campo %q", tc.wantField)
			}
			if _, ok := verr.Fields[tc.wantField]; !ok {
				t.Fatalf("esperava erro no campo %q, veio: %v", tc.wantField, verr.Fields)
			}
		})
	}
}

func TestProfileCreatePayloadValidate(t *testing.T) {
	if verr := (&ProfileCreatePayload{Name: "Minha Agência"}).Validate(); verr != nil {
		t.Fatalf("nome válido deveria passar: %v", verr.Fields)
	}
	if verr := (&ProfileCreatePayload{Name: ""}).Validate(); verr == nil {
		t.Fatal("nome vazio deveria falhar")
	}
	if verr := (&ProfileCreatePayload{Name: string(make([]byte, 101))}).Validate(); verr == nil {
		t.Fatal("nome > 100 deveria falhar")
	}
}
