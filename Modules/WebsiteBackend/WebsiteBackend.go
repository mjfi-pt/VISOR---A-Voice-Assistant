/*******************************************************************************
 * Copyright 2023-2025 The V.I.S.O.R. authors
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 ******************************************************************************/

package WebsiteBackend

import (
	"Utils"
	"Utils/ModsFileInfo"
	"context"
	Tcef "github.com/Edw590/TryCatch-go"
	"github.com/gorilla/websocket"
	"github.com/yousifnimah/Cryptx/CRC16"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Website Backend //

const MAX_CLIENTS int = 100

const PONG_WAIT = 120 * time.Second // Allow X time before considering the client unreachable.
const PING_PERIOD = 60 * time.Second // Must be less than PONG_WAIT. -->

var channels_GL [MAX_CLIENTS]chan []byte = [MAX_CLIENTS]chan []byte{}
var used_channels_GL [MAX_CLIENTS]bool = [MAX_CLIENTS]bool{}

var modDirsInfo_GL Utils.ModDirsInfo
func Start(module *Utils.Module) {Utils.ModStartup(main, module)}
func main(module_stop *bool, moduleInfo_any any) {
	modDirsInfo_GL = moduleInfo_any.(Utils.ModDirsInfo)

	go func() {
		for {
			var comms_map map[string]any = Utils.GetFromCommsChannel(true, Utils.NUM_MOD_WebsiteBackend, 0)
			if comms_map == nil {
				return
			}
			map_value, ok := comms_map["Message"]
			if !ok {
				continue
			}

			var message []byte = map_value.([]byte)
			for i := 0; i < MAX_CLIENTS; i++ {
				if used_channels_GL[i] {
					channels_GL[i] <- message
				}
			}
		}
	}()

	var srv *http.Server = nil
	go func() {
		Tcef.Tcef{
			Try: func() {
				// Try to register. If it's already registered, ignore the panic.
				http.HandleFunc("/ws", basicAuth(webSocketsHandler))
			},
		}.Do()

		//log.Println("Server running on port 3234")
		srv = &http.Server{Addr: ":3234"}
		err := srv.ListenAndServeTLS(getModUserInfo().Crt_file, getModUserInfo().Key_file)
		if err != nil {
			log.Println("ListenAndServeTLS error:", err)

			*module_stop = true
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	for {
		if Utils.WaitWithStopDATETIME(module_stop, 1000000000) {
			_ = srv.Shutdown(ctx)

			for i := 0; i < MAX_CLIENTS; i++ {
				unregisterChannel(i)
			}

			return
		}
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func webSocketsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("WebSocketsHandler called")

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)

		return
	}
	defer conn.Close()

	var device_id string = ""

	_ = conn.SetReadDeadline(time.Now().Add(PONG_WAIT))
	conn.SetPongHandler(func(appData string) error {
		_ = conn.SetReadDeadline(time.Now().Add(PONG_WAIT))

		return nil
	})

	ticker := time.NewTicker(PING_PERIOD)
	defer ticker.Stop()

	var channel_num int = registerChannel()
	if channel_num == -1 {
		log.Println("No available channels")

		return
	}

	var mutex sync.Mutex

	sendData := func(message_type int, bytes []byte) error {
		mutex.Lock()
		defer mutex.Unlock()

		if err = conn.WriteMessage(message_type, bytes); err != nil {
			return err
		}

		return err
	}

	go func() {
		for {
			select {
				case <- ticker.C:
					if sendData(websocket.PingMessage, nil) != nil {
						log.Println("Ping error:", err)

						return
					}
				case message := <- channels_GL[channel_num]:
					if message == nil {
						return
					}

					var msg_device_id string = strings.Split(string(message), "|")[0]
					if msg_device_id != device_id && msg_device_id != "3234_ALL" {
						continue
					}

					var index_bar int = strings.Index(string(message), "|")
					var truncated_msg []byte = message[index_bar + 1:]

					if err = sendData(websocket.BinaryMessage, truncated_msg); err == nil {
						log.Printf("Message sent 2. Length: %d; CRC16: %d; Content: %s", len(truncated_msg),
							CRC16.Result(truncated_msg, "CCIT_ZERO"), truncated_msg[:strings.Index(string(truncated_msg), "|")])
					} else {
						log.Println("Write error:", err)

						return
					}
			}
		}
	}()

	var first_message bool = true
	for {
		// Read message from WebSocket
		message_type, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)

			break
		}

		if message_type != websocket.BinaryMessage {
			continue
		}
		if first_message {
			first_message = false

			device_id = string(message)

			log.Println("Device ID connected:", device_id)

			continue
		}

		var message_str string = string(message)
		var index_bar int = strings.Index(message_str, "|")
		var index_2nd_bar int = strings.Index(message_str[index_bar + 1:], "|")
		if index_bar == -1 || index_2nd_bar == -1 {
			continue
		}

		// Print received message
		log.Printf("Received: %s", message[:index_bar + index_2nd_bar + 2])

		var message_parts []string = strings.Split(message_str, "|")
		if len(message_parts) < 3 {
			continue
		}
		var msg_to string = message_parts[0]
		var type_ string = message_parts[1]
		var bytes []byte = message[index_bar + index_2nd_bar + 2:]

		var partial_resp []byte = handleMessage(device_id, type_, bytes)
		if msg_to == "N" {
			log.Println("Returning no response")
		} else {
			var response []byte = []byte(msg_to + "|")
			response = append(response, partial_resp...)

			if err = sendData(websocket.BinaryMessage, response); err == nil {
				log.Printf("Message sent 1. Length: %d; CRC16: %d; Content: %s", len(response),
					CRC16.Result(response, "CCIT_ZERO"), response[:strings.Index(string(response), "|")])
			} else {
				log.Println("Write error:", err)

				break
			}
		}
	}

	unregisterChannel(channel_num)

	log.Println("WebSocketsHandler ended")
}

