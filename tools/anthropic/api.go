package anthropic

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/slack-go/slack"
)

func SummariseMyMessagesUsingClaude(originalUserQuery string, messages []slack.SearchMessage) string {
	// 1. Limit number of messages to avoid huge prompts
	const maxMessages = 20 // adjust this number based on testing
	if len(messages) > maxMessages {
		messages = messages[:maxMessages]
	}

	// 2. Format messages nicely
	var combinedMessages []string
	for i, msg := range messages {
		cleanText := sanitizeSlackText(msg.Text)
		combinedMessages = append(combinedMessages, fmt.Sprintf("%d. %s", i+1, cleanText))
	}
	combinedMessagesText := strings.Join(combinedMessages, "\n")
	combinedMessagesText = strings.TrimSpace(combinedMessagesText)

	// 3. Structured prompt
	prompt := fmt.Sprintf(`
			You are a helpful team member assisting a newcomer.

			User's original query:
			"%s"

			Slack messages related to the query:
			%s

			Task:
			- Summarize the key information relevant to solving the user's query.
			- Give clear, step-by-step guidance if possible.
			- Cite messages by their number (e.g., "as seen in message 3").
			- Keep it short, focused, and actionable.
			- If information is missing or 2025-04-28T17:30:46.787Z [info] [beacon-mcp-server] Message from server: {"jsonrpc":"2.0","id":20,"error":{"code":-32603,"message":"panic recovered in getMessagesFromSlack tool handler: Claude API error: POST \"https://api.anthropic.com/v1/messages\": 400 Bad Request {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"message\":\"messages: final assistant content cannot end with trailing whitespace\"}}"}}
				not enough, mention that.
			- Do not give additional information, unless specifically requested.
			Begin your answer below:
		`, originalUserQuery, combinedMessagesText)

	prompt = strings.TrimSpace(prompt)
	// 4. Send prompt to Claude
	message, err := SendMessageToClaude(prompt)
	if err != nil {
		panic(fmt.Sprintf("Claude API error: %v", err))
	}

	// 5. Safely extract summarized text
	var summarizedText string
	if message != nil && len(message.Content) > 0 {
		for _, block := range message.Content {
			summarizedText += block.Text + "\n"
		}
	} else {
		summarizedText = "Could not summarize messages properly. Claude did not return a valid response."
	}

	return strings.TrimSpace(summarizedText)
}

func ExtractRelevantTopics(userQuery string) ([]string, error) {
	prompt := fmt.Sprintf(`You are a system designed to extract the most relevant keywords for searching Slack messages based on the user's query. 
Follow the instructions below to extract keywords:
1. Analyze the user's query carefully and extract up to **5 keywords** that best represent the context of the query.
2. Retain **technical terms** (e.g., error codes, specific tool names, domain jargon) **exactly as they appear** in the user's query.
3. Break down longer phrases into **core meaningful keywords**, but ensure any technical term is preserved **as is**.
4. If the query involves an error message or specific tool/process names (e.g., “ERR: NO ACCESS” or “AMI rotation”), keep those **exactly** as they are.
5. The keywords should be **single words**, but if necessary, include multi-word technical terms, ensuring they are still relevant to the query.
6. Return the keywords as a comma-separated list. No explanations, no extra sentences. Only the keywords.

User query: "%s"`, userQuery)

	resp, err := SendMessageToClaude(prompt)
	if err != nil {
		panic(fmt.Sprintf("Claude API error: %v", err))
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("Claude returned empty content")
	}

	responseText := strings.TrimSpace(resp.Content[0].Text)

	// Parse topics from Claude's response
	topics := parseTopics(responseText)

	return topics, nil
}

func parseTopics(response string) []string {
	if response == "" {
		return []string{}
	}
	parts := strings.Split(response, ",")
	var topics []string
	for _, part := range parts {
		cleaned := strings.TrimSpace(part)
		if cleaned != "" {
			topics = append(topics, cleaned)
		}
	}
	return topics
}

func SendMessageToClaude(prompt string) (message *anthropic.Message, err error) {

	client := anthropic.NewClient(
		option.WithAPIKey("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"),
	)

	message, err = client.Messages.New(context.TODO(), anthropic.MessageNewParams{
		MaxTokens: 2048,
		Messages: []anthropic.MessageParam{
			{
				Role:    anthropic.MessageParamRoleAssistant,
				Content: []anthropic.ContentBlockParamUnion{{OfRequestTextBlock: &anthropic.TextBlockParam{Text: prompt}}},
			},
		},
		Model: anthropic.ModelClaude3_7SonnetLatest, // Choose the appropriate Claude model
	})

	return
}

func sanitizeSlackText(input string) string {
	var output []rune
	for _, r := range input {
		if unicode.IsPrint(r) && r != '\uE000' && r != '\uE001' {
			output = append(output, r)
		}
	}
	return strings.TrimSpace(string(output))
}
