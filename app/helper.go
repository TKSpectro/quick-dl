package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// func printStruct(s interface{}) {
// 	// print key-value pairs of a struct

// 	v := reflect.ValueOf(s)
// 	t := v.Type()

// 	for i := 0; i < v.NumField(); i++ {
// 		fmt.Printf("%s: %v\n", t.Field(i).Name, v.Field(i).Interface())
// 	}

// 	fmt.Println()
// }

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

func sendError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	w.Write([]byte(message))
}

// func sendSuccess(w http.ResponseWriter, message string) {
// 	w.WriteHeader(http.StatusOK)
// 	w.Write([]byte(message))
// }

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

func parsePossiblePaths(possiblePaths *[]POSSIBLE_PATH) {
	var id = 0

	// read possible paths from config file
	for _, url := range viper.Get("paths").([]interface{}) {
		*possiblePaths = append(*possiblePaths, POSSIBLE_PATH{
			Id:       fmt.Sprintf("%d", id),
			Name:     getString(url, "name"),
			Path:     getString(url, "path"),
			Keywords: lowerCaseStringArray(getStringArray(url, "keywords")),
		})

		id++
	}
}

func parseUrls(urls *[]URL) {
	for _, url := range viper.Get("urls").([]interface{}) {
		*urls = append(*urls, URL{
			Url:     getString(url, "url"),
			Cookies: getString(url, "cookies"),
			Format:  getString(url, "format"),
		})
	}
}
