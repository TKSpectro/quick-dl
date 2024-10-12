package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/lrstanley/go-ytdlp"
	"github.com/spf13/viper"
)

const PORT = 9778

var yt = ytdlp.New()

func init() {
	setupViperConfig()

	parsePossiblePaths()

	ytdlp.MustInstall(context.TODO(), nil)

	if viper.GetBool("quiet") {
		yt.Quiet()
	}
}

func main() {
	setupHttpServer()
}

func printStruct(s interface{}) {
	// print key-value pairs of a struct

	v := reflect.ValueOf(s)
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fmt.Printf("%s: %v\n", t.Field(i).Name, v.Field(i).Interface())
	}

	fmt.Println()
}

func setupViperConfig() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	configDir = path.Join(configDir, "quick-dl")

	// Create the config directory if it doesn't exist
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		fmt.Println("MkdirAll | " + err.Error())
		os.Exit(1)
	}

	viper.SetDefault("path", "")
	viper.SetDefault("quiet", false)

	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("UserHomeDir | " + err.Error())
		os.Exit(1)
	}

	viper.SetDefault("paths", []interface{}{
		map[string]interface{}{
			"name":     "Downloads",
			"path":     path.Join(homedir, "Downloads"),
			"keywords": []interface{}{},
		},
		map[string]interface{}{
			"name":     "Music",
			"path":     path.Join(homedir, "Music"),
			"keywords": []interface{}{"music", "song", "album"},
		},
		map[string]interface{}{
			"name":     "Videos",
			"path":     path.Join(homedir, "Videos"),
			"keywords": []interface{}{"video", "movie", "film"},
		},
	})

	viper.AddConfigPath(configDir)
	viper.SetConfigName("config")
	viper.SetConfigType("json")

	_ = viper.SafeWriteConfig()

	err = viper.ReadInConfig()
	if err != nil {
		fmt.Println("ReadInConfig | " + err.Error())
		os.Exit(1)
	}
}

var possiblePaths = []POSSIBLE_PATH{}

func parsePossiblePaths() {
	var id = 0

	// read possible paths from config file
	for _, url := range viper.Get("paths").([]interface{}) {
		possiblePaths = append(possiblePaths, POSSIBLE_PATH{
			Id:       fmt.Sprintf("%d", id),
			Name:     getString(url, "name"),
			Path:     getString(url, "path"),
			Keywords: lowerCaseStringArray(getStringArray(url, "keywords")),
		})

		id++
	}

	fmt.Println("Possible paths:")
	for _, path := range possiblePaths {
		printStruct(path)
	}
}

func getString(urlMap interface{}, key string) string {
	if value, ok := urlMap.(map[string]interface{})[key]; ok {
		return value.(string)
	}
	return ""
}

func getStringArray(urlMap interface{}, key string) []string {
	if value, ok := urlMap.(map[string]interface{})[key]; ok {
		var arr []string
		for _, v := range value.([]interface{}) {
			arr = append(arr, v.(string))
		}
		return arr
	}
	return []string{}
}

func lowerCaseStringArray(arr []string) []string {
	var newArr []string
	for _, s := range arr {
		newArr = append(newArr, strings.ToLower(s))
	}
	return newArr
}

