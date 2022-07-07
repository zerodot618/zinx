package main

import (
	"fmt"
	"zinx/ziface"
	"zinx/znet"
)

// ping test 自定义路由
type PingRouter struct {
	znet.BaseRouter
}

// Test Handle
func (pr *PingRouter) Handle(request ziface.IRequest) {
	fmt.Println("Call PingRouer Handle")
	// 先读取客户端的数据，再回写ping...ping...ping
	fmt.Println("recv from client: msgId=", request.GetMsgId(), " ,data=", string(request.GetData()))
	// 回写数据
	err := request.GetConnection().SendMsg(1, []byte("ping...ping...ping"))
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	// 创建一个 server 句柄
	s := znet.NewServer()
	// 配置路由
	s.AddRouter(&PingRouter{})
	// 开启服务
	s.Serve()
}
