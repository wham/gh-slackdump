package users

import (
	"testing"

	"github.com/rusq/slack"
	"github.com/rusq/slackdump/v3/types"
)

func TestResolveConversation(t *testing.T) {
	m := HandleMap{
		"U001": "alice",
		"U002": "bob",
		"U003": "charlie",
	}

	conv := &types.Conversation{
		Messages: []types.Message{
			{
				Message: slack.Message{
					Msg: slack.Msg{
						User: "U001",
						Text: "Hello <@U002>, meet <@U003>!",
						Reactions: []slack.ItemReaction{
							{Name: "thumbsup", Users: []string{"U002", "U003"}},
						},
						ReplyUsers:   []string{"U002"},
						ParentUserId: "U001",
						Edited:       &slack.Edited{User: "U001"},
						Inviter:      "U003",
					},
				},
				ThreadReplies: []types.Message{
					{
						Message: slack.Message{
							Msg: slack.Msg{
								User: "U002",
								Text: "Thanks <@U001>",
							},
						},
					},
				},
			},
		},
	}

	ResolveConversation(conv, m)

	msg := conv.Messages[0]
	if msg.User != "alice" {
		t.Errorf("User = %q, want alice", msg.User)
	}
	if msg.Text != "Hello @bob, meet @charlie!" {
		t.Errorf("Text = %q, want resolved mentions", msg.Text)
	}
	if msg.Reactions[0].Users[0] != "bob" || msg.Reactions[0].Users[1] != "charlie" {
		t.Errorf("Reactions.Users = %v, want [bob charlie]", msg.Reactions[0].Users)
	}
	if msg.ReplyUsers[0] != "bob" {
		t.Errorf("ReplyUsers = %v, want [bob]", msg.ReplyUsers)
	}
	if msg.ParentUserId != "alice" {
		t.Errorf("ParentUserId = %q, want alice", msg.ParentUserId)
	}
	if msg.Edited.User != "alice" {
		t.Errorf("Edited.User = %q, want alice", msg.Edited.User)
	}
	if msg.Inviter != "charlie" {
		t.Errorf("Inviter = %q, want charlie", msg.Inviter)
	}

	reply := msg.ThreadReplies[0]
	if reply.User != "bob" {
		t.Errorf("ThreadReply.User = %q, want bob", reply.User)
	}
	if reply.Text != "Thanks @alice" {
		t.Errorf("ThreadReply.Text = %q, want resolved mention", reply.Text)
	}
}

func TestResolveConversationUnknownUser(t *testing.T) {
	m := HandleMap{"U001": "alice"}

	conv := &types.Conversation{
		Messages: []types.Message{
			{
				Message: slack.Message{
					Msg: slack.Msg{
						User: "U999",
						Text: "Hello <@U999>",
					},
				},
			},
		},
	}

	ResolveConversation(conv, m)

	if conv.Messages[0].User != "U999" {
		t.Errorf("Unknown user should keep ID, got %q", conv.Messages[0].User)
	}
	if conv.Messages[0].Text != "Hello <@U999>" {
		t.Errorf("Unknown mention should stay, got %q", conv.Messages[0].Text)
	}
}

func TestResolveConversationSubMessages(t *testing.T) {
	m := HandleMap{"U001": "alice", "U002": "bob"}

	conv := &types.Conversation{
		Messages: []types.Message{
			{
				Message: slack.Message{
					Msg:             slack.Msg{User: "U001"},
					SubMessage:      &slack.Msg{User: "U002"},
					PreviousMessage: &slack.Msg{User: "U001"},
					Root:            &slack.Msg{User: "U002"},
				},
			},
		},
	}

	ResolveConversation(conv, m)

	msg := conv.Messages[0]
	if msg.SubMessage.User != "bob" {
		t.Errorf("SubMessage.User = %q, want bob", msg.SubMessage.User)
	}
	if msg.PreviousMessage.User != "alice" {
		t.Errorf("PreviousMessage.User = %q, want alice", msg.PreviousMessage.User)
	}
	if msg.Root.User != "bob" {
		t.Errorf("Root.User = %q, want bob", msg.Root.User)
	}
}

func TestResolveConversationEmptyFields(t *testing.T) {
	m := HandleMap{"U001": "alice"}

	conv := &types.Conversation{
		Messages: []types.Message{
			{
				Message: slack.Message{
					Msg: slack.Msg{
						User:    "U001",
						Text:    "no mentions here",
						Inviter: "",
					},
				},
			},
		},
	}

	ResolveConversation(conv, m)

	if conv.Messages[0].User != "alice" {
		t.Errorf("User = %q, want alice", conv.Messages[0].User)
	}
	if conv.Messages[0].Inviter != "" {
		t.Errorf("Inviter should remain empty, got %q", conv.Messages[0].Inviter)
	}
}

func TestResolveConversationBlocks(t *testing.T) {
	m := HandleMap{"U001": "alice", "U002": "bob"}

	sectionBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", "Hello <@U001>!", false, false),
		[]*slack.TextBlockObject{
			slack.NewTextBlockObject("mrkdwn", "Field <@U002>", false, false),
		},
		nil,
	)

	conv := &types.Conversation{
		Messages: []types.Message{
			{
				Message: slack.Message{
					Msg: slack.Msg{
						User:   "U001",
						Text:   "Hello <@U001>!",
						Blocks: slack.Blocks{BlockSet: []slack.Block{sectionBlock}},
						Attachments: []slack.Attachment{
							{Text: "att <@U002>", Fallback: "fb <@U001>"},
						},
					},
				},
			},
		},
	}

	ResolveConversation(conv, m)

	sb := conv.Messages[0].Blocks.BlockSet[0].(*slack.SectionBlock)
	if sb.Text.Text != "Hello @alice!" {
		t.Errorf("SectionBlock.Text = %q, want resolved", sb.Text.Text)
	}
	if sb.Fields[0].Text != "Field @bob" {
		t.Errorf("SectionBlock.Fields[0] = %q, want resolved", sb.Fields[0].Text)
	}
	att := conv.Messages[0].Attachments[0]
	if att.Text != "att @bob" {
		t.Errorf("Attachment.Text = %q, want resolved", att.Text)
	}
	if att.Fallback != "fb @alice" {
		t.Errorf("Attachment.Fallback = %q, want resolved", att.Fallback)
	}
}

func TestResolveConversationRichText(t *testing.T) {
	m := HandleMap{"U001": "alice"}

	userEl := slack.NewRichTextSectionUserElement("U001", nil)
	section := slack.NewRichTextSection(userEl)
	richBlock := slack.NewRichTextBlock("blk1", section)

	conv := &types.Conversation{
		Messages: []types.Message{
			{
				Message: slack.Message{
					Msg: slack.Msg{
						User:   "U001",
						Blocks: slack.Blocks{BlockSet: []slack.Block{richBlock}},
					},
				},
			},
		},
	}

	ResolveConversation(conv, m)

	rtb := conv.Messages[0].Blocks.BlockSet[0].(*slack.RichTextBlock)
	rts := rtb.Elements[0].(*slack.RichTextSection)
	ue := rts.Elements[0].(*slack.RichTextSectionUserElement)
	if ue.UserID != "alice" {
		t.Errorf("RichTextSectionUserElement.UserID = %q, want alice", ue.UserID)
	}
}
