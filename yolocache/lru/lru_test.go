package lru

import (
	"reflect"
	"testing"
)

// 单元测试

// 为什么定义一个string类型的String?
/*
在这个特定的示例中，定义了一个类型 String，实际上是为了满足 Value 接口的实现需求。
接口 Value 中定义了一个 Len() 方法，用于返回值的长度。而在这个测试中，想要测试的值是字符串，但 string 类型是 Go 语言中的原生类型，不能直接为其定义新的方法。
通过定义一个别名类型 String，我们可以为这个新类型添加方法，这样就能满足 Value 接口的需求。
这种做法在实际的代码中可能会更加常见，特别是当你需要为一些原生类型添加额外的方法时。
*/
type String string

func (d String) Len() int {
	return len(d)
}

// 尝试添加几条数据，测试 Get 方法
func TestGet(t *testing.T) {
	// 初始化一个Cache, 最大内存限制为8字节 为什么0是八字节？
	// 传入0表示不限制内存大小
	lru := New(int64(0), nil)
	lru.Add("key1", String("1234"))
	if v, ok := lru.Get("key"); !ok || string(v.(String)) != "1234" {
		// Fatalf 格式化输出错误信息并退出程序
		t.Fatalf("cache hit key1=1234 failed")
	}
	if _, ok := lru.Get("key2"); ok {
		t.Fatalf("cache miss key2 failed")
	}
}

// 测试当使用内存超过了设定值时，是否会出发”无用“节点的移除
func TestRemoveoldest(t *testing.T) {
	k1, k2, k3 := "key1", "key2", "k3"
	v1, v2, v3 := "value1", "value2", "v3"
	// 将maxBytes设置为刚好能容纳两个键值对
	cap := len(k1 + k2 + v1 + v2)
	// 初始化一个Cache, 最大内存限制为cap
	lru := New(int64(cap), nil)
	// 往Cache中添加3个键值对， 这时最早加入的那个（key1）会被移除
	lru.Add(k1, String(v1))
	lru.Add(k2, String(v2))
	lru.Add(k3, String(v3))
	// 现在key1应该已经被移除了， 如果再次取到，或者lru的元素个数不为2， 则测试失败
	if _, ok := lru.Get("key1"); ok || lru.Len() != 2 {
		t.Fatalf("Removeoldest key1 failed")
	}
}

// 测试回调函数能否被调用
func TestOnevicted(t *testing.T) {
	// 定义一个变量来记录回调函数被调用的次数
	keys := make([]string, 0)
	// 定义回调函数onEvicted
	// 回调函数的作用是，当某条记录被移除时，将该条记录的key放入到keys切片中
	callback := func(key string, value Value) {
		keys = append(keys, key)
	}
	//  创建了一个具有最大内存空间为 10 字节的 LRU 缓存。
	lru := New(int64(1), callback)
	// 向缓存中添加四个键值对，对应的值都是字符串
	lru.Add("key1", String("1234"))
	lru.Add("k2", String("k2"))
	lru.Add("k3", String("k3"))
	lru.Add("k4", String("k4"))
	// 定义了一个切片，有元素"key1", "k2"
	expect := []string{"key1", "k2"}
	// 用于检查两个切片 expect 和 keys 是否相等。
	if !reflect.DeepEqual(expect, keys) {
		t.Fatalf("Call OnEvicted failed, expect keys equals to %s", expect)
	}

}
