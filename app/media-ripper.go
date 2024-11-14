package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/lrstanley/go-ytdlp"
	"github.com/spf13/viper"
)

const PORT = 9778

type URL struct {
	Url     string `json:"url"`
	Cookies string `json:"cookies"`
	Format  string `json:"format"`
}

type POSSIBLE_PATH struct {
	Id       string   `json:"id"`
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Keywords []string `json:"keywords"`
}

var possiblePaths = []POSSIBLE_PATH{}
var urls = []URL{}

var logger = slog.New(NewPrettyHandler(os.Stdout, PrettyHandlerOptions{}))

func init() {
	setupViperConfig()

	parsePossiblePaths(&possiblePaths)
	parseUrls(&urls)
	logger.Info("possiblePaths", "possiblePaths", possiblePaths)
	logger.Info("urls", "urls", urls)

	ytdlp.MustInstall(context.TODO(), nil)
}

func main() {
	setupHttpServer()
}

func setupViperConfig() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		logger.Error("UserConfigDir", "error", err.Error())
		os.Exit(1)
	}
	configDir = path.Join(configDir, "quick-dl")

	// Create the config directory if it doesn't exist
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		logger.Error("MkdirAll", "error", err.Error())
		os.Exit(1)
	}

	viper.SetDefault("path", "")
	viper.SetDefault("quiet", false)

	homedir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("UserHomeDir", "error", err.Error())
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

	viper.SetDefault("urls", []interface{}{
		map[string]interface{}{
			"url":     "",
			"cookies": "",
			"format":  "",
		},
	})

	viper.AddConfigPath(configDir)
	viper.SetConfigName("config")
	viper.SetConfigType("json")

	_ = viper.SafeWriteConfig()

	err = viper.ReadInConfig()
	if err != nil {
		logger.Error("ReadInConfig", "error", err.Error())
		os.Exit(1)
	}
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
			fmt.Println("Upgrade", "error", err)
			return
		}

		wsConnections = append(wsConnections, conn)
		logger.Info("New connection", "addr", conn.RemoteAddr())

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("ReadMessage", "error", err)

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

			go handleWsMessage(string(message))
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
			fmt.Println("WriteMessage", "error", err)
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
	var msg WS_MESSAGE
	err := json.Unmarshal([]byte(message), &msg)
	if err != nil {
		fmt.Println("Unmarshal", "error", err)
		return
	}

	logger.Info("Message", "msg", msg)

	switch msg.Type {
	case "download":
		downloadFile(REQUEST_BODY{Url: msg.Data.Url}, "")

	case "picked_path":
		downloadFile(REQUEST_BODY{Url: msg.Data.Url}, msg.Data.Id)

	default:
		logger.Info("Unknown message type", "type", msg.Type)
	}
}

// log error and send error message to ws with handleWsMessage
func handleErrorToWS(key string, err error) {
	fullString := `{"type":"error", "error":"` + key + `", "message":"` + escapeError(err) + `"}`

	sendWsMessage(fullString)
}

func escapeError(err error) string {
	res := strings.ReplaceAll(err.Error(), `"`, `\"`)
	res = strings.ReplaceAll(res, "\n", "\\n")

	return res
}

func downloadFile(body REQUEST_BODY, pathId string) {
	yt := getYt()
	yt.PrintJSON()

	if pathId == "" {
		setCookies(yt, checkCookies(body.Url))

		yt.SkipDownload()

		r, err := yt.Run(context.TODO(), body.Url)
		if err != nil {
			logger.Error("Run", "error", err.Error())
			handleErrorToWS("run", err)
			return
		}

		var result map[string]any
		if err := json.Unmarshal([]byte(r.Stdout), &result); err != nil {
			logger.Error("Unmarshal", "error", err.Error())
			handleErrorToWS("", err)
			return
		}

		var title string
		var tags []string

		if result["title"] != nil {
			title = result["title"].(string)
		}

		if result["tags"] != nil {
			for _, tag := range result["tags"].([]interface{}) {
				tags = append(tags, tag.(string))
			}
		}

		var formats []string
		if result["formats"] != nil {
			// formats is array of objects which contain a data field which contains the format key and value
			for _, format := range result["formats"].([]interface{}) {
				formatMap := format.(map[string]interface{})
				if formatMap["format"] != nil {
					formats = append(formats, formatMap["format"].(string))
				}
			}
		}

		logger.Info("Title", "title", title)
		logger.Info("Tags", "tags", tags)
		logger.Info("Formats", "formats", formats)

		if title != "" {
			paths := getPaths(title, tags)

			if len(paths) == 0 {
				sendWsMessage("No paths found")
				return
			}

			json, _ := json.Marshal(&paths)

			fullString := `{"type":"choose_path","url":"` + body.Url + `","paths":` + string(json) + `}`

			sendWsMessage(fullString)
		}
	} else {
		yt.UnsetSkipDownload()

		path := getPathById(pathId)
		logger.Info("Downloading to path", "path", path)

		if path == nil {
			logger.Error("DownloadFile", "error", "Path is nil")
			return
		}

		if path.Path == "" {
			logger.Error("DownloadFile", "error", "Path is empty")
			return
		}

		yt.Paths(path.Path)

		// check if we have some custom url settings for the given url
		customUrl := checkCookies(body.Url)

		setCookies(yt, customUrl)

		yt.UnsetFormat()
		if customUrl != nil {
			if customUrl.Format != "" {
				yt.Format(customUrl.Format)
			}

			logger.Info("Custom url settings", "customUrl", customUrl)
		}

		result, err := yt.Run(context.TODO(), body.Url)
		if err != nil {
			logger.Error("Run", "error", err.Error())
			handleErrorToWS("run", err)
			return
		}

		logger.Info("Result", "result", result)

	}
}

func getYt() *ytdlp.Command {
	var yt = ytdlp.New()

	if viper.GetBool("quiet") {
		yt.Quiet()
	}

	return yt
}

func checkCookies(url string) *URL {
	for _, u := range urls {
		if strings.Contains(url, u.Url) {
			return &u
		}
	}

	return nil
}

func setCookies(yt *ytdlp.Command, url *URL) {
	if url == nil {
		return
	}

	yt.UnsetCookies()
	yt.Cookies(url.Cookies)
}
