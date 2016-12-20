package webapi

import (
	"encoding/json"
	"net/url"
	"strconv"
)

type AttachmentField struct {
	Title string `json:"title,omitempty"`
	Value string `json:"value"`
	Short bool   `json:"short,omitempty"`
}

type MessageAttachment struct {
	Fallback   string             `json:"fallback"`
	Color      string             `json:"color,omitempty"`
	Pretext    string             `json:"pretext,omitempty"`
	AuthorName string             `json:"author_name,omitempty"`
	AuthorLink string             `json:"author_link,omitempty"`
	AuthorIcon string             `json:"author_icon,omitempty"`
	Title      string             `json:"title,omitempty"`
	TitleLink  string             `json:"title_link,omitempty"`
	Text       string             `json:"text,omitempty"`
	Fields     []*AttachmentField `json:"fields"`
	ImageURL   string             `json:"image_url,omitempty"`
	ThumbURL   string             `json:"thumb_url,omitempty"`
}

// https://api.slack.com/docs/message-guidelines
type PostMessage struct {
	Channel     string
	Text        string
	Parse       string
	LinkNames   int
	Attachments []*MessageAttachment
	UnfurlLinks bool
	UnfurlMedia bool
	UserName    string
	AsUser      bool
	IconURL     string
	IconEmoji   string
}

func (message *PostMessage) ToURLValues() url.Values {
	values := url.Values{}
	values.Add("channel", message.Channel)
	values.Add("text", message.Text)
	values.Add("parse", message.Parse)
	values.Add("link_names", string(message.LinkNames))
	values.Add("unfurl_links", strconv.FormatBool(message.UnfurlLinks))
	values.Add("unfurl_media", strconv.FormatBool(message.UnfurlMedia))
	values.Add("as_user", strconv.FormatBool(message.AsUser))
	if message.UserName != "" {
		values.Add("user_name", message.UserName)
	}
	if message.IconURL != "" {
		values.Add("icon_url", message.IconURL)
	}
	if message.IconEmoji != "" {
		values.Add("icon_emoji", message.IconEmoji)
	}
	if message.Attachments != nil {
		s, _ := json.Marshal(message.Attachments)
		values.Add("attachments", string(s))
	}

	return values
}

func NewPostMessage(channel string, text string) *PostMessage {
	return &PostMessage{
		Channel:     channel,
		Text:        text,
		LinkNames:   1,
		Parse:       "full",
		UnfurlLinks: true,
		UnfurlMedia: true,
	}
}

func NewPostMessageWithAttachments(channel string, text string, attachments []*MessageAttachment) *PostMessage {
	return &PostMessage{
		Channel:     channel,
		Text:        text,
		LinkNames:   1,
		Attachments: attachments,
		Parse:       "full",
		UnfurlLinks: true,
		UnfurlMedia: true,
	}
}
