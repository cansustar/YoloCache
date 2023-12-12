package main

import (
	"YoloCache/yolocache"
	"flag"
	"fmt"
	"log"
	"net/http"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *yolocache.Group {
	return yolocache.NewGroup("scores", 2<<10, yolocache.GetterFunc(
		// 匿名函数 缓存未命中时的回调
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

// 用来启动缓存服务器：创建 HTTPPool，添加节点信息，注册到 gee 中，启动 HTTP 服务（共3个端口，8001/8002/8003），用户不感知。
func startCacheServer(addr string, addrs []string, yolo *yolocache.Group) {
	peers := yolocache.NewHTTPPool(addr)
	peers.Set(addrs...)
	yolo.RegisterPeers(peers)
	log.Println("yolocache is running at", addr)
	// peers实现ServeHTTP方法，任何实现了 ServeHTTP 方法的对象都可以作为 HTTP 的 Handler。
	log.Fatal(http.ListenAndServe(addr[7:], peers))

}

func startAPIServer(apiaddr string, yolo *yolocache.Group) {
	// 对外暴露一个api接口
	http.Handle("/api", http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			key := request.URL.Query().Get("key")
			view, err := yolo.Get(key)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
				return
			}
			writer.Header().Set("Content-Type", "application/octet-stream")
			writer.Write(view.ByteSlice())
		}))
	log.Println("fontend server is running at", apiaddr)
	// 使用 http.ListenAndServe 启动一个 HTTP 服务器，监听指定的地址，并使用默认的 nil 处理器（handler）。
	//如果启动过程中发生任何错误，它会调用 log.Fatal 输出错误信息，并退出程序。
	// apiaddr[7:]: 可能是去除了地址的前缀(去掉了http://)
	log.Fatal(http.ListenAndServe(apiaddr[7:], nil))
}

func main() {
	var port int // 在命令行参数中赋值
	var api bool
	/*
		使用 flag 包来定义一个整数变量 port，并将该变量与命令行参数关联起来。具体来说：

		flag.IntVar 用于定义一个整数变量，它接受四个参数：
		第一个参数是整数变量的地址，这里是 &port，表示将命令行参数的值赋给 port 变量。
		第二个参数是命令行标志的名称，这里是 "port"。
		第三个参数是默认值，如果命令行中没有提供该标志，则使用默认值。在这里，默认值是 8001。
		第四个参数是该标志的描述，这里是 "Yolocache server port"。
		通过这行代码，程序就能够接受命令行参数 -port 或者 --port 来指定服务器的端口号，如果没有提供，则使用默认值 8001。例如，可以在命令行中这样使用：

		bash
		Copy code
		./your_program -port=8080
	*/
	flag.IntVar(&port, "port", 8001, "Yolocache server port")
	// 命令行参数，bool
	flag.BoolVar(&api, "api", false, "Start a api server?")
	/*
		flag.Parse() 是用于解析命令行参数的函数。在使用 flag 包定义命令行标志之后，需要调用 flag.Parse() 来解析命令行参数，并将它们赋值给相应的变量。
		具体而言，flag.Parse() 将扫描命令行参数列表，并设置已定义标志的值。
		在之前的例子中，如果你的程序中包含了类似 flag.IntVar(&port, "port", 8001, "Yolocache server port") 这样的代码，
		那么在程序的入口处（通常是 main 函数的开始部分），你需要调用 flag.Parse()。
	*/
	flag.Parse()
	// 你运行程序时，可以通过命令行参数来设置 port 的值，而这些参数会在 flag.Parse() 被调用时生效。

	// 启动一个 API 服务（端口 9999），与用户进行交互，用户感知。
	apiAddr := "http://localhost:9999"
	/*
		启动 HTTP 服务（共3个端口，8001/8002/8003），用户不感知。
	*/
	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	yolo := createGroup()
	if api {
		go startAPIServer(apiAddr, yolo)
	}
	// 冗余类型转换 addrs已经是一个[]string
	startCacheServer(addrMap[port], addrs, yolo)
}