type REQUEST_BODY struct {
	Url string `json:"url"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var wsConnections []*websocket.Conn

func setupHttpServer() {
	router := http.NewServeMux()

	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println("Upgrade | " + err.Error())
			return
		}

		wsConnections = append(wsConnections, conn)

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("ReadMessage | " + err.Error())

				conn.Close()
				// remove connection from wsConnections
				for i, c := range wsConnections {
					if c == conn {
						wsConnections = append(wsConnections[:i], wsConnections[i+1:]...)
						break
					}
				}

				return
			}

			handleWsMessage(string(message))
		}
	})

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		sendError(w, http.StatusNotFound, "Invalid route")
	})

	http.ListenAndServe(":"+fmt.Sprintf("%d", PORT), router)
}

func sendWsMessage(message string) {
	for _, conn := range wsConnections {
		err := conn.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			fmt.Println("WriteMessage | " + err.Error())
		}
	}
}

type WS_MESSAGE_DATA struct {
	Url string `json:"url"`
	Id  string `json:"id"`
}

type WS_MESSAGE struct {
	Type string          `json:"type"`
	Data WS_MESSAGE_DATA `json:"data"`
}

func handleWsMessage(message string) {
	fmt.Println("Message: " + message)

	var msg WS_MESSAGE
	err := json.Unmarshal([]byte(message), &msg)
	if err != nil {
		fmt.Println("Unmarshal | " + err.Error())
		return
	}

	fmt.Println("Type: " + msg.Type)

	switch msg.Type {
	case "download":
		downloadFile(REQUEST_BODY{Url: msg.Data.Url}, "")
		break

	case "picked_path":
		fmt.Println("Picked path: " + msg.Data.Url)
		downloadFile(REQUEST_BODY{Url: msg.Data.Url}, msg.Data.Id)
		break
	default:
		fmt.Println("Invalid message type")
	}

}

func sendError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	w.Write([]byte(message))
}

func sendSuccess(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}

type POSSIBLE_PATH struct {
	Id       string   `json:"id"`
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Keywords []string `json:"keywords"`
}

func downloadFile(body REQUEST_BODY, pathId string) {
	yt.PrintJSON()

	if pathId == "" {
		yt.SkipDownload()

		r, err := yt.Run(context.TODO(), body.Url)
		if err != nil {
			fmt.Println("Run | " + err.Error())
		}

		info, err := r.GetExtractedInfo()
		if err != nil {
			fmt.Println("GetExtractedInfo | " + err.Error())
			return
		}

		// write info to test.json
		// f, err := os.Create("test.json")
		// if err != nil {
		// 	fmt.Println("Create | " + err.Error())
		// 	return
		// }

		// enc := json.NewEncoder(f)
		// enc.SetIndent("", "  ")
		// enc.Encode(info)

		if len(info) > 0 {
			if info[0].Title == nil {
				fmt.Println("Title is nil")
				return
			}

			title := *info[0].Title
			fmt.Println("Title: " + title)

			paths := getPaths(title)

			if len(paths) == 0 {
				sendWsMessage("No paths found")
				return
			}

			for _, path := range paths {
				printStruct(*path)
			}

			// send ws message with json of paths

			json, _ := json.Marshal(&paths)

			fullString := `{"type":"choose_path","url":"` + body.Url + `","paths":` + string(json) + `}`

			sendWsMessage(fullString)
		}
	} else {
		yt.UnsetSkipDownload()

		fmt.Println("Downloading to path with id: ", pathId)

		path := getPathById(pathId)

		if path == nil {
			fmt.Println("Path not found")
			return
		}

		fmt.Println("Path: " + path.Path)

		if path.Path == "" {
			fmt.Println("Path is empty")
			return
		}

		yt.Paths(path.Path)

		_, err := yt.Run(context.TODO(), body.Url)
		if err != nil {
			fmt.Println("Run | " + err.Error())
		}

	}
}

// check title for any of the keywords. Copy all paths into a new array, sort it by the number of keywords found in the title and return the array
func getPaths(title string) []*POSSIBLE_PATH {
	var cleanTitle = strings.ToLower(title)

	var paths []*POSSIBLE_PATH

	for _, path := range possiblePaths {
		paths = append(paths, &path)
	}

	sort.Slice(paths, func(i, j int) bool {
		var iCount int = 0
		var jCount int = 0

		for _, keyword := range paths[i].Keywords {
			if containsKeyword(cleanTitle, keyword) {
				iCount++
			}
		}

		for _, keyword := range paths[j].Keywords {
			if containsKeyword(cleanTitle, keyword) {
				jCount++
			}
		}

		return iCount > jCount
	})

	return paths
}

func getPathById(id string) *POSSIBLE_PATH {
	for _, path := range possiblePaths {
		if path.Id == id {
			return &path
		}
	}

	return nil
}

func containsKeyword(title string, keyword string) bool {
	return strings.Contains(title, keyword)
}
