package YoloCache

import (
	"fmt"
	"log"
	"sync"
)

/*
************************实现主体结构Group*************************
Group是YoloCache最核心的数据结构，负责与用户交互，控制缓存存储和获取的主流程

                            是
接收 key --> 检查是否被缓存 -----> 返回缓存值 ⑴
                |  否                         是
                |-----> 是否应当从远程节点获取 -----> 与远程节点交互 --> 返回缓存值 ⑵
                            |  否
                            |-----> 调用`回调函数`，获取值并添加到缓存 --> 返回缓存值 ⑶
*/

type Group struct {
	// 一个Group可以认为是一个缓存的命名空间，每个Group拥有一个唯一地名称name
	//比如可以创建三个 Group，缓存学生的成绩命名为 scores，缓存学生信息的命名为 info，缓存学生课程的命名为 courses。
	name      string
	getter    Getter // 第二个属性是 getter Getter，即缓存未命中时获取源数据的回调(callback)。
	mainCache cache  // 第三个属性是 mainCache cache，即一开始实现的并发缓存。
}

var (
	mu     sync.RWMutex              // 全局的锁
	groups = make(map[string]*Group) // 全局的一个groups
)

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
		缓存不存在，则调用 load 方法，
		load 调用 getLocally（分布式场景下会调用 getFromPeer 从其他节点获取），
		getLocally 调用用户回调函数 g.getter.Get() 获取源数据，
		并且将源数据添加到缓存 mainCache 中（通过 populateCache 方法）
	*/
	return g.load(key)
}

func (g *Group) load(key string) (value ByteView, err error) {
	return g.getLocally(key)
}

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
