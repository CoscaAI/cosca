package scene

// Construtores de entidades comuns — a matéria-prima do DSL futuro (Visual
// Compiler, PRÓXIMA fase). Aqui ainda é Go puro: cada construtor devolve uma
// entidade com os campos certos (Type, NodeRef, Params).

// NewCamera cria uma câmera na posição pos, com foco lookAt. O lookAt é
// guardado nos Params ("look_at") — o rasterizador do Scene Graph (futuro)
// o consome; a câmera não aponta para nó de computação (NodeRef vazio).
func NewCamera(id string, pos Vec3, lookAt Vec3) *Entity {
	tf := Identity()
	tf.Position = pos
	return &Entity{
		ID:        id,
		Name:      id,
		Type:      Camera,
		Transform: tf,
		Params: map[string]any{
			"look_at": lookAt,
		},
	}
}

// NewLight cria uma luz. lightType: "directional" | "point" | "spot".
// color é RGB no range [0,1].
func NewLight(id string, lightType string, intensity float64, color [3]float64) *Entity {
	return &Entity{
		ID:        id,
		Name:      id,
		Type:      Light,
		Transform: Identity(),
		Params: map[string]any{
			"type":      lightType,
			"intensity": intensity,
			"color":     color,
		},
	}
}

// NewTerrain cria um terreno (Type=Terrain). Aponta NodeRef para o nó
// procedural do terreno (noiseType, ex: "fbm") e carrega nos Params tudo que
// o executor daquele nó consome: width/height (a partir de width/depth),
// height_scale, noise_type, seed e octaves padrão (5).
func NewTerrain(id string, width, depth int, heightScale float64, noiseType string, seed int64) *Entity {
	return &Entity{
		ID:        id,
		Name:      id,
		Type:      Terrain,
		Transform: Identity(),
		NodeRef:   noiseType,
		Params: map[string]any{
			"width":        width,
			"height":       depth,
			"height_scale": heightScale,
			"noise_type":   noiseType,
			"seed":         seed,
			"octaves":      int64(5),
		},
	}
}

// NewProceduralEntity cria uma entidade genérica que aponta para um nó
// procedural (Type=Procedural, NodeRef=nodeType, Params=params). O seed é
// garantido nos Params (não sobrescreve um seed já declarado pelo chamador).
func NewProceduralEntity(id string, nodeType string, seed int64, params map[string]any) *Entity {
	p := make(map[string]any, len(params)+1)
	for k, v := range params {
		p[k] = v
	}
	if _, ok := p["seed"]; !ok {
		p["seed"] = seed
	}
	return &Entity{
		ID:        id,
		Name:      id,
		Type:      Procedural,
		Transform: Identity(),
		NodeRef:   nodeType,
		Params:    p,
	}
}

// NewGroup cria um agrupador (Type=Group) — não gera computação própria; só
// organiza a hierarquia espacial.
func NewGroup(id string) *Entity {
	return &Entity{ID: id, Name: id, Type: Group, Transform: Identity()}
}
