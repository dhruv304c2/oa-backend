package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Gemini Agent (type 'exit' to quit)")

	history := []*genai.Content{}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\nYou: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if strings.ToLower(input) == "exit" {
			break
		}

		history = append(history, genai.NewContentFromText(input, genai.RoleUser))

		resp, err := client.Models.GenerateContent(
			ctx,
			"gemini-2.5-flash",
			history,
			nil,
		)
		if err != nil {
			log.Fatal(err)
		}

		reply := resp.Text()
		fmt.Println("Agent:", reply)

		history = append(history, genai.NewContentFromText(reply, genai.RoleModel))
	}
}
