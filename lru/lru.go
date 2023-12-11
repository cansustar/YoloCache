package lru

import "container/list"

/*
***********************LRU核心数据结构***************************
 */

// Value 为了通用性，允许值是实现了Value接口的任意类型，该接口只包含了一个方法Len() int， 用于返回值所占用的内存大小。
type Value interface {
	Len() int
}

// Cache LRU缓存
type Cache struct {
	maxBytes int64                    // 允许使用的最大内存
	nbytes   int64                    // 当前已使用的内存
	ll       *list.List               // 双向链表, Go语言标准库实现
	cache    map[string]*list.Element // 键是字符串, 值是双向链表中对应节点的指针
	// value的类型是interface{}，可以接收任意类型的值
	OnEvicted func(key string, value Value) // 某条记录被移除时的回调函数, 可为 nil
}

// entry 是双向链表的节点类型, 在链表中仍保存每个值对应的key的好处在于，淘汰队首节点时，需要用key从字典中删除对应的映射。
type entry struct {
	key   string
	value Value
}

// Len 返回值所占用的内存大小
func (c *Cache) Len() int {
	// Cache类型的值c的Len()方法，返回当前所用内存c.nbytes
	return c.ll.Len()
}

// New 为了方便实例化Cache, 实现New()函数
func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes: maxBytes,
		// nbytes:    0, 这里无需显示初始化nbytes, 因为结构体初始化时，所有的字段都会被初始化为对应类型的零值（结构体类型的为nil）
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

/*
*************************查找功能*************************
 */

// 查找功能主要有两步
//第一步是从字典中找到对应的双向链表的节点
// 第二步 将该节点移动到队尾

// Get 根据传入的key,从字典中找到双向链表节点
func (c *Cache) Get(key string) (value Value, ok bool) {
	// 如果对应的链表节点存在，则将对应节点移动到队尾， 并返回查找到的值
	if ele, ok := c.cache[key]; ok {
		// MovewToFront方法是list包中的方法，将对应的节点移动到队尾(双向链表作为队列，队首队尾是相对的，约定front为队尾)
		c.ll.MoveToFront(ele)
		// 因为这个Value是any类型，实际上就是interface{}类型，所以需要断言成*entry类型
		// 断言的作用：通常用于处理容器中包含多种类型的情况，而程序员需要根据实际类型来执行不同的操作。
		// 在这里，ele.Value 被假设为一个 *entry 类型的指针。如果断言成功，那么变量 v 将是一个 *entry 类型的指针，可以通过 v 访问其成员变量；否则，断言失败，将会触发一个 panic。
		// entry是结构体类型，所以这里要传的是指针，否则传的是值的话，只是传了一个副本，对副本的修改不会影响原来的值
		// 这里类型断言的目的是 目的是将缓存中的值从接口类型还原为实际存储的 *entry 类型。
		/*
			为什么要进行这次类型断言呢？因为在 Cache 中，
			Value 是一个接口类型，可以存储不同类型的值，
			而在实际使用时，我们可能需要访问这个值的具体字段或方法。
			通过类型断言，可以将接口值还原为原始的 *entry 类型，以便进一步操作。
		*/
		// 另一个更直观的原因就是 Add时，Push进去的是一个*entry类型的指针，所以这里取出来的时候也要取出来一个*entry类型的指针
		kv := ele.Value.(*entry)
		return kv.value, true

	}
	return
}

/*
**************************删除功能***************************
 */

// 这里的删除实际上是缓存淘汰，即移除最近最少访问的节点（队首）

func (c *Cache) RemoveOldest() {
	ele := c.ll.Back() // 双向链表的Back()方法返回队首节点
	if ele != nil {
		c.ll.Remove(ele) // 如果队首节点存在，则将其从双向链表中删除
		// entry是结构体类型，所以这里要传的是指针，否则传的是值的话，只是传了一个副本，对副本的修改不会影响原来的值
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)                                // 从字典中删除对应的映射关系
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len()) // 更新当前所用内存
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value) // 如果回调函数OnEvicted不为nil，则调用回调函数
		}
	}
}

/*
**************************新增/修改功能***************************
 */

// Add 新增/修改功能
func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele) // 如果键存在，则将对应节点移动到队尾
		// 为什么这里传递的是指针？
		// entry是结构体类型，所以这里要传的是指针，否则传的是值的话，只是传了一个副本，对副本的修改不会影响原来的值
		kv := ele.Value.(*entry)
		//更新nbytes, 因为新值可能和旧值大小不同
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		// 更新节点值
		kv.value = value
	} else {
		// 如果不存在，则在队尾添加新节点，并在字典中添加key和节点的映射关系
		ele := c.ll.PushFront(&entry{key, value})
		// 在字典中添加key和节点的映射关系
		c.cache[key] = ele
		// 更新当前所用内存
		// 为什么这里value可以调用Len方法
		// 因为value是Value类型，而Value类型是一个接口，接口是一种特殊的类型，它规定了变量有哪些方法，但是没有具体实现，具体实现由其他类型提供
		// 也就是说，这里调用的Len实际上是底层的数据结构，实现了Len方法，所以这里可以调用
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	// 如果超过了设定的最大值c.maxBytes,则移除最少访问的节点。
	// 如果c.maxBytes为0，则不限制最大内存，即不会移除任何节点
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}
