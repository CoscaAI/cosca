package gitmgr

import "encoding/json"

// jsonMarshalIndent é um wrapper de json.MarshalIndent (para manter os imports
// do pacote limpos e permitir sobrescrita em teste se necessário).
func jsonMarshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
