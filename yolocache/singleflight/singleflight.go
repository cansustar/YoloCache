package singleflight

import "sync"

// 为了避免缓存击穿，我们需要在高并发场景下，对相同的key，只让一个请求去查询数据，其他请求等待这个请求的结果即可

// 为了实现这个功能，我们需要一个数据结构，来记录正在进行或者已经结束的请求，这样其他请求就可以等待这个请求的结果了

// call 代表正在进行，或者已经结束的请求，使用sync.WaitGroup锁避免重入

// 关于如何抽象出来call？
// TODO  表示这个请求的状态？ 主要通过wg的状态来表示 错 key就已经可以表示当前请求的状态了，
// TODO 现在的想法是：  call主要是为了实现重入锁wg，以及fn的执行状态和结果，因为防止重入，本质上是要防止fn的重复执行
type call struct {
	wg  sync.WaitGroup // 用于实现重入锁  // 为什么要用sync.WaitGroup呢？ 并发协程之间不需要消息传递，非常适合 sync.WaitGroup。
	val interface{}    // 函数执行的结果
	err error          // 函数执行的错误
}

// Group 是 singleflight 的主数据结构，管理不同 key 的请求(call)。
// Group的作用是，防止重复的请求，每次都要去对目标节点发起请求
// 所以可以想到，这个Group应该是全局唯一的，所以应该是单例模式（这里我其实是看到Do里的锁的时候，才考虑到这个问题的）
// 因为如果是每个节点都有一个Group的话，那么同样的请求发往不同的节点，会导致每个节点都去请求数据源，这样就没意义了
type Group struct {
	mu sync.Mutex       // 保护 Group 的成员变量 m 不被并发读写而加上的锁。
	m  map[string]*call // 用于记录每个key的请求状态
}

// Do 我们已经写好了请求从节点获取数据的逻辑，这时候又要控制从请求到达节点的过程， 那么需要在这个过程中加一个中间函数，该函数用来在请求到达节点前，判断是否有相同请求正在运行
// 所以这个函数Do,需要接受一个函数作为参数，而这个参数就是允许请求进入后，请求去节点获取数据的函数
// Do   需要一个函数，作用是针对相同的key，无论Do被调用多少次，函数fn都只会被调用一次，等待fn调用结束后，返回返回值或者错误
// 这里为什么要传入一个fn呢？ TODO
// 对于参数的类型，因为不确定，所以使用了interface{}，对于返回值，因为不确定，所以使用了interface{}和error
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	/* 对于每一个请求，都有两种情况:
	1. 这个key的请求从来没有被发起过
	2. 已经有相同key的请求正在进行中

	针对这两种不同的情况，我们需要做不同的处理
	*/
	g.mu.Lock() // 这个锁要保护的是map
	// TODO 这里是否可以直接用Once来优化？
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	// 肯定首先是要尝试从map中获取这个key的请求状态
	// 读map，需要加锁
	if c, ok := g.m[key]; ok {
		// 读取到后解锁  TODO 这把锁的作用范围
		g.mu.Unlock()
		// 如果这个key的请求正在进行中，则等待这个请求结束，返回结果或者错误
		c.wg.Wait()
		// 如果这个key的请求已经结束了，则删除这个key的请求状态，返回结果或者错误
		return c.val, c.err // Wait结束后，这里的c已经被前面进行的那次请求修改成结果了，所以这里不需要再调用fn
	}
	// 如果map中没有这个key的请求，则发起这个请求，返回结果或者错误
	// 实例化一个c, 将用来承载fn的返回值
	c := new(call)
	c.wg.Add(1) // 发起请求前，加锁,表示有一个请求正在进行中
	// 因为进行到这里，说明没走上面判断key在进行中，所以上面读map的锁还没有解锁
	g.m[key] = c // 将这个key的请求状态添加到map中，表明key已经有对应的请求在处理
	g.mu.Unlock()
	c.val, c.err = fn() // 发起请求, 执行fn函数
	// TODO v1 先写的是不考虑并发的版本，所以v1里没有使用group的mu, 思考在并发情况下，应该在哪里加锁？
	c.wg.Done() // 请求结束，解锁， 这时在上面的Wait被释放
	// 要进行删除操作，所以要加锁
	g.mu.Lock()
	delete(g.m, key) // 跟新g.m,删除key
	g.mu.Unlock()
	return c.val, c.err // 返回结果
}

// Do方法接收一个key和一个函数fn，如果这个key的请求正在进行中，则等待这个请求结束，返回结果或者错误
// 如果这个key的请求已经结束了，则删除这个key的请求状态，返回结果或者错误
// 如果这个key的请求从来没有被发起过，则发起这个请求，返回结果或者错误
