package main

import (
	"YoloCache"
	"fmt"
	"log"
	"net/http"
)

// 实例化group, 并启动HTTP服务

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func main() {
	YoloCache.NewGroup("scores", 2<<10, YoloCache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))

	addr := "localhost:9999"
	peers := YoloCache.NewHTTPPool(addr)
	log.Println("yolocache is running at", addr)
	log.Fatal(http.ListenAndServe(addr, peers))
}
