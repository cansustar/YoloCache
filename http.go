package YoloCache

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

/*
***********************HTTP服务端*********************************
分布式缓存需要实现节点间通信，建立基于 HTTP 的通信机制是比较常见和简单的做法。
如果一个节点启动了 HTTP 服务，那么这个节点就可以被其他节点访问。
今天我们就为单机节点搭建 HTTP Server。
*/

// 创建一个结构体HTTPPool, 作为承载节点间HTTP通信的核心数据结构， 包括服务端和客户端， 今天只实现服务端

// 服务端默认前缀
const defaultBasePath = "/_yolocache/"

type HTTPPool struct {
	self     string // self用来记录自己的地址，包括主机名/IP 和端口
	basePath string //  basePath是作为节点间通讯地址的前缀，默认是 /_yolocache/
	// 所以 http://example.com/_yolocache/ 开头的请求，就用于节点间的访问。因为一个主机上还可能承载其他的服务，加一段 Path 是一个好习惯。
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

/*
***********************实现最为核心的ServeHTTP方法*********************************
 */

func (p *HTTPPool) Log(format string, v ...interface{}) {
	// v...interface{} 代表可变参数
	log.Printf("[Server %s, %s", p.self, fmt.Sprintf(format, v...))
}

// 为HTTPPool实现ServeHTTP方法，任何实现了 ServeHTTP 方法的对象都可以作为 HTTP 的 Handler。
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 判断请求路径是否正确
	// 请求的url没有以basePath开头
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// 请求url的格式： /<basepath>/<groupname>/<key>
	// 分割字符串 第二个参数表示最多分割的次数
	// 对Path前缀后的部分按照 / 进行分割，分成2 部分
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	// 判断请求的url是否符合格式，即是否能被分成两部分
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// 获取groupname和key
	groupName := parts[0]
	key := parts[1]
	// 根据groupname获取group
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}
	// 根据key获取缓存值
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 将缓存值写入到ResponseWriter
	w.Header().Set("Content-Type", "application/octet-stream")
	// 使用 w.Write() 将缓存值作为 httpResponse 的 body 返回。
	w.Write(view.ByteSlice())
}
