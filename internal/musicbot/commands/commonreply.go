package commands

import "github.com/szczursonn/rythm5/internal/musicbot/messages"

var (
	ReplyUnexpectedError = Reply{
		Content: messages.IconAppError + " **An unexpected error has occured**",
	}
)
