package znet

import (
	"fmt"
	"net"
	"time"
	"zinx/ziface"
)

// iServer 接口实现，定义一个 Server 服务结构体
type Server struct {
	// 服务器名称
	Name string
	// tcp4 or other
	IPVersion string
	// 服务绑定的 IP 地址
	IP string
	// 服务绑定的端口
	Port int
}

// NewServer 创建一个服务器句柄
func NewServer(name string) ziface.IServer {
	s := &Server{
		Name:      name,
		IPVersion: "tcp4",
		IP:        "0.0.0.0",
		Port:      7777,
	}
	return s
}

// Start 开启网络服务
func (s *Server) Start() {
	fmt.Printf("[START] Server listener at IP: %s, Port %d, is starting\n", s.IP, s.Port)

	// 开启一个 go 去做服务端 Listenner 业务
	go func() {
		// 1. 获取一个 TCP 的 Addr
		addr, err := net.ResolveTCPAddr(s.IPVersion, fmt.Sprintf("%s:%d", s.IP, s.Port))
		if err != nil {
			fmt.Println("resolve tcp addr err: ", err)
			return
		}
		// 2. 监听服务器地址
		listenner, err := net.ListenTCP(s.IPVersion, addr)
		if err != nil {
			fmt.Println("listen ", s.IPVersion, " err", err)
			return
		}
		// 已经监听成功
		fmt.Println("start Zinx server ", s.Name, " success, now listenning...")
		// 3. 启动 server 网络链接业务
		for {
			// 3.1 阻塞等待客户端建立连接请求
			conn, err := listenner.AcceptTCP()
			if err != nil {
				fmt.Println("Accept err ", err)
				continue
			}
			// 3.2 TODO Server.Start() 设置服务器最大连接控制，超过最大连接，那么则关闭此新的连接
			// 3.3 TODO Server.Start() 处理该新连接请求的 业务 方法，此时应该有 handler 和 conn 是绑定的
			// 这里暂时做一个最大 512 字节的回显服务
			go func() {
				// 不断的循环从客户端获取数据
				for {
					buf := make([]byte, 512)
					cnt, err := conn.Read(buf)
					if err != nil {
						fmt.Println("recv buf err ", err)
						continue
					}
					// 回显
					if _, err := conn.Write(buf[:cnt]); err != nil {
						fmt.Println("write back buf err ", err)
						continue
					}
				}
			}()
		}
	}()
}

// Stop 停止服务
func (s *Server) Stop() {
	fmt.Println("[STOP] zinx server, name ", s.Name)
	// TODO Server.Stop() 将其他需要清理的连接信息或者其他信息，也要一并停止或者清理
}

func (s *Server) Serve() {
	s.Start()
	// TODO Server.Serve() 是否在启动服务的时候，还要处理其他的事情呢，可以在这里添加
	// 阻塞，否则主 go 退出，listenner 的 go 将会退出
	for {
		time.Sleep(10 * time.Second)
	}
}