package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type WordMessage struct {
	Word string `json:"word"`
}

type WordHolder struct {
	CurrentWord string
	Mu          *sync.Mutex
}

type SSEService struct {
	Server        *http.Server
	Router        *http.ServeMux
	BroadcastChan chan string
	WordHolder    *WordHolder
	ClientsCount  int
}

func NewService() *SSEService {
	return &SSEService{
		Server:        &http.Server{},
		Router:        http.NewServeMux(),
		BroadcastChan: make(chan string, 1),
		WordHolder:    &WordHolder{CurrentWord: "initial", Mu: &sync.Mutex{}},
		ClientsCount:  0,
	}
}

func (s *SSEService) initRoutes() {
	s.Router.HandleFunc("/listen", s.broadcastWord)
	s.Router.HandleFunc("/say", s.updateWord)
	s.Server.Handler = s.Router
}

func Run() {
	sseService := NewService()
	sseService.initRoutes()
	go sseService.fillChannelContinuously()
	sseService.Server.Addr = "localhost:8080"

	log.Fatal(sseService.Server.ListenAndServe())
}

func (s *SSEService) fillChannelContinuously() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		if s.ClientsCount > 0 {
			log.Printf(`sending word "%s" for %d clients`, s.WordHolder.CurrentWord, s.ClientsCount)
			for i := 0; i < s.ClientsCount; i++ {
				s.BroadcastChan <- s.WordHolder.CurrentWord
			}
		}
	}
}

func (s *SSEService) broadcastWord(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)

	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	s.ClientsCount += 1
	defer func() { s.ClientsCount -= 1 }()

	for {
		select {
		case message := <-s.BroadcastChan:
			fmt.Fprintf(w, "data: %s\n\n", message)
			flusher.Flush()

		case <-req.Context().Done():
			return
		}
	}
}

func (s *SSEService) updateWord(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "bad method", http.StatusMethodNotAllowed)
		return
	}

	msg := &WordMessage{}
	err := json.NewDecoder(req.Body).Decode(msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s.WordHolder.CurrentWord != msg.Word {
		s.WordHolder.Mu.Lock()
		s.WordHolder.CurrentWord = msg.Word
		s.WordHolder.Mu.Unlock()
		log.Printf(`word changed to "%s"`, msg.Word)
	}
}
