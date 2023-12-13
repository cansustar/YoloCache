package yolocache

/*
**********************为lru.Cache添加并发特性*********************************
 */
import (
	"YoloCache/yolocache/lru"
	"sync"
)

// 并发缓存结构体
type cache struct {
	mu  sync.Mutex
	lru *lru.Cache
	// 缓存最大值, 与lru中的maxBytes相同
	cacheBytes int64
	once       sync.Once
}

// 封装get和add方法，并添加互斥锁mu
// func (c *cache) add(key string, value ByteView) {
// TODO:这里有较大的优化空间，add操作不管是否存在lru,都会加一个锁
//
//		c.mu.Lock()
//		defer c.mu.Unlock()
//		// 这里的lru有点类似一个单例,对吗
//		// a: 是的，这里的lru是一个单例
//		if c.lru == nil {
//			c.lru = lru.New(c.cacheBytes, nil)
//		}
//		c.lru.Add(key, value)
//	}

// TODO  是我错了，Add方法涉及到了对map的写操作，所以Add也要在锁内
//func (c *cache) add(key string, value ByteView) {
//	c.once.Do(func() {
//		c.lru = lru.New(c.cacheBytes, nil)
//	})
//	// 不需要加锁，因为 sync.Once.Do 保证其中的函数只执行一次
//	c.lru.Add(key, value)
//}
//

// 最终的修改解决方案
func (c *cache) add(key string, value ByteView) {
	c.once.Do(func() {
		c.lru = lru.New(c.cacheBytes, nil)
	})
	c.mu.Lock()
	defer c.mu.Unlock()
	// 确保在初始化完成后再执行 Add 操作
	c.lru.Add(key, value)
}

// TODO 同样存在锁粒度的问题
//
//	func (c *cache) get(key string) (value ByteView, ok bool) {
//		c.mu.Lock()
//		defer c.mu.Unlock()
//		if c.lru == nil {
//			return
//		}
//		if v, ok := c.lru.Get(key); ok {
//			/*
//				在这段代码中，c.lru.Get(key) 返回的值的类型是 lru.Value，它是一个接口类型。
//				由于 ByteView 类型实现了 Value 接口，所以可以进行类型断言，将 v 转换为 ByteView 类型。
//				具体而言，lru.Value 接口定义如下：
//				type Value interface {
//				    Len() int
//				}
//				而 ByteView 类型实现了 Len() int 方法：
//				func (v ByteView) Len() int {
//				    return len(v.b)
//				}
//				因此，ByteView 类型满足 lru.Value 接口的要求。
//				在使用 v.(ByteView) 进行类型断言时，表示我们期望 v 是 ByteView 类型，而编译器会检查 v 是否真的实现了 Value 接口，以及 v 的底层类型是否是 ByteView。
//			*/
//			return v.(ByteView), ok
//		}
//		return
//	}
// # TODO 最开始的修改版本， 感觉做了无意义的双重检查，以及，get操作需要加锁吗？get方法里涉及到了对map的读，和对链表节点的移动，这似乎是并发安全的（GPT说的）
// TODO!!!! 兔兔老师在评论区里说，cache 的 get 和 add 都涉及到写操作(LRU 将最近访问元素移动到链表头)，所以不能直接改为读写锁。 所以get应该还得加锁
//func (c *cache) get(key string) (value ByteView, ok bool) {
//	// 这里的思想是 双重检查锁定  当然可以用once来优化
//	// 1. 先检查 lru 是否为 nil
//	if c.lru == nil {
//		return
//	}
//	// 2. 加锁
//	c.mu.Lock()
//	defer c.mu.Unlock()
//	// 3. 检查 lru 是否为 nil，因为在加锁期间可能被其他 goroutine 初始化
//	if c.lru == nil {
//		return
//	}
//	// 4. 获取值
//	if v, ok := c.lru.Get(key); ok {
//		// 5. 类型断言
//		return v.(ByteView), ok
//	}
//	return
//}

// TODO 尝试去掉锁
func (c *cache) get(key string) (value ByteView, ok bool) {
	// 这里的思想是 双重检查锁定  当然可以用once来优化
	// 1. 先检查 lru 是否为 nil
	if c.lru == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// 2. 不加锁了
	// 4. 获取值
	if v, ok := c.lru.Get(key); ok {
		// 5. 类型断言
		return v.(ByteView), ok
	}
	return
}
