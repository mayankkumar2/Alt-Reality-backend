package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{}



func main() {


	router := mux.NewRouter()

	router.HandleFunc("/rooms/create", func(w http.ResponseWriter, r *http.Request) {



		json.NewEncoder(w).Encode(map[string] interface{}{
			"error": false,
			"message": "created a room",
			"room_id": "ID",
		})
	})
	router.HandleFunc("/rooms/join", func(w http.ResponseWriter, r *http.Request) {



		json.NewEncoder(w).Encode(map[string] interface{}{
			"error": false,
			"message": "joined a room",
			"room_id": "ID",
		})
	})

	router.HandleFunc("/rooms/broadcast", func(w http.ResponseWriter, r *http.Request) {
		_, err :=upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		json.NewEncoder(w).Encode(map[string] interface{}{
			"error": false,
			"message": "joined a room",
			"room_id": "ID",
		})
	})

	router.HandleFunc("/rooms/info/:id", func(w http.ResponseWriter, r *http.Request) {

	})

	err := http.ListenAndServe("0.0.0.0:8080", router)
	if err != nil {
		log.Fatalln("Error occurred in binding to port 8080")
	}
}