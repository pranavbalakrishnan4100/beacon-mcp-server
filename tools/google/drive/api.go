package drive

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pranavbalakrishnan4100/beacon-mcp-server/tools/anthropic"
)

func GetFilesFromDrive(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.SetOutput(os.Stderr)

	topic := request.Params.Arguments["topic"].(string)
	query := request.Params.Arguments["query"].(string)
	responseText := ""
	driveSrv, err := Authorize()
	if err != nil {
		log.Printf("Failed to authorize: %v", err)
		return mcp.NewToolResultText("Unable to authorize and connect to Google."), nil
	}

	files, err := driveSrv.Files.List().
		Q(fmt.Sprintf("fullText contains '%s' or name contains '%s'", topic, topic)).
		IncludeItemsFromAllDrives(true).
		SupportsAllDrives(true).
		Fields("files(id, name, mimeType, webViewLink)").
		PageSize(10).
		Do()
	if err != nil {
		log.Printf("Unable to retrieve files: %v", err)
		return mcp.NewToolResultText(fmt.Sprintf("Unable to retrieve files: %v", err)), nil
	}

	if len(files.Files) == 0 {
		return mcp.NewToolResultText("No matching files were found in the Google Drive."), nil
	}

	supportedMimeTypes := map[string]bool{
		"application/vnd.google-apps.document": true,
		"text/plain":                           true,
		"application/pdf":                      true,
	}

	type FileSummary struct {
		Name   string
		ID     string
		Link   string
		Answer string
	}

	var summaries []FileSummary
	fileIds := []string{}

	for _, file := range files.Files {

		if !supportedMimeTypes[file.MimeType] {
			log.Printf("Skipping unsupported mime type: %s", file.MimeType)
			continue
		}

		fileIds = append(fileIds, file.MimeType)

		var content []byte
		var readErr error

		if file.MimeType == "application/vnd.google-apps.document" {
			// Export Google Docs as plain text
			resp, err := driveSrv.Files.Export(file.Id, "text/plain").Download()
			if err != nil || resp == nil || resp.Body == nil {
				log.Printf("Failed to export Google Doc: %v", err)
				return mcp.NewToolResultText(fmt.Sprintf("Failed to export Google Doc: %v", err)), nil
				//continue
			}
			content, readErr = io.ReadAll(resp.Body)
			resp.Body.Close()
		} else {
			resp, err := driveSrv.Files.Get(file.Id).Download()
			if err != nil || resp == nil || resp.Body == nil {
				log.Printf("Failed to download file: %v", err)
				return mcp.NewToolResultText(fmt.Sprintf("Failed to download file: %v", err)), nil
				//continue
			}
			content, readErr = io.ReadAll(resp.Body)
			resp.Body.Close()
		}

		if readErr != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Error reading file content: %v", readErr)), nil
			//continue
		}

		answer := ""
		message, err := anthropic.SendMessageToClaude(fmt.Sprintf("File name: %s\nHere is a file's content: ```%s```. Please extract and summarize only what's relevant to the query: %s", file.Name, string(content), query))
		if err != nil {
			log.Printf("Claude error: %v", err)
			answer = "Could not extract answer from Claude."
		} else {
			for _, msg := range message.Content {
				answer += msg.Text
			}
		}

		summaries = append(summaries, FileSummary{
			Name:   file.Name,
			ID:     file.Id,
			Link:   file.WebViewLink,
			Answer: string(answer),
		})

	}

	if len(summaries) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("Files were found but no readable content could be extracted or analyzed.\n\nFile types:\n%v\n\nTotal Files: %d", fileIds, len(fileIds))), nil
	}

	responseText = "Please explain as though you are the source of information, do not have to mention where you obtained teh information from or anything. Imagine yourself as a member of the team with all the knowledge present in you. Now, here are the most relevant file summaries based on your query:\n\n"
	for _, s := range summaries {
		responseText += fmt.Sprintf("📄 *%s* (ID: `%s`)\n🔗 %s\n🧠 %s\n\n", s.Name, s.ID, s.Link, s.Answer)
	}

	return mcp.NewToolResultText(responseText), nil
}
