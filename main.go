package main

import (
	"fmt"
	"time"
	"log"
	"net/http"															
)

type Broker struct {
	Notifier chan []byte
	newClients chan chan []byte
	closingClients chan chan []byte
	clients map[chan []byte]bool
}

func main() {

	broker := NewServer()

	log.Fatal("HTTP server error: ", http.ListenAndServe(":3000", broker))

}

func NewServer() (broker *Broker) {
	// insntantiate a broker
	broker = &Broker{
		Notifier: make(chan []byte, 1),
		newClients: make(chan chan []byte),
		closingClients: make(chan chan []byte),
		clients: make(map[chan []byte]bool),							
	}

	go broker.listen()

	go func() {
		for {
			time.Sleep(time.Second * 2)
			eventString := fmt.Sprintf("the time is %v", time.Now())
			log.Println("Receiving event")
			broker.Notifier <- []byte(eventString)
		}
	}()

	log.Fatal("HTTP server error: ", http.ListenAndServe(":3000", broker))

	return																
}

func (broker *Broker) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
		return															
	}																	

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	rw.Header().Set("Access-Control-Allow-Origin", "*")

	// each connection registers its own message channel with the broker's
	messageChan := make(chan []byte)

	//signal the broker that we have a new connection
	broker.newClients <- messageChan

	// remove client from the map of connected clients when this handler exits
	defer func() {
		broker.closingClients <- messageChan
	}()

	// listen to connection close and unregister messageChan
	notify := rw.(http.CloseNotifier).CloseNotify()

	go func() {
		<-notify														
			broker.closingClients <- messageChan
	}()																	

	// block waiting for message bradcast on this connections messagechan
	for {
		// write to responsewriter
		// server sent events compatible
		fmt.Fprintf(rw, "data: %s\n\n", <- messageChan)

		//flush the data immediatly, no buffering!
		flusher.Flush()
	}
}

func (broker *Broker) listen() {
	for {
		select {
		case s := <-broker.newClients:
			broker.clients[s] = true
			log.Printf("New client added, %d registered clients", len(broker.clients))

		case s := <-broker.closingClients:
			delete(broker.clients, s)
			log.Printf("Client removed. %d registered clients", len(broker.clients))

		case event := <-broker.Notifier:								
			for clientMessageChan, _ := range broker.clients {
				clientMessageChan <- event
			}
		}
	
	}	
}																		
																		
