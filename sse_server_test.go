package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func startListener(c *http.Client, url string, wg *sync.WaitGroup) {
	res, err := c.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	log.Println(string(data))
	wg.Done()
}

func startTeller(c *http.Client, url string, word string, sleepTime time.Duration) {
	for {
		wordStruct := WordMessage{Word: word}
		wordJson, err := json.Marshal(&wordStruct)
		if err != nil {
			log.Fatal(err)
		}
		c.Post(url, "application/json", bytes.NewBuffer(wordJson))
		time.Sleep(sleepTime)
	}

}
func TestSingle(t *testing.T) {

	service := NewService()
	service.initRoutes()
	go service.fillChannelContinuously()

	server := httptest.NewServer(service.Router)
	defer server.Close()

	listenerClient := &http.Client{
		Timeout: time.Second * 10,
	}

	go startTeller(&http.Client{}, server.URL+"/say", "testmsg", time.Duration(time.Millisecond*200))

	listenerWg := &sync.WaitGroup{}
	listenerWg.Add(1)
	go startListener(listenerClient, server.URL+"/listen", listenerWg)
	listenerWg.Wait()

}

func TestMultiple(t *testing.T) {
	service := NewService()
	service.initRoutes()
	go service.fillChannelContinuously()

	server := httptest.NewServer(service.Router)
	defer server.Close()

	testListeners := []*http.Client{}
	testTellers := []*http.Client{}
	listenerWg := &sync.WaitGroup{}

	for i := 0; i < 10; i++ {
		testListeners = append(testListeners, &http.Client{Timeout: time.Second * 10})
		testTellers = append(testTellers, &http.Client{})
	}

	for i := 0; i < 10; i++ {
		listenerWg.Add(1)
		go startListener(testListeners[i], server.URL+"/listen", listenerWg)
		randomTime := rand.Intn(500) + 100
		go startTeller(testTellers[i], server.URL+"/say", fmt.Sprintf("testmsg%d", i), time.Millisecond*time.Duration(randomTime))

	}
	listenerWg.Wait()
}
