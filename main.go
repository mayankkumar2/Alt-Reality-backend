package main

import (
	"context"
	"encoding/json"
	"github.com/TeamRekursion/Alt-Reality-backend/participant"
	roomPkg "github.com/TeamRekursion/Alt-Reality-backend/room"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{}
var redisClient *redis.Client

func main() {
	// TODO: move to .env
	redisClient = redis.NewClient(&redis.Options{
		Addr: "0.0.0.0:6379",
	})

	err := redisClient.Ping(context.Background()).Err()
	if err != nil {
		log.Fatalln("error connecting to redis database:", err)
	}

	router := mux.NewRouter()

	router.HandleFunc("/rooms/create", func(w http.ResponseWriter, r *http.Request) {
		if http.MethodPost == r.Method {

			reqBody := struct {
				OfferStr string `json:"offer_str"`
				Name     string `json:"name"`
			}{}

			err := json.NewDecoder(r.Body).Decode(&reqBody)
			if err != nil || reqBody.Name == "" || reqBody.OfferStr == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			room := roomPkg.CreateRoom(reqBody.Name, reqBody.OfferStr)

			key := room.RoomID.String() + ":Room"

			err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   false,
				"message": "created a room",
				"body":    room,
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	router.HandleFunc("/rooms/join", func(w http.ResponseWriter, r *http.Request) {

		if http.MethodPost == r.Method {
			reqBody := struct {
				RoomID uuid.UUID `json:"room_id"`
			}{}

			err := json.NewDecoder(r.Body).Decode(&reqBody)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			key := reqBody.RoomID.String() + ":Room"
			roomCmd := redisClient.Get(context.Background(), key)

			var room roomPkg.Room
			err = roomCmd.Scan(&room)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			p := participant.CreateParticipant()
			room.AddParticipants(p)
			err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   false,
				"message": "joined a room",
				"body": map[string]interface{}{
					"participant": p,
					"room":        room,
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	router.HandleFunc("/rooms/broadcast/receive", func(w http.ResponseWriter, r *http.Request) {
		_, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   false,
			"message": "joined a room",
			"room_id": "ID",
		})
	})
	router.HandleFunc("/rooms/broadcast/send", func(w http.ResponseWriter, r *http.Request) {
		_, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   false,
			"message": "joined a room",
			"room_id": "ID",
		})
	})

	router.HandleFunc("/rooms/info", func(w http.ResponseWriter, r *http.Request) {
		if http.MethodPost == r.Method {
			reqBody := struct {
				RoomID uuid.UUID `json:"room_id"`
			}{}

			err = json.NewDecoder(r.Body).Decode(&reqBody)

			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			key := reqBody.RoomID.String() + ":Room"
			roomCmd := redisClient.Get(context.Background(), key)

			var room roomPkg.Room
			err = roomCmd.Scan(&room)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   false,
				"message": "team info",
				"body": room,
			})


		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	err = http.ListenAndServe("0.0.0.0:8080", router)
	if err != nil {
		log.Fatalln("Error occurred in binding to port 8080")
	}
}
