package messages

import (
	"fmt"
	"strings"
)

var escapeMarkdownReplacer = strings.NewReplacer(
	`\`, `\\`,
	`*`, `\*`,
	`_`, `\_`,
	`~`, `\~`,
	"`", "\\`",
	`|`, `\|`,
	`>`, `\>`,
)

func EscapeMarkdown(str string) string {
	return escapeMarkdownReplacer.Replace(str)
}

func MakeMarkdownLink(title string, link string) string {
	if link == "" {
		return EscapeMarkdown(title)
	}

	return fmt.Sprintf("[%s](%s)", EscapeMarkdown(title), link)
}
