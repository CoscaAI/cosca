package local

import (
	"context"
	"testing"
)

func BenchmarkAutopsyLocalEmbed_Query(b *testing.B) {
	p, _ := New(DefaultConfig())
	q := "como funciona a busca vetorial do cosca"
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := p.GenerateEmbedding(context.Background(), q)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAutopsyLocalEmbed_Content(b *testing.B) {
	p, _ := New(DefaultConfig())
	q := "O Cosca kernel orquestra agentes. A memoria usa blockchain de memoria com blocks imutaveis e merkle. A busca hibrida combina FTS5, BM25 e similaridade vetorial para respostas determinísticas."
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := p.GenerateEmbedding(context.Background(), q)
		if err != nil {
			b.Fatal(err)
		}
	}
}
