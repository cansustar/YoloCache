package yolocache

import pb "YoloCache/yolocache/yolocachepb"

/*
*******************************注册节点， 借助一致性哈希算法选择节点*********************************

*****************************实现HTTP 客户端，与远程节点的服务端通信*********************************
 */

/*
 流程回顾：
						是
接收 key --> 检查是否被缓存 -----> 返回缓存值 ⑴
                |  否                         是
                |-----> 是否应当从远程节点获取 -----> 与远程节点交互 --> 返回缓存值 ⑵
                            |  否
                            |-----> 调用`回调函数`，获取值并添加到缓存 --> 返回缓存值 ⑶
*/

/*
细化流程2：
使用一致性哈希选择节点        是                                    是
    |-----> 是否是远程节点 -----> HTTP 客户端访问远程节点 --> 成功？-----> 服务端返回返回值
                    |  否                                    ↓  否
                    |----------------------------> 回退到本地节点处理。
*/

// PeerPicker 抽象一个PeerPicker
type PeerPicker interface {
	// PickPeer 用于根据传入的key, 选择相应节点的PeerGetter
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 对应于上述流程中的HTTP客户端，使用Get方法从对应group查找缓存值
// 该接口对应着，http客户端请求去找值
//type PeerGetter interface {
//	// Get 用于从对应group查找缓存值
//	Get(group string, key string) ([]byte, error)
//}

type PeerGetter interface {
	// Get 用于从对应group查找缓存值
	//Get(in *pb.Request, out *pb.Response) ([]byte, error)
	Get(in *pb.Request, out *pb.Response) error
}
