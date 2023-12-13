#!/bin/bash
# trap 命令设置一个退出时执行的命令。 当脚本退出时，会删除名为 server 的可执行文件，并向当前进程组中的所有子进程发送 kill 信号。
trap "rm server;kill 0" EXIT
# 使用 go build 命令编译一个名为 server 的可执行文件。
go build -o server
# 启动三个 server 实例，分别监听不同的端口（8001、8002、8003），并通过 & 将它们放入后台运行。
./server -port=8001 &
./server -port=8002 &
./server -port=8003 -api=1 &
#  命令等待 2 秒，以确保服务器实例已经成功启动。
sleep 2
# 输出 ">>> start test" 作为提示信息。
echo ">>> start test"
# 使用三个 curl 命令向 http://localhost:9999/api 发送请求，模拟对 API 的并发访问。这里有三个并发请求，每个请求都带有相同的参数 key=Tom。
curl "http://localhost:9999/api?key=Tom" &
curl "http://localhost:9999/api?key=Tom" &
curl "http://localhost:9999/api?key=Tom" &
curl "http://localhost:9999/api?key=Tom" &
curl "http://localhost:9999/api?key=Tom" &


# 使用 wait 命令等待所有后台任务完成。这包括三个 server 实例的运行以及三个并发的 curl 请求。
wait

# 主要用于编译并并发地启动多个 server 实例，并模拟对 API 的并发请求以进行测试。在测试结束后，会清理生成的可执行文件。