func handleMessage(device_id string, type_ string, bytes []byte) []byte {
	switch type_ {
		case "Echo":
			// Echo the message.
			// Example: "Hello world!"
			// Returns: the same message
			return bytes
		case "Email":
			// Send an email.
			// Example: "email_to@gmail.com|[a compressed EML file]"
			// Returns: nothing
			_ = Utils.QueueEmailEMAIL(Utils.EmailInfo{
				Mail_to: strings.Split(string(bytes), "|")[0],
				Eml:     Utils.DecompressString(bytes[strings.Index(string(bytes), "|") + 1:]),
			})
		case "File":
			// Get a file from the website.
			// Example: "[true to get CRC16 checksum, false to get file contents]|[partial_path]"
			// Returns: a CRC16 checksum or a compressed file
			var bytes_split []string = strings.Split(string(bytes), "|")
			var get_crc16 bool = bytes_split[0] == "true"
			var partial_path string = bytes_split[1]

			var p_file_contents *string = Utils.GetWebsiteFilesDirFILESDIRS().Add2(false, partial_path).ReadTextFile()
			if p_file_contents == nil {
				log.Println("Error file not found:", partial_path)

				break
			}

			if get_crc16 {
				return getCRC16([]byte(*p_file_contents))
			} else {
				return Utils.CompressString(*p_file_contents)
			}
		case "GPT":
			// Send a text to be processed by the GPT model.
			// Example: a compressed string or nil to just get the return value
			// Returns: "true" if the text will be processed immediately, "false" if the GPT is busy for now and the
			// text will wait
			var ret []byte = []byte(strconv.Itoa(int(Utils.GetGenSettings().MOD_7.State)))

			if len(bytes) > 0 {
				// Don't use channels for this. What if various messages are sent while one is still be processed? The
				// module will lock - as it did now.
				_ = Utils.GetUserDataDirMODULES(Utils.NUM_MOD_GPTCommunicator).Add2(false, "to_process",
					Utils.RandStringGENERAL(10) + ".txt").WriteTextFile(Utils.DecompressString(bytes), false)
			}

			return ret
		case "G_S":
			// Get settings.
			// Example: "[true to get file contents, false to get CRC16 checksum]|[one of the strings below]"
			// Allowed strings: one of the ones on the switch statement
			// Returns: the user settings in JSON format compressed
			var bytes_split []string = strings.Split(string(bytes), "|")
			var get_json bool = bytes_split[0] == "true"
			var json_origin string = bytes_split[1]
			var settings string = ""
			switch json_origin {
				case "US":
					settings = *Utils.ToJsonGENERAL(*Utils.GetUserSettings())
				case "Weather":
					settings = *Utils.ToJsonGENERAL(Utils.GetGenSettings().MOD_6.Weather)
				case "News":
					settings = *Utils.ToJsonGENERAL(Utils.GetGenSettings().MOD_6.News)
				case "GPTMem":
					settings = *Utils.ToJsonGENERAL(Utils.GetGenSettings().MOD_7.Memories)
				case "GPTSessions":
					settings = *Utils.ToJsonGENERAL(Utils.GetGenSettings().MOD_7.Sessions)
				case "GManTok":
					settings = *Utils.ToJsonGENERAL(Utils.GetGenSettings().MOD_14.Token)
				case "GManEvents":
					settings = *Utils.ToJsonGENERAL(Utils.GetGenSettings().MOD_14.Events)
				case "GManTasks":
					settings = *Utils.ToJsonGENERAL(Utils.GetGenSettings().MOD_14.Tasks)
				case "GManTokVal":
					settings = strconv.FormatBool(!Utils.GetGenSettings().MOD_14.Token_invalid)
				default:
					log.Println("Invalid JSON origin:", json_origin)
			}
			if get_json {
				return Utils.CompressString(settings)
			} else {
				return getCRC16([]byte(settings))
			}
		case "S_S":
			// Set settings.
			// Example: "[one of the strings below]|[a compressed JSON string]"
			// Allowed strings: one of the ones on the switch statement
			// Returns: nothing
			var bytes_split []string = strings.Split(string(bytes), "|")
			var origin string = bytes_split[0]
			var settings string = Utils.DecompressString(bytes[strings.Index(string(bytes), "|") + 1:])
			switch origin {
				case "US":
					var user_settings Utils.UserSettings
					_ = Utils.FromJsonGENERAL([]byte(settings), &user_settings)
					if user_settings.General.Website_domain != "" &&
							user_settings.General.Website_pw != "" &&
							user_settings.General.User_email_addr != "" {
						// Only accept the new settings if as a start the website information exists (or else the
						// client will be locked out of the server surely), but also if the user email is set (that will
						// mean the settings are not empty).
						*Utils.GetUserSettings() = user_settings
					}
				case "GPTMem":
					settings = strings.Replace(settings, "\\r", "", -1)
					_ = Utils.FromJsonGENERAL([]byte(settings), &Utils.GetGenSettings().MOD_7.Memories)
				case "GManTok":
					Utils.GetGenSettings().MOD_14.Token = settings
				case "GPTSession":
					var instructions []string = strings.Split(settings, "\000")
					var session_id string = instructions[0]
					var action string = instructions[1]
					if action == "delete" {
						for i, session := range Utils.GetGenSettings().MOD_7.Sessions {
							if session.Id == session_id {
								Utils.DelElemSLICES(&Utils.GetGenSettings().MOD_7.Sessions, i)

								break
							}
						}
					} else if action == "rename" {
						for i, session := range Utils.GetGenSettings().MOD_7.Sessions {
							if session.Id == session_id {
								Utils.GetGenSettings().MOD_7.Sessions[i].Name = instructions[2]

								break
							}
						}
					}
				default:
					log.Println("Invalid JSON destination:", origin)
			}
	}

	// Just to return something
	return nil
}

func registerChannel() int {
	for i := 0; i < MAX_CLIENTS; i++ {
		if !used_channels_GL[i] {
			channels_GL[i] = make(chan []byte)
			used_channels_GL[i] = true

			return i
		}
	}

	return -1
}

func unregisterChannel(channel_num int) {
	if channel_num >= 0 && channel_num < MAX_CLIENTS {
		close(channels_GL[channel_num])
		channels_GL[channel_num] = nil
		used_channels_GL[channel_num] = false
	}
}

func getCRC16(bytes []byte) []byte {
	var crc16 uint16 = CRC16.Result(bytes, "CCIT_ZERO")
	var crc16_bytes []byte = make([]byte, 2)
	crc16_bytes[0] = byte(crc16 >> 8)
	crc16_bytes[1] = byte(crc16)

	return crc16_bytes
}

func getModUserInfo() *ModsFileInfo.Mod8UserInfo {
	return &Utils.GetUserSettings().WebsiteBackend
}
