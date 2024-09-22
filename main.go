package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config struct {
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
}

type OpenAIRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", filepath.Join(os.Getenv("HOME"), ".lolconfig.yaml"), "Path to config file")
	flag.Parse()

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	var input string
	var stdinInput string

	// Check if there's input from stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		stdinBytes, _ := io.ReadAll(os.Stdin)
		stdinInput = string(stdinBytes)
	}

	// Get input from command-line arguments
	argInput := strings.Join(flag.Args(), " ")

	// Combine stdin and argument inputs
	if stdinInput != "" && argInput != "" {
		input = fmt.Sprintf("Question: %s\n\nContext:\n%s", argInput, stdinInput)
	} else if stdinInput != "" {
		input = stdinInput
	} else if argInput != "" {
		input = argInput
	} else {
		fmt.Println("Usage: lol [--config <filepath>] <input>")
		fmt.Println("   or: <command> | lol <question>")
		os.Exit(1)
	}

	response, err := generateLLMResponse(config, input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating response: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(response)
}

func loadConfig(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func generateLLMResponse(config *Config, input string) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"

	requestBody := OpenAIRequest{
		Model: config.Model,
		Messages: []Message{
			{
				Role:    "user",
				Content: input,
			},
		},
		MaxTokens: config.MaxTokens,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status code %d: %s", resp.StatusCode, string(body))
	}

	var openAIResp OpenAIResponse
	err = json.Unmarshal(body, &openAIResp)
	if err != nil {
		return "", err
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return openAIResp.Choices[0].Message.Content, nil
}
