package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

type ChatRequest struct {
	Content    string  `json:"content"`
	MaxToken   int64   `json:"max_token"`
	Temparture float64 `json:"tempature"`
}

func init() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}
}

func chatHandler(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.MaxToken == 0 {
		req.MaxToken = 20
	}
	if req.Temparture == 0 {
		req.Temparture = 0.8
	}

	// Create a new OpenAI client
	env := os.Getenv("DEVELOPMENT")
	fmt.Print(env)
	if env == "true" {
		paragraph := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Vestibulum volutpat magna quis dui ultrices, sed ornare diam aliquet. Sed sem nisi, molestie vitae pretium dictum, pretium vel felis. Vestibulum ultricies semper erat tempor maximus. Aliquam eu cursus velit, eu blandit enim. Suspendisse quis convallis ligula. Donec eros justo, lobortis ut ullamcorper vel, mattis quis libero."

		words := []string{}
		for _, word := range strings.Split(paragraph, " ") {
			words = append(words, word)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.(http.Flusher).Flush()

		// Send a message every second
		for i := 0; i < len(words); i++ {
			time.Sleep(time.Second / 10)
			fmt.Fprintf(w, "%s ", words[i])
			w.(http.Flusher).Flush()
		}
	} else {
		apiKey := os.Getenv("OPENAI_SERCET_KEY")
		if apiKey == "" {
			http.Error(w, "Missing OpenAI API key", http.StatusInternalServerError)
			return
		}
		c := openai.NewClient(apiKey)

		reqBody := openai.ChatCompletionRequest{
			Model:       openai.GPT3Dot5Turbo,
			MaxTokens:   int(req.MaxToken),
			Temperature: float32(req.Temparture),
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: req.Content,
				},
			},
			Stream: true,
		}

		ctx := context.Background()

		// Create the chat completion stream
		stream, err := c.CreateChatCompletionStream(ctx, reqBody)
		if err != nil {
			http.Error(w, "Error creating chat completion stream", http.StatusInternalServerError)
			return
		}
		defer stream.Close()

		// Set the response header
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// w.WriteHeader(http.StatusOK)

		// Stream the responses directly to the client
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				http.Error(w, "Error receiving stream response", http.StatusInternalServerError)
				return
			}
			eventData := fmt.Sprintf(response.Choices[0].Delta.Content)
			fmt.Println(eventData)
			_, err = w.Write([]byte(eventData))
			if err != nil {
				http.Error(w, "Error writing server-sent event", http.StatusInternalServerError)
			}
			w.(http.Flusher).Flush()
		}
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	// r.Use(httprate.LimitByIP(1, 1*time.Minute))
	r.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"https://*", "http://*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	r.Get("/chat", chatHandler)
	http.ListenAndServe(":3450", r)
	if err != nil {
		fmt.Printf("Error starting server")
	}
}
