package domain

import "strings"

type UserMessage struct {
	Text string
}

func NewUserMessage(text string) UserMessage {
	return UserMessage{Text: strings.TrimSpace(text)}
}

func (m UserMessage) Empty() bool {
	return m.Text == ""
}
