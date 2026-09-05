package procgen

// Família MATH (os 6 do Professor) — funções puras, sem estado, float64.
// Operam sobre valores normalizados [0,1] quando usadas como nós de imagem.

// Add soma dois valores (o chamador decide se clamp).

func Add(a, b float64) float64 { return a + b }

// Multiply multiplica dois valores.

func Multiply(a, b float64) float64 { return a * b }

// Remap re-mapeia v de [inMin,inMax] para [outMin,outMax].
// Fail-closed: faixa de entrada vazia (inMin == inMax) devolve outMin.
func Remap(v, inMin, inMax, outMin, outMax float64) float64 {
	if inMax == inMin {
		return outMin
	}
	t := (v - inMin) / (inMax - inMin)
	return outMin + t*(outMax-outMin)
}

// Clamp restringe v a [min,max].

func Clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// Lerp interpola linearmente a→b com t (sem clamp).

func Lerp(a, b, t float64) float64 { return a + t*(b-a) }

// Curve é o smoothstep clássico t*t*(3-2t) — curva S suave em [0,1].

func Curve(t float64) float64 { return t * t * (3 - 2*t) }

// Mix é o alias de Lerp (mantém a API do Professor).

func Mix(a, b, t float64) float64 { return Lerp(a, b, t) }
