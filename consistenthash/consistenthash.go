package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

/*
*****************************一致性哈希算法的实现*********************************
 */

// Hash 定义一个函数类型Hash,采取依赖注入的方式，允许用于替换成自定义的Hash函数
type Hash func(data []byte) uint32

// 一致性哈希算法的主要数据结构
type Map struct {
	hash     Hash           // hash函数
	replicas int            // 虚拟节点倍数
	keys     []int          // 哈希环，使用一个有序的int数组来存储哈希环上的所有节点的哈希值
	hashMap  map[int]string // 虚拟节点与真实节点的映射表，键是虚拟节点的哈希值，值是真实节点的名称

}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		hash:     fn,
		replicas: replicas,
		keys:     nil,
		hashMap:  make(map[int]string),
	}

	if m.hash == nil {
		// 默认的Hash函数是crc32.ChecksumIEEE
		m.hash = crc32.ChecksumIEEE
	}

	return m
}

// Add  添加真实节点/机器的Add方法，允许传入0或多个真实节点的名称
func (m *Map) Add(keys ...string) {
	// 每个key 代表着真实节点
	for _, key := range keys {
		// 为每个真实节点key，创建m.replicas个虚拟节点，
		for i := 0; i < m.replicas; i++ {
			// 虚拟节点的名称是：strconv.Itoa(i) + key，即通过添加编号的方式区分不同虚拟节点
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			// 将虚拟节点的哈希值添加到环上
			m.keys = append(m.keys, hash)
			// 在Hashmap中添加虚拟节点和真实节点的映射关系
			// 将虚拟节点的hash值作为key, 真实节点名（1，2，3.。） 作为value
			m.hashMap[hash] = key
		}
	}
	// 在环上，将所有的虚拟节点的哈希值进行排序，方便之后进行二分查找
	sort.Ints(m.keys)
}

// Get 实现选择节点的Get方法
func (m *Map) Get(key string) string {
	// 如果哈希环为空，就直接返回空
	if len(m.keys) == 0 {
		return ""
	}
	// 计算传入的key的哈希值
	hash := int(m.hash([]byte(key)))
	// 寻找第一个匹配的虚拟节点的下标
	// Search方法是二分查找
	idx := sort.Search(len(m.keys), func(i int) bool {
		// 注意这里， return这一行还在上面的匿名函数里， 接受一个参数i，
		// i 实际上是二分法 二分查找，如果没有找到满足条件的元素，则返回要插入该元素以保持切片有序的索引。
		//return m.keys[i] >= hash 如果keys中i处的元素大于传入的key的哈希值， 返回true
		// 将该函数传入二分查找中，实际上获得的idx， 要么就是元素值等于当前hash值的元素的索引，要么就是当前hash值，应该插入的位置的索引
		// 也就是说，如果当前的这个hash值大于keys中最大的hash值，那么idx就是最大的hash值的索引+1
		return m.keys[i] >= hash
	})
	// 找到虚拟节点的在hash环中的下标后，还需要确定其hash值，再通过hashmap找真实节点
	// keys切片中存储的是哈希环中的哈希值，因为这里的idx,可能会大于keys的长度，所以需要取余，得到真实的下标
	return m.hashMap[m.keys[idx%len(m.keys)]]
}
