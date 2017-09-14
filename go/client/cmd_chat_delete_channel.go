package client

import (
	"context"
	"fmt"

	"github.com/keybase/cli"
	"github.com/keybase/client/go/chat/utils"
	"github.com/keybase/client/go/libcmdline"
	"github.com/keybase/client/go/libkb"
	"github.com/keybase/client/go/protocol/chat1"
	"github.com/keybase/client/go/protocol/keybase1"
)

type CmdChatDeleteChannel struct {
	libkb.Contextified

	resolvingRequest chatConversationResolvingRequest
}

func NewCmdChatDeleteChannelRunner(g *libkb.GlobalContext) *CmdChatDeleteChannel {
	return &CmdChatDeleteChannel{
		Contextified: libkb.NewContextified(g),
	}
}

func newCmdChatDeleteChannel(cl *libcmdline.CommandLine, g *libkb.GlobalContext) cli.Command {
	return cli.Command{
		Name:         "delete-channel",
		Usage:        "Delete a conversation channel",
		ArgumentHelp: "[conversation [channel name]]",
		Action: func(c *cli.Context) {
			cl.ChooseCommand(NewCmdChatDeleteChannelRunner(g), "delete-channel", c)
		},
		Flags: mustGetChatFlags("topic-type"),
	}
}

func (c *CmdChatDeleteChannel) Run() error {
	chatClient, err := GetChatLocalClient(c.G())
	if err != nil {
		return err
	}

	ctx := context.Background()
	resolver := &chatConversationResolver{G: c.G(), ChatClient: chatClient}
	conv, _, err := resolver.Resolve(ctx, c.resolvingRequest, chatConversationResolvingBehavior{
		CreateIfNotExists: false,
		MustNotExist:      false,
		Interactive:       false,
		IdentifyBehavior:  keybase1.TLFIdentifyBehavior_CHAT_CLI,
	})
	if err != nil {
		return err
	}

	_, err = chatClient.DeleteConversationLocal(ctx, conv.GetConvID())
	if err != nil {
		return err
	}

	return nil
}

func (c *CmdChatDeleteChannel) ParseArgv(ctx *cli.Context) (err error) {
	if len(ctx.Args()) != 2 {
		cli.ShowCommandHelp(ctx, "delete-channel")
		return fmt.Errorf("Incorrect usage.")
	}
	teamName := ctx.Args().Get(0)
	topicName := ctx.Args().Get(1)

	if c.resolvingRequest, err = parseConversationResolvingRequest(ctx, teamName); err != nil {
		return err
	}

	// Force team for now
	c.resolvingRequest.MembersType = chat1.ConversationMembersType_TEAM
	c.resolvingRequest.Visibility = keybase1.TLFVisibility_PRIVATE
	c.resolvingRequest.TopicName = utils.SanitizeTopicName(topicName)

	return nil
}

func (c *CmdChatDeleteChannel) GetUsage() libkb.Usage {
	return libkb.Usage{
		Config: true,
		API:    true,
	}
}
