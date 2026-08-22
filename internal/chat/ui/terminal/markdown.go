package terminal

func RenderMarkdown(content string, width int) string {
	renderer, err := th.MarkdownRenderer(width)
	if err != nil {
		return content
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}

	return rendered
}
