package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// NewModelVisionCommand creates `cosca model vision` — the status report for the
// native Go ONNX vision engine's model directory. It lists each canonical
// .onnx model and whether it is present (sovereignty: never a fixed path).
func NewModelVisionCommand() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "vision",
		Short: "Vision ONNX models — status of the native vision engine",
		Long: `Report the presence/absence of the vision .onnx models used by the
native Go ONNX vision engine (no Python, no PyTorch).

The engine resolves the models directory in this order:
  1. $COSCA_MODELS
  2. ~/.cosca/models/vision/
  3. ./.cosca/models/vision

Place the .onnx files there to enable perception. Use --dir to inspect another
directory.`,
		Example: `  cosca model vision
  cosca model vision --dir ~/.cosca/models/vision
  cosca model vision --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			modelsDir := vision.ModelsDirFor(dir)

			type entry struct {
				File string `json:"file"`
				Desc string `json:"desc"`
				OK   bool   `json:"present"`
				Path string `json:"path"`
				Size int64  `json:"size_bytes,omitempty"`
			}
			entries := []entry{
				{vision.ClipModelFile, "CLIP ViT-B/32 (classification + embeddings)", false, "", 0},
				{vision.SAMModelFile, "SAM2 (segmentation)", false, "", 0},
				{vision.GroundingModelFile, "GroundingDINO (text-prompted detection)", false, "", 0},
				{vision.DepthModelFile, "Depth Anything V2 (monocular depth)", false, "", 0},
			}

			present := 0
			for i := range entries {
				e := &entries[i]
				p := filepath.Join(modelsDir, e.File)
				if st, err := os.Stat(p); err == nil && !st.IsDir() {
					e.OK = true
					e.Path = p
					e.Size = st.Size()
					present++
				} else {
					e.Path = p
				}
			}

			if useJSON {
				return printJSON(cmd, map[string]any{
					"models_dir": modelsDir,
					"present":    present,
					"total":      len(entries),
					"models":     entries,
				})
			}

			formatter.Header("Vision ONNX engine")
			formatter.KeyValue("ModelsDir", modelsDir)
			formatter.KeyValue("Status", fmt.Sprintf("%d/%d present", present, len(entries)))
			formatter.Println("")

			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				status := "missing"
				size := ""
				if e.OK {
					status = "present"
					size = formatSize(e.Size)
				}
				rows = append(rows, []string{e.File, status, size})
			}
			formatter.Table([]string{"Model", "Status", "Size"}, rows)

			if present < len(entries) {
				formatter.Warning("Some models are missing — perception degrades gracefully. See internal/worldmodel/vision/MODELS.md for download instructions.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Models directory to inspect (default: resolve from $COSCA_MODELS / ~/.cosca/models/vision)")
	return cmd
}
