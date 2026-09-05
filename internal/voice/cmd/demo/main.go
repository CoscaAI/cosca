package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/CoscaAI/cosca/internal/voice"
)

func main() {
	texto := "O Cosca protege o conhecimento acima de tudo, chef."
	if len(os.Args) > 1 {
		texto = strings.Join(os.Args[1:], " ")
	}
	b, err := voice.AbrirBanco("/home/cosca/.cosca/voice/bank/diphones")
	if err != nil {
		fmt.Println("ERRO banco:", err)
		os.Exit(1)
	}
	fonemas := voice.G2P(texto)
	sampa := make([]string, 0, len(fonemas))
	for _, f := range fonemas {
		sampa = append(sampa, f.SAMPA())
	}
	fmt.Println("fonemas:", strings.Join(sampa, " "))
	prev := ""
	faltando := 0
	for _, f := range sampa {
		if f == "'" || f == " " {
			continue
		}
		if prev != "" && prev != " " {
			if !b.Tem(prev, f) {
				faltando++
			}
		}
		prev = f
	}
	fmt.Printf("banco: %d difonos, %d faltando\n", b.Cobertura(), faltando)
	smp, err := b.Sintetizar(texto, voice.StyleCoscaExecutive)
	if err != nil {
		fmt.Println("ERRO síntese:", err)
		os.Exit(1)
	}
	out := "/tmp/opencode/voz_cosca.wav"
	if err := voice.EscreverWAV16(out, smp); err != nil {
		fmt.Println("ERRO wav:", err)
		os.Exit(1)
	}
	fmt.Printf("OK: %d amostras = %.2fs → %s\n", len(smp), float64(len(smp))/24000, out)
}