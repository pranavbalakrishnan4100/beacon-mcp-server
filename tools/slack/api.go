package slack

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/slack-go/slack"
)

const slackUserToken = "xoxp-8811934186997-8811934228885-8826515839153-85a05274ce235be2d04fca4ba6e2f1e9"

func GetMessagesFromSlack(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	topic := request.Params.Arguments["topic"].(string)
	// topics, _ := anthropic.ExtractRelevantTopics(query)

	api := slack.New(slackUserToken)

	params := slack.SearchParameters{
		Sort:          "score", // or "score"
		SortDirection: "desc",  // newest first
		Highlight:     true,    // optional: highlight query matches
		Count:         20,      // how many results to fetch at a time
	}

	var resultMatches []slack.SearchMessage
	result, err := api.SearchMessages(topic, params)
	if err != nil {
		// Log or handle the error
	} else {
		resultMatches = append(resultMatches, result.Matches...)
	}

	messages := removeDuplicateMessages(resultMatches)

	responseText := ""
	if len(messages) == 0 {
		responseText = fmt.Sprintf("No information was found for the topic from Slack. Ask the user if they would like generic information instead. If they agree, proceed accordingly.")
	} else {
		responseText = fmt.Sprintf("Summarize ONLY the below messages. Do NOT add any additional information unless specifically requested. Summarize it as though you are the one saying it, you dont have to mention where this was obtained from. Make sure to say it in a detailed explanatory manner and bold the important parts. Messages: %+v", messages)

	}

	return mcp.NewToolResultText(responseText), nil

}

func rankByRelevance(query string, results []slack.SearchMessage) []slack.SearchMessage {
	//Need to add more advanced logic here, like NLP or semantic analysis
	return results
}

func GetChannelsFromSlack(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	channelsParams := slack.GetConversationsParameters{}
	api := slack.New(slackUserToken)
	resultConversations, cursor, _ := api.GetConversations(&channelsParams)
	channels := []string{}
	for _, channel := range resultConversations {
		channels = append(channels, channel.Name)
	}
	return mcp.NewToolResultText("Cursor:" + cursor + ". Total channels found on slack:" + strconv.Itoa(len(resultConversations)) + "\n Channels found:\n" + fmt.Sprintf("%+v", channels)), nil

}

func removeDuplicateMessages(messages []slack.SearchMessage) []string {
	seen := make(map[string]bool)
	var unique []string
	for _, msg := range messages {
		conversationMessages, _ := GetFullConversationForMatch(msg)

		for _, convoMsg := range conversationMessages {
			if !seen[convoMsg.Permalink] {
				seen[convoMsg.Permalink] = true
				unique = append(unique, convoMsg.Text)
			}
		}

	}
	return unique
}

// GetFullConversationForMatch fetches full conversation context for a slack search match.
// - If the message is standalone, it returns it as is.
// - If the message is part of a thread (or a thread parent), it fetches all thread messages.
func GetFullConversationForMatch(match slack.SearchMessage) ([]slack.Message, error) {
	channelID := match.Channel.ID
	messageTs := match.Timestamp
	api := slack.New(slackUserToken)
	// Step 1: Fetch the exact message from channel history
	fullMessage, err := fetchMessageByTimestamp(api, channelID, messageTs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch full message: %w", err)
	}

	// Step 2: Decide based on ThreadTimestamp
	if fullMessage.ThreadTimestamp != "" && fullMessage.ThreadTimestamp != fullMessage.Timestamp {
		// This message is a reply in a thread, fetch full thread
		return fetchThreadMessages(api, channelID, fullMessage.ThreadTimestamp)
	} else if fullMessage.ThreadTimestamp != "" && fullMessage.ThreadTimestamp == fullMessage.Timestamp {
		// This message is the thread starter (root)
		return fetchThreadMessages(api, channelID, fullMessage.ThreadTimestamp)
	} else {
		// No thread, just return this one message
		return []slack.Message{*fullMessage}, nil
	}
}

// fetchMessageByTimestamp fetches a single message from a channel at a specific timestamp.
func fetchMessageByTimestamp(api *slack.Client, channelID, timestamp string) (*slack.Message, error) {
	historyParams := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Inclusive: true,
		Latest:    timestamp,
		Limit:     1,
	}

	history, err := api.GetConversationHistory(historyParams)
	if err != nil {
		return nil, fmt.Errorf("error fetching conversation history: %w", err)
	}
	if len(history.Messages) == 0 {
		return nil, fmt.Errorf("no message found at timestamp %s", timestamp)
	}

	return &history.Messages[0], nil
}

// fetchThreadMessages fetches all messages in a thread given the thread timestamp.
func fetchThreadMessages(api *slack.Client, channelID, threadTimestamp string) ([]slack.Message, error) {
	repliesParams := &slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTimestamp,
		Limit:     100, // adjust if needed
	}

	messages, _, _, err := api.GetConversationReplies(repliesParams)
	if err != nil {
		return nil, fmt.Errorf("error fetching conversation replies: %w", err)
	}

	return messages, nil
}

func generateTopicsFromQuery(query string) []string {
	// For now just return the query itself
	return []string{query}
}
