package yolocache

import (
	"YoloCache/yolocache/consistenthash"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

/*
***********************HTTP服务端*********************************
分布式缓存需要实现节点间通信，建立基于 HTTP 的通信机制是比较常见和简单的做法。
如果一个节点启动了 HTTP 服务，那么这个节点就可以被其他节点访问。
今天我们就为单机节点搭建 HTTP Server。
*/

// 创建一个结构体HTTPPool, 作为承载节点间HTTP通信的核心数据结构， 包括服务端和客户端， 今天只实现服务端

// 服务端默认前缀
const (
	defaultBasePath = "/_yolocache/"
	// 默认节点数
	defaultReplicas = 50
)

type HTTPPool struct {
	self     string // self用来记录自己的地址，包括主机名/IP 和端口
	basePath string //  basePath是作为节点间通讯地址的前缀，默认是 /_yolocache/
	// 所以 http://example.com/_yolocache/ 开头的请求，就用于节点间的访问。因为一个主机上还可能承载其他的服务，加一段 Path 是一个好习惯。
	mu sync.Mutex //  这个锁用来保护 peers 变量 和 httpGetters 变量
	// 多个 goroutine 同时添加/删除节点： 如果有一个 goroutine 正在添加或删除节点，而另一个 goroutine 同时也在修改节点信息，没有锁的话可能导致不一致的状态。
	//
	//并发的 HTTP 请求： 当有多个请求同时发生，它们可能会涉及到节点的增加、删除等操作，需要保证这些操作的原子性，避免竞态条件。
	peers       *consistenthash.Map    // 一致性哈希算法的Map，用来根据具体的key选择节点
	httpGetters map[string]*httpGetter // 映射远程节点与对应的 httpGetter。每一个远程节点对应一个 httpGetter，因为 httpGetter 与远程节点的地址 baseURL 有关
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

/*
***********************实现HTTP客户端*********************************
 */

type httpGetter struct { // httpGetter实现了peerGetter的Get函数
	// 表示将要访问的远程节点的地址
	baseURL string
}

func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL, // baseURL这里的最后一个字符是 /，所以不用再加了
		url.QueryEscape(group),
		url.QueryEscape(key),
	) //
	// TODO 与远程节点通信 可以考虑使用rpc
	res, err := http.Get(u) // 向远程节点发送HTTP请求
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	// 如果返回的状态码不是OK，就返回错误
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned: %v", res.Status)
	}
	// 读取body
	bytes, err := io.ReadAll(res.Body)

	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}

	return bytes, nil
}

// 表示创建了一个 *httpGetter 类型的 nil 值，并将其转换为 PeerGetter 接口类型。   类型断言：v.(ByteView)
/*
使用 var _ InterfaceType = (*ConcreteType)(nil)
这种语法的目的是确保在编译时进行检查，以保证 ConcreteType 类型实现了 InterfaceType 接口。
这种写法不仅仅是声明一个变量，更是在声明的同时对其进行初始化，并将其赋值为 nil，以确保编译时能够检查到 ConcreteType 类型是否满足 InterfaceType 接口。

当我们使用类型断言时，
例如 if v, ok := someValue.(InterfaceType); ok，
这是在运行时进行的检查。如果我们只使用类型断言而不使用 var _ 这种方式，那么在编译时是无法确保类型实现的正确性的。这种方式更容易出现在运行时才发现的错误。
因此，var _ InterfaceType = (*ConcreteType)(nil) 这种写法是为了在编译时就确保接口的正确实现，是一种更严格的检查方式。
*/
// 接口健全性检查  由于接口的实现是隐式的，有时候我们希望在编译时就能够确保某个类型实现了某个接口，这样可以避免在运行时发生错误。
// 将空值 nil 转换为 *httpGetter，再转换为 PeerGetter 接口，如果转换失败，说明 httpGetter 并没有实现 PeerGetter 接口的所有方法。
var _ PeerGetter = (*httpGetter)(nil)

// 为HTTPPool添加节点选择的功能

// Set 方法实例化了一致性哈希算法，并且添加了传入的节点。
// peers ...string 表示这里接受的peers是一个可变参数，可以传入0个或多个参数
// s如果使用s...符号解压缩切片，则可以将切片直接传递给可变参数函数。在这种情况下，不会创建新的切片。
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 实例化一个一致性哈希算法， defaultReplicas是虚拟节点的倍数, nil表示使用默认的hash函数
	p.peers = consistenthash.New(defaultReplicas, nil)
	// 添加节点, 这里的peers...表示将切片peers打散传入
	p.peers.Add(peers...)
	// 初始化httpGetter， 每一个远程节点对应一个httpGetter，映射远程节点与对应的 httpGetter
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		// 为每一个远程节点创建一个httpGetter
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// PickPeer PickerPeer() 包装一致性哈希算法的Get() 方法，并根据具体的key， 选择节点， 返回节点对应的http客户端
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 根据传入的key， 选择节点
	// 如果选择的节点不是当前节点，那么就返回这个节点对应的http客户端
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("pick peer %s", peer)
		// httpGetter实现了PeerGetter的Get方法，所以可以认为返回的httpGetter类型，就是PeerGetter类型
		return p.httpGetters[peer], true // 返回节点对应的http客户端
	}
	return nil, false
}

// 编译时检查 HTTPPool 是否实现了 PeerPicker 接口
var _ PeerPicker = (*HTTPPool)(nil)
