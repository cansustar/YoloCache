package yolocache

import (
	"YoloCache/yolocache/singleflight"
	"fmt"
	"log"
	"sync"
)

/*
*
************************实现主体结构Group*************************
Group是YoloCache最核心的数据结构，负责与用户交互，控制缓存存储和获取的主流程

                            是
接收 key --> 检查是否被缓存 -----> 返回缓存值 ⑴
                |  否                         是
                |-----> 是否应当从远程节点获取 -----> 与远程节点交互 --> 返回缓存值 ⑵
                            |  否
                            |-----> 调用`回调函数`，获取值并添加到缓存 --> 返回缓存值 ⑶
*/

// Group /*
// !!!day6  TODO!!! 注意理解Group的这几个成员变量，为什么有的类型是对应的结构体类型，而loader是引用类型！！！
// day6 因为对于loader来说，这个管理请求的数据结构，应该是需要在多个Group实例中共享的，所以要是指针类型
// 而对于name,mainCache，每个节点的实例都是独立的，所以选择传递值
// TODO: 但这就产生了另外的一个问题，peers和getter,应该也是每个节点都一致的吧？ 按理来说也可以使用指针类型
type Group struct {
	// 一个Group可以认为是一个缓存的命名空间，每个Group拥有一个唯一的名称name
	//比如可以创建三个 Group，缓存学生的成绩命名为 scores，缓存学生信息的命名为 info，缓存学生课程的命名为 courses。
	name      string              // 每个Group拥有唯一的名称name
	getter    Getter              // 第二个属性是 getter Getter，即缓存未命中时获取源数据的回调(callback)。
	mainCache cache               // 第三个属性是 mainCache cache，即一开始实现的并发缓存。
	peers     PeerPicker          // 将用于获取远程节点
	loader    *singleflight.Group // 管理请求的数据结构，这里为什么要想到把singleflight里的group加到Group中？ 可以想到， 他们应该在一起初始化。所以下一步就是更新初始化函数
}

var (
	mu     sync.RWMutex              // 全局的锁
	groups = make(map[string]*Group) // 全局的一个groups
)

// RegisterPeers RegisterPeers方法，将 实现了 PeerPicker 接口的 HTTPPool 注入到 Group 中。
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// NewGroup 构建NewGroup， 实例化Group
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock() // 为什么这里的锁是全局的？
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

func GetGroup(name string) *Group {
	// 这里用的是只读锁,因为不涉及任何冲突变量的写操作。
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

/*
*****************Group最核心的方法：Get******************************
为什么Group要实现Get方法？
因为Group负责与用户的交互，控制缓存值存储和获取的流程，

*/

func (g *Group) Get(key string) (ByteView, error) {
	// 如果key为空，返回以零值初始化的ByteView实例，和错误
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	// 从 mainCache 中查找缓存，如果存在则返回缓存值。
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[YoloCache] hit")
		return v, nil
	}
	/*
		缓存不存在，尝试去其他节点寻找缓存。 调用 load 方法，
		load 调用 getLocally（分布式场景下会调用 getFromPeer 从其他节点获取），
		getLocally 调用用户回调函数 g.getter.Get() 获取源数据，
		并且将源数据添加到缓存 mainCache 中（通过 populateCache 方法）
	*/
	return g.load(key)
}

// 当在本节点没有找到时，调用load尝试从其他节点获取
// 设计时预留：分布式场景下，load 会先从远程节点获取 getFromPeer，失败了再回退到 getLocally
func (g *Group) load(key string) (value ByteView, err error) {
	// 使用g.loader.Do包裹原来的代码，这样确保了在并发场景下针对相同的key,load过程只会调用一次 day6
	view, err := g.loader.Do(key, func() (interface{}, error) {
		// 如果有其他节点存在
		if g.peers != nil {
			// 如果是分布式节点，从其他节点获取， 这里p返回的peer是目标节点的
			if peer, ok := g.peers.PickPeer(key); ok {
				// 再用这个baseurl传入getFromPeer函数中，去获取这个key的value
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil // 从其他节点获取成功，返回
				}
				log.Println("[YoloCache] Failed to get from peer", err)
			}
		}
		return g.getLocally(key)
	})
	// day6
	if err == nil {
		return view.(ByteView), nil
	}
	return
}

func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	// 调用peer的Get方法，向其他节点发起请求，查询value
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
}

// 从本地没找到，先尝试去从其他节点找，如果其他节点也没找到的话，那就再返回本地来，去调用的回调函数，获取数据源中的数据，再添加到缓存中并返回
func (g *Group) getLocally(key string) (ByteView, error) {
	// 调用用户回调函数g.getter.Get() 获取源数据
	bytes, err := g.getter.Get(key) // Get方法返回f(key)， 这里也就是把key传到用户提供的匿名函数中，调用获取返回值
	// 获取失败
	if err != nil {
		return ByteView{}, err
	}
	// 获取成功，添加到缓存mainCache中
	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	// 添加到mainCache中
	g.mainCache.add(key, value)
}

// 回调Getter
/*
如果缓存不存在，应从数据源（文件，数据库等）获取数据并添加到缓存中。
GeeCache 是否应该支持多种数据源的配置呢？
不应该，一是数据源的种类太多，没办法一一实现；二是扩展性不好。
如何从源头获取数据，应该是用户决定的事情，我们就把这件事交给用户好了。
因此，我们设计了一个回调函数(callback)，在缓存不存在时，调用这个函数，得到源数据。
*/

// Getter 为了兼容用户自定义的获取数据方式，定义了一个接口Getter，并且只要实现这个接口的类型都可以作为Getter。
type Getter interface {
	// Get 定义回调函数Get，用于从数据源（如MySQL、文件等）获取数据
	Get(key string) ([]byte, error)
}

// GetterFunc 是一个函数类型, 且实现了Getter接口的Get方法
// GetterFunc 函数实现了某个接口，这称为接口型函数
type GetterFunc func(key string) ([]byte, error)

// Get 实现了Getter接口的Get方法
/*
定义一个函数类型 F，并且实现接口 A 的方法，然后在这个方法中调用自己。这是 Go 语言中将其他函数（参数返回值定义与 F 一致）转换为接口 A 的常用技巧。
*/
func (f GetterFunc) Get(key string) ([]byte, error) {
	// 为什么是调用自己呢？ 因为GetterFunc本身是一个函数类型，在使用时，会赋值给一个Getter接口类型的变量，这个变量就是f，值为这个func类型实例化时传入的匿名函数
	// 所以这里的f，就是匿名函数， 传入key，即以key为参数，调用匿名函数
	return f(key)
}
