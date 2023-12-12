package YoloCache

// 使用sync.Mutex封装LRU的几个方法，使之支持并发的读写

/*
***********************抽象一个只读数据结构ByteView用来表示缓存值，这是YoloCache的主要数据结构之一*************************
 */

type ByteView struct {
	// ByteView 只有一个数据成员，b []byte，b 将会存储真实的缓存值。选择 byte 类型是为了能够支持任意的数据类型的存储，例如字符串、图片等。
	b []byte
}

// Len 在lru.Cache的实现中，要求被缓存对象必须实现Value接口，即Len() int方法，返回其所占的内存大小
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice b是只读的（从struct对其的定义中也可以看出来,b不想被外界访问）
// 返回一个b的拷贝，防止缓存值被外部程序修改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// 实现了 String 方法的类型可以被用于格式化输出
func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
