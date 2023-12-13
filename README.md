# YoloCache
go实现分布式缓存
##根据proto生成go文件：

 protoc --gofast_out=plugins=grpc:. --proto_path= yolocache/yolocachepb/*.proto  ./yolocache/yolocachepb/*.proto
