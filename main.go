package main

import (
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pranavbalakrishnan4100/beacon-mcp-server/tools/google/drive"
	"github.com/pranavbalakrishnan4100/beacon-mcp-server/tools/slack"
)

var Serv *server.MCPServer

func init() {
	//fmt.Printf("INIT")
	drive.Authorize()
}

func main() {

	// Setup log file
	// logFile, err := os.OpenFile("beacon.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	// if err != nil {
	// 	log.Fatalf("Failed to open log file: %v", err)
	// }
	log.SetOutput(os.Stderr)

	Serv = server.NewMCPServer(
		"Beacon MCP",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
	)

	addTools()

	err := server.ServeStdio(Serv)
	if err != nil {
		//fmt.Printf("Server error: %v\n", err)
	}

}

func addTools() {
	slackMessagesTool := mcp.NewTool("getMessagesFromSlack",
		mcp.WithDescription("Get all relevant Slack messages regarding the user's query."),
		mcp.WithString("topic",
			mcp.Required(),
			mcp.Description("The specific topic the user is looking to know about, without changing the terminology. "),
		),
	)
	Serv.AddTool(slackMessagesTool, slack.GetMessagesFromSlack)

	googleDriveTool := mcp.NewTool("getFilesFromDrive",
		mcp.WithDescription("Get all files and its details from google drive regarding the topic of uesers query."),
		mcp.WithString("topic",
			mcp.Required(),
			mcp.Description("The specific topic the user is looking to know about, without changing the terminology. "),
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The query entered by the user as it is without any changes."),
		),
	)
	Serv.AddTool(googleDriveTool, drive.GetFilesFromDrive)

	// slackChannelsTool := mcp.NewTool("getChannelsFromSlack",
	// 	mcp.WithDescription("Get the list of slack channels available to the user."))
	// Serv.AddTool(slackChannelsTool, slack.GetMessagesFromSlack)
}